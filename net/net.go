package net

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/ipfs/go-cid"
	bs "github.com/ipfs/go-ipfs-blockstore"
	format "github.com/ipfs/go-ipld-format"
	logging "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	pstore "github.com/libp2p/go-libp2p-core/peerstore"
	gostream "github.com/libp2p/go-libp2p-gostream"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/textileio/go-threads/broadcast"
	"github.com/textileio/go-threads/cbor"
	lstore "github.com/textileio/go-threads/core/logstore"
	core "github.com/textileio/go-threads/core/net"
	"github.com/textileio/go-threads/core/thread"
	sym "github.com/textileio/go-threads/crypto/symmetric"
	pb "github.com/textileio/go-threads/net/pb"
	"github.com/textileio/go-threads/util"
	"google.golang.org/grpc"
)

var (
	log = logging.Logger("threadnet")

	// MaxPullLimit is the maximum page size for pulling records.
	MaxPullLimit = 10000

	// InitialPullInterval is the interval between automatic log pulls.
	InitialPullInterval = time.Second

	// PullInterval is the interval between automatic log pulls.
	PullInterval = time.Second * 10

	// notifyTimeout is the duration to wait for a subscriber to read a new record.
	notifyTimeout = time.Second * 5
)

// net is an implementation of core.Net.
type net struct {
	format.DAGService
	host   host.Host
	bstore bs.Blockstore

	store lstore.Logstore

	rpc    *grpc.Server
	server *server
	bus    *broadcast.Broadcaster

	ctx    context.Context
	cancel context.CancelFunc

	pullLock  sync.Mutex
	pullLocks map[thread.ID]chan struct{}
}

// Config is used to specify thread instance options.
type Config struct {
	Debug bool
}

// NewNetwork creates an instance of net from the given host and thread store.
func NewNetwork(ctx context.Context, h host.Host, bstore bs.Blockstore, ds format.DAGService, ls lstore.Logstore, conf Config, opts ...grpc.ServerOption) (core.Net, error) {
	var err error
	if conf.Debug {
		if err = util.SetLogLevels(map[string]logging.LogLevel{
			"threadnet": logging.LevelDebug,
			"logstore":  logging.LevelDebug,
		}); err != nil {
			return nil, err
		}
	}

	ctx, cancel := context.WithCancel(ctx)
	t := &net{
		DAGService: ds,
		host:       h,
		bstore:     bstore,
		store:      ls,
		rpc:        grpc.NewServer(opts...),
		bus:        broadcast.NewBroadcaster(0),
		ctx:        ctx,
		cancel:     cancel,
		pullLocks:  make(map[thread.ID]chan struct{}),
	}
	t.server, err = newServer(t)
	if err != nil {
		return nil, err
	}

	listener, err := gostream.Listen(h, thread.Protocol)
	if err != nil {
		return nil, err
	}
	go func() {
		pb.RegisterServiceServer(t.rpc, t.server)
		if err := t.rpc.Serve(listener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			log.Fatalf("serve error: %v", err)
		}
	}()

	go t.startPulling()

	return t, nil
}

func (t *net) Close() (err error) {
	t.rpc.GracefulStop()

	var errs []error
	weakClose := func(name string, c interface{}) {
		if cl, ok := c.(io.Closer); ok {
			if err = cl.Close(); err != nil {
				errs = append(errs, fmt.Errorf("%s error: %s", name, err))
			}
		}
	}

	weakClose("DAGService", t.DAGService)
	weakClose("host", t.host)
	weakClose("threadstore", t.store)

	t.bus.Discard()
	t.cancel()

	if len(errs) > 0 {
		return fmt.Errorf("failed while closing net; err(s): %q", errs)
	}

	t.pullLock.Lock()
	defer t.pullLock.Unlock()
	// Wait for all thread pulls to finish
	for _, semaph := range t.pullLocks {
		semaph <- struct{}{}
	}

	return nil
}

func (t *net) Host() host.Host {
	return t.host
}

func (t *net) Store() lstore.Logstore {
	return t.store
}

func (t *net) GetHostID(context.Context) (peer.ID, error) {
	return t.host.ID(), nil
}

func (t *net) CreateThread(_ context.Context, id thread.ID, opts ...core.KeyOption) (info thread.Info, err error) {
	if err = t.ensureUnique(id); err != nil {
		return
	}

	args := &core.KeyOptions{}
	for _, opt := range opts {
		opt(args)
	}

	info = thread.Info{
		ID:  id,
		Key: args.ThreadKey,
	}
	if !info.Key.Defined() {
		info.Key = thread.NewRandomKey()
	}
	if err = t.store.AddThread(info); err != nil {
		return
	}
	linfo, err := createLog(t.host.ID(), args.LogKey)
	if err != nil {
		return
	}
	if err = t.store.AddLog(id, linfo); err != nil {
		return
	}

	if err = t.server.ps.Add(id); err != nil {
		return
	}

	return t.store.GetThread(id)
}

func (t *net) ensureUnique(id thread.ID) error {
	_, err := t.store.GetThread(id)
	if err == nil {
		return fmt.Errorf("thread %s already exists", id)
	}
	if !errors.Is(err, lstore.ErrThreadNotFound) {
		return err
	}
	return nil
}

func (t *net) AddThread(ctx context.Context, addr ma.Multiaddr, opts ...core.KeyOption) (info thread.Info, err error) {
	id, err := thread.FromAddr(addr)
	if err != nil {
		return
	}
	if err = t.ensureUnique(id); err != nil {
		return
	}

	args := &core.KeyOptions{}
	for _, opt := range opts {
		opt(args)
	}

	if err = t.store.AddThread(thread.Info{
		ID:  id,
		Key: args.ThreadKey,
	}); err != nil {
		return
	}
	if args.ThreadKey.CanRead() {
		var linfo thread.LogInfo
		linfo, err = createLog(t.host.ID(), args.LogKey)
		if err != nil {
			return
		}
		if err = t.store.AddLog(id, linfo); err != nil {
			return
		}
	}

	threadComp, err := ma.NewComponent(thread.Name, id.String())
	if err != nil {
		return
	}
	peerAddr := addr.Decapsulate(threadComp)
	addri, err := peer.AddrInfoFromP2pAddr(peerAddr)
	if err != nil {
		return
	}
	if err = t.Host().Connect(ctx, *addri); err != nil {
		return
	}
	lgs, err := t.server.getLogs(ctx, id, addri.ID)
	if err != nil {
		return
	}

	for _, l := range lgs {
		if err = t.createExternalLogIfNotExist(id, l.ID, l.PubKey, l.PrivKey, l.Addrs); err != nil {
			return
		}
	}

	if err = t.server.ps.Add(id); err != nil {
		return
	}

	return t.store.GetThread(id)
}

func (t *net) GetThread(_ context.Context, id thread.ID) (thread.Info, error) {
	return t.store.GetThread(id)
}

func (t *net) getThreadSemaphore(id thread.ID) chan struct{} {
	var ptl chan struct{}
	var ok bool
	t.pullLock.Lock()
	defer t.pullLock.Unlock()
	if ptl, ok = t.pullLocks[id]; !ok {
		ptl = make(chan struct{}, 1)
		t.pullLocks[id] = ptl
	}
	return ptl
}

func (t *net) PullThread(ctx context.Context, id thread.ID) error {
	log.Debugf("pulling thread %s...", id)
	ptl := t.getThreadSemaphore(id)
	select {
	case ptl <- struct{}{}:
		err := t.pullThread(ctx, id)
		if err != nil {
			<-ptl
			return err
		}
		<-ptl
	default:
		log.Warnf("pull thread %s ignored since already being pulled", id)
	}
	return nil
}

// pullThread for new records.
// This method is internal and *not* thread-safe. It assumes we currently own the thread-lock.
func (t *net) pullThread(ctx context.Context, id thread.ID) error {
	info, err := t.store.GetThread(id)
	if err != nil {
		return err
	}

	// Gather offsets for each log
	offsets := make(map[peer.ID]cid.Cid)
	for _, lg := range info.Logs {
		has, err := t.bstore.Has(lg.Head)
		if err != nil {
			return err
		}
		if has {
			offsets[lg.ID] = lg.Head
		} else {
			offsets[lg.ID] = cid.Undef
		}
	}
	var fetchedRcs []map[peer.ID][]core.Record
	wg := sync.WaitGroup{}
	for _, lg := range info.Logs {
		wg.Add(1)
		go func(lg thread.LogInfo) {
			defer wg.Done()
			// Pull from addresses
			recs, err := t.server.getRecords(
				ctx,
				id,
				lg.ID,
				offsets,
				MaxPullLimit)
			if err != nil {
				log.Error(err)
				return
			}
			fetchedRcs = append(fetchedRcs, recs)
		}(lg)
	}
	wg.Wait()
	for _, recs := range fetchedRcs {
		for lid, rs := range recs {
			for _, r := range rs {
				if err = t.putRecord(ctx, id, lid, r); err != nil {
					log.Error(err)
					return err
				}
			}
		}
	}

	return nil
}

func (t *net) DeleteThread(ctx context.Context, id thread.ID) error {
	log.Debugf("deleting thread %s...", id)
	ptl := t.getThreadSemaphore(id)
	select {
	case ptl <- struct{}{}: // Must block in case the thread is being pulled
		err := t.deleteThread(ctx, id)
		if err != nil {
			<-ptl
			return err
		}
		<-ptl
	}
	return nil
}

// deleteThread cleans up all the persistent and in-memory bits of a thread. This includes:
// - Removing all record and event nodes.
// - Deleting all logstore keys, addresses, and heads.
// - Cancelling the pubsub subscription and topic.
// Local subscriptions will not be cancelled and will simply stop reporting.
// This method is internal and *not* thread-safe. It assumes we currently own the thread-lock.
func (t *net) deleteThread(ctx context.Context, id thread.ID) error {
	if err := t.server.ps.Remove(id); err != nil {
		return err
	}

	info, err := t.store.GetThread(id)
	if err != nil {
		return err
	}
	for _, lg := range info.Logs { // Walk logs, removing record and event nodes
		head := lg.Head
		for head.Defined() {
			head, err = t.deleteRecord(ctx, head, info.Key.Service())
			if err != nil {
				return err
			}
		}
	}

	return t.store.DeleteThread(id) // Delete logstore keys, addresses, and heads
}

func (t *net) AddReplicator(ctx context.Context, id thread.ID, paddr ma.Multiaddr) (pid peer.ID, err error) {
	info, err := t.store.GetThread(id)
	if err != nil {
		return
	}

	// Extract peer portion
	p2p, err := paddr.ValueForProtocol(ma.P_P2P)
	if err != nil {
		return
	}
	pid, err = peer.Decode(p2p)
	if err != nil {
		return
	}

	// Update local addresses
	addr, err := ma.NewMultiaddr("/" + ma.ProtocolWithCode(ma.P_P2P).Name + "/" + p2p)
	if err != nil {
		return
	}
	ownlg, err := t.getOwnLog(id)
	if err != nil {
		return
	}
	if err = t.store.AddAddr(id, ownlg.ID, addr, pstore.PermanentAddrTTL); err != nil {
		return
	}
	info, err = t.store.GetThread(id) // Update info
	if err != nil {
		return
	}

	// Update peerstore address
	dialable, err := getDialable(paddr)
	if err == nil {
		t.host.Peerstore().AddAddr(pid, dialable, pstore.PermanentAddrTTL)
	} else {
		log.Warnf("peer %s address requires a DHT lookup", pid)
	}

	// Send all logs to the new replicator
	for _, l := range info.Logs {
		if err = t.server.pushLog(ctx, id, l, pid, info.Key.Service(), nil); err != nil {
			if err := t.store.SetAddrs(id, ownlg.ID, ownlg.Addrs, pstore.PermanentAddrTTL); err != nil {
				log.Errorf("error rolling back log address change: %s", err)
			}
			return
		}
	}

	// Send the updated log to peers
	var addrs []ma.Multiaddr
	for _, l := range info.Logs {
		addrs = append(addrs, l.Addrs...)
	}

	wg := sync.WaitGroup{}
	for _, addr := range addrs {
		wg.Add(1)
		go func(addr ma.Multiaddr) {
			defer wg.Done()
			p, err := addr.ValueForProtocol(ma.P_P2P)
			if err != nil {
				log.Error(err)
				return
			}
			pid, err := peer.Decode(p)
			if err != nil {
				log.Error(err)
				return
			}
			if pid.String() == t.host.ID().String() {
				return
			}

			if err = t.server.pushLog(ctx, id, ownlg, pid, nil, nil); err != nil {
				log.Errorf("error pushing log %s to %s", ownlg.ID, p)
			}
		}(addr)
	}

	wg.Wait()
	return pid, nil
}

func getDialable(addr ma.Multiaddr) (ma.Multiaddr, error) {
	parts := strings.Split(addr.String(), "/"+ma.ProtocolWithCode(ma.P_P2P).Name)
	return ma.NewMultiaddr(parts[0])
}

func (t *net) CreateRecord(ctx context.Context, id thread.ID, body format.Node) (r core.ThreadRecord, err error) {
	// Get or create a log for the new node
	lg, err := t.getOrCreateOwnLog(id)
	if err != nil {
		return
	}

	// Write a record locally
	rec, err := t.createRecord(ctx, id, lg, body)
	if err != nil {
		return
	}

	// Update head
	if err = t.store.SetHead(id, lg.ID, rec.Cid()); err != nil {
		return nil, err
	}

	log.Debugf("added record %s (thread=%s, log=%s)", rec.Cid(), id, lg.ID)

	// Notify local listeners
	r = NewRecord(rec, id, lg.ID)
	if err = t.bus.SendWithTimeout(r, notifyTimeout); err != nil {
		return
	}

	// Push out the new record
	if err = t.server.pushRecord(ctx, id, lg.ID, rec); err != nil {
		return
	}

	return r, nil
}

func (t *net) AddRecord(ctx context.Context, id thread.ID, lid peer.ID, rec core.Record) error {
	logpk, err := t.store.PubKey(id, lid)
	if err != nil {
		return err
	}
	if logpk == nil {
		return fmt.Errorf("log not found")
	}

	knownRecord, err := t.bstore.Has(rec.Cid())
	if err != nil {
		return err
	}
	if knownRecord {
		return nil
	}

	// Verify the record signature
	if err = rec.Verify(rec.Sig(), logpk); err != nil {
		return err
	}

	return t.PutRecord(ctx, id, lid, rec)
}

func (t *net) GetRecord(ctx context.Context, id thread.ID, rid cid.Cid) (core.Record, error) {
	sk, err := t.store.ServiceKey(id)
	if err != nil {
		return nil, err
	}
	if sk == nil {
		return nil, fmt.Errorf("a service-key is required to get records")
	}
	return cbor.GetRecord(ctx, t, rid, sk)
}

// Record wraps a core.Record within a thread and log context.
type Record struct {
	core.Record
	threadID thread.ID
	logID    peer.ID
}

// NewRecord returns a record with the given values.
func NewRecord(r core.Record, id thread.ID, lid peer.ID) core.ThreadRecord {
	return &Record{Record: r, threadID: id, logID: lid}
}

// Value returns the underlying record.
func (r *Record) Value() core.Record {
	return r
}

// ThreadID returns the record's thread ID.
func (r *Record) ThreadID() thread.ID {
	return r.threadID
}

// LogID returns the record's log ID.
func (r *Record) LogID() peer.ID {
	return r.logID
}

func (t *net) Subscribe(ctx context.Context, opts ...core.SubOption) (<-chan core.ThreadRecord, error) {
	args := &core.SubOptions{}
	for _, opt := range opts {
		opt(args)
	}
	filter := make(map[thread.ID]struct{})
	for _, id := range args.ThreadIDs {
		if id.Defined() {
			filter[id] = struct{}{}
		}
	}
	channel := make(chan core.ThreadRecord)
	go func() {
		defer close(channel)
		listener := t.bus.Listen()
		defer listener.Discard()
		for {
			select {
			case <-ctx.Done():
				return
			case i, ok := <-listener.Channel():
				if !ok {
					return
				}
				if rec, ok := i.(*Record); ok {
					if len(filter) > 0 {
						if _, ok := filter[rec.threadID]; ok {
							channel <- rec
						}
					} else {
						channel <- rec
					}
				} else {
					log.Warn("listener received a non-record value")
				}
			}
		}
	}()
	return channel, nil
}

// PutRecord adds an existing record. This method is thread-safe
func (t *net) PutRecord(ctx context.Context, id thread.ID, lid peer.ID, rec core.Record) error {
	tsph := t.getThreadSemaphore(id)
	tsph <- struct{}{}
	defer func() { <-tsph }()
	return t.putRecord(ctx, id, lid, rec)
}

// putRecord adds an existing record. See PutOption for more.This method
// *should be thread-guarded*
func (t *net) putRecord(ctx context.Context, id thread.ID, lid peer.ID, rec core.Record) error {
	var unknownRecords []core.Record
	c := rec.Cid()
	for c.Defined() {
		exist, err := t.bstore.Has(c)
		if err != nil {
			return err
		}
		if exist {
			break
		}
		var r core.Record
		if c.String() != rec.Cid().String() {
			r, err = t.GetRecord(ctx, id, c)
			if err != nil {
				return err
			}
		} else {
			r = rec
		}
		unknownRecords = append(unknownRecords, r)
		c = r.PrevID()
	}
	if len(unknownRecords) == 0 {
		return nil
	}
	// Get or create a log for the new rec
	lg, err := t.getLog(id, lid)
	if err != nil {
		return err
	}

	for i := len(unknownRecords) - 1; i >= 0; i-- {
		r := unknownRecords[i]
		// Save the record locally
		// Note: These get methods will return cached nodes.
		block, err := r.GetBlock(ctx, t)
		if err != nil {
			return err
		}
		event, ok := block.(*cbor.Event)
		if !ok {
			event, err = cbor.EventFromNode(block)
			if err != nil {
				return fmt.Errorf("invalid event: %v", err)
			}
		}
		header, err := event.GetHeader(ctx, t, nil)
		if err != nil {
			return err
		}
		body, err := event.GetBody(ctx, t, nil)
		if err != nil {
			return err
		}
		if err = t.AddMany(ctx, []format.Node{r, event, header, body}); err != nil {
			return err
		}

		log.Debugf("put record %s (thread=%s, log=%s)", r.Cid(), id, lg.ID)

		// Notify local listeners
		if err = t.bus.SendWithTimeout(NewRecord(r, id, lg.ID), notifyTimeout); err != nil {
			return err
		}
		// Update head
		if err = t.store.SetHead(id, lg.ID, r.Cid()); err != nil {
			return err
		}
	}
	return nil
}

// getPrivKey returns the host's private key.
func (t *net) getPrivKey() crypto.PrivKey {
	return t.host.Peerstore().PrivKey(t.host.ID())
}

// createRecord creates a new record with the given body as a new event body.
func (t *net) createRecord(ctx context.Context, id thread.ID, lg thread.LogInfo, body format.Node) (core.Record, error) {
	if lg.PrivKey == nil {
		return nil, fmt.Errorf("a private-key is required to create records")
	}
	sk, err := t.store.ServiceKey(id)
	if err != nil {
		return nil, err
	}
	if sk == nil {
		return nil, fmt.Errorf("a service-key is required to create records")
	}
	rk, err := t.store.ReadKey(id)
	if err != nil {
		return nil, err
	}
	if rk == nil {
		return nil, fmt.Errorf("a read-key is required to create records")
	}
	event, err := cbor.CreateEvent(ctx, t, body, rk)
	if err != nil {
		return nil, err
	}

	return cbor.CreateRecord(ctx, t, cbor.CreateRecordConfig{
		Block:      event,
		Prev:       lg.Head,
		Key:        lg.PrivKey,
		AuthorKey:  t.getPrivKey(),
		ServiceKey: sk,
	})
}

// getLocalRecords returns local records from the given thread that are ahead of
// offset but not farther than limit.
// It is possible to reach limit before offset, meaning that the caller
// will be responsible for the remaining traversal.
func (t *net) getLocalRecords(ctx context.Context, id thread.ID, lid peer.ID, offset cid.Cid, limit int) ([]core.Record, error) {
	lg, err := t.store.GetLog(id, lid)
	if err != nil {
		return nil, err
	}
	if lg.PubKey == nil {
		return nil, fmt.Errorf("log not found")
	}
	sk, err := t.store.ServiceKey(id)
	if err != nil {
		return nil, err
	}
	if sk == nil {
		return nil, fmt.Errorf("a service-key is required to get records")
	}

	var recs []core.Record
	if limit <= 0 {
		return recs, nil
	}

	cursor := lg.Head
	for {
		if !cursor.Defined() || cursor.String() == offset.String() {
			break
		}
		r, err := cbor.GetRecord(ctx, t, cursor, sk) // Important invariant: heads are always in blockstore
		if err != nil {
			return nil, err
		}
		recs = append([]core.Record{r}, recs...)
		if len(recs) >= MaxPullLimit {
			break
		}
		cursor = r.PrevID()
	}

	return recs, nil
}

// deleteRecord remove a record from the dag service.
func (t *net) deleteRecord(ctx context.Context, rid cid.Cid, sk *sym.Key) (prev cid.Cid, err error) {
	rec, err := cbor.GetRecord(ctx, t, rid, sk)
	if err != nil {
		return
	}
	if err = cbor.RemoveRecord(ctx, t, rec); err != nil {
		return
	}
	event, err := cbor.EventFromRecord(ctx, t, rec)
	if err != nil {
		return
	}
	if err = cbor.RemoveEvent(ctx, t, event); err != nil {
		return
	}
	return rec.PrevID(), nil
}

// startPulling periodically pulls on all threads.
func (t *net) startPulling() {
	pull := func() {
		ts, err := t.store.Threads()
		if err != nil {
			log.Errorf("error listing threads: %s", err)
			return
		}
		for _, id := range ts {
			go func(id thread.ID) {
				if err := t.PullThread(t.ctx, id); err != nil {
					log.Errorf("error pulling thread %s: %s", id, err)
				}
			}(id)
		}
	}
	timer := time.NewTimer(InitialPullInterval)
	select {
	case <-timer.C:
		pull()
	case <-t.ctx.Done():
		return
	}

	tick := time.NewTicker(PullInterval)
	defer tick.Stop()

	for {
		select {
		case <-tick.C:
			pull()
		case <-t.ctx.Done():
			return
		}
	}
}

// getLog returns the log with the given thread and log id.
func (t *net) getLog(id thread.ID, lid peer.ID) (info thread.LogInfo, err error) {
	info, err = t.store.GetLog(id, lid)
	if err != nil {
		return
	}
	if info.PubKey != nil {
		return
	}
	return info, fmt.Errorf("log %s doesn't exist for thread %s", lid, id)
}

// getOwnLoad returns the log owned by the host under the given thread.
func (t *net) getOwnLog(id thread.ID) (info thread.LogInfo, err error) {
	logs, err := t.store.LogsWithKeys(id)
	if err != nil {
		return
	}
	for _, lid := range logs {
		sk, err := t.store.PrivKey(id, lid)
		if err != nil {
			return info, err
		}
		if sk != nil {
			return t.store.GetLog(id, lid)
		}
	}
	return info, nil
}

// getOrCreateOwnLoad returns the log owned by the host under the given thread.
// If no log exists, a new one is created under the given thread.
func (t *net) getOrCreateOwnLog(id thread.ID) (info thread.LogInfo, err error) {
	info, err = t.getOwnLog(id)
	if err != nil {
		return info, err
	}
	if info.PubKey != nil {
		return
	}
	info, err = createLog(t.host.ID(), nil)
	if err != nil {
		return
	}
	err = t.store.AddLog(id, info)
	return info, err
}

// createExternalLogIfNotExist creates an external log if doesn't exists. The created
// log will have cid.Undef as the current head. Is thread-safe.
func (t *net) createExternalLogIfNotExist(tid thread.ID, lid peer.ID, pubKey crypto.PubKey,
	privKey crypto.PrivKey, addrs []ma.Multiaddr) error {
	tsph := t.getThreadSemaphore(tid)
	tsph <- struct{}{}
	defer func() { <-tsph }()
	currHeads, err := t.Store().Heads(tid, lid)
	if err != nil {
		return err
	}
	if len(currHeads) == 0 {
		lginfo := thread.LogInfo{
			ID:      lid,
			PubKey:  pubKey,
			PrivKey: privKey,
			Addrs:   addrs,
			Head:    cid.Undef,
		}
		if err := t.store.AddLog(tid, lginfo); err != nil {
			return err
		}
	}
	return nil
}

// updateRecordsFromLog will fetch lid addrs for new logs & records,
// and will add them in the local peer store. It assumes  Is thread-safe.
func (t *net) updateRecordsFromLog(tid thread.ID, lid peer.ID) {
	tsph := t.getThreadSemaphore(tid)
	tsph <- struct{}{}
	defer func() { <-tsph }()
	// Get log records for this new log
	recs, err := t.server.getRecords(
		t.ctx,
		tid,
		lid,
		map[peer.ID]cid.Cid{lid: cid.Undef},
		MaxPullLimit)
	if err != nil {
		log.Error(err)
		return
	}
	for lid, rs := range recs { // @todo: verify if they're ordered since this will optimize
		for _, r := range rs {
			if err = t.putRecord(t.ctx, tid, lid, r); err != nil {
				log.Error(err)
				return
			}
		}
	}
}

// createLog creates a new log with the given peer as host.
func createLog(host peer.ID, key crypto.Key) (info thread.LogInfo, err error) {
	var ok bool
	if key == nil {
		info.PrivKey, info.PubKey, err = crypto.GenerateEd25519Key(rand.Reader)
		if err != nil {
			return
		}
	} else if info.PrivKey, ok = key.(crypto.PrivKey); ok {
		info.PubKey = info.PrivKey.GetPublic()
	} else if info.PubKey, ok = key.(crypto.PubKey); !ok {
		return info, fmt.Errorf("invalid log-key")
	}
	info.ID, err = peer.IDFromPublicKey(info.PubKey)
	if err != nil {
		return
	}
	addr, err := ma.NewMultiaddr("/" + ma.ProtocolWithCode(ma.P_P2P).Name + "/" + host.String())
	if err != nil {
		return
	}
	info.Addrs = []ma.Multiaddr{addr}
	return info, nil
}
