package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/improbable-eng/grpc-web/go/grpcweb"
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
	core "github.com/textileio/go-threads/core/service"
	"github.com/textileio/go-threads/core/thread"
	pb "github.com/textileio/go-threads/service/pb"
	"github.com/textileio/go-threads/util"
	"google.golang.org/grpc"
)

var (
	log = logging.Logger("threadservice")

	// MaxPullLimit is the maximum page size for pulling records.
	MaxPullLimit = 10000

	// InitialPullInterval is the interval between automatic log pulls.
	InitialPullInterval = time.Second

	// PullInterval is the interval between automatic log pulls.
	PullInterval = time.Second * 10
)

// service is an implementation of core.Service.
type service struct {
	format.DAGService
	host   host.Host
	bstore bs.Blockstore

	store lstore.Logstore

	rpc    *grpc.Server
	proxy  *http.Server
	server *server
	bus    *broadcast.Broadcaster

	ctx    context.Context
	cancel context.CancelFunc

	pullLock  sync.Mutex
	pullLocks map[thread.ID]chan struct{}
}

// Config is used to specify thread instance options.
type Config struct {
	ProxyAddr ma.Multiaddr
	Debug     bool
}

// NewService creates an instance of service from the given host and thread store.
func NewService(
	ctx context.Context,
	h host.Host,
	bstore bs.Blockstore,
	ds format.DAGService,
	ls lstore.Logstore,
	conf Config,
) (core.Service, error) {
	var err error
	if conf.Debug {
		err = util.SetLogLevels(map[string]logging.LogLevel{
			"threadservice": logging.LevelDebug,
			"logstore":      logging.LevelDebug,
		})
		if err != nil {
			return nil, err
		}
	}

	ctx, cancel := context.WithCancel(ctx)
	t := &service{
		DAGService: ds,
		host:       h,
		bstore:     bstore,
		store:      ls,
		rpc:        grpc.NewServer(),
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
		t.rpc.Serve(listener)
	}()

	// Start a web RPC proxy
	webrpc := grpcweb.WrapServer(
		t.rpc,
		grpcweb.WithOriginFunc(func(origin string) bool {
			return true
		}),
		grpcweb.WithWebsockets(true),
		grpcweb.WithWebsocketOriginFunc(func(req *http.Request) bool {
			return true
		}))
	proxyAddr, err := util.TCPAddrFromMultiAddr(conf.ProxyAddr)
	if err != nil {
		return nil, err
	}
	t.proxy = &http.Server{
		Addr: proxyAddr,
	}
	t.proxy.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if webrpc.IsGrpcWebRequest(r) ||
			webrpc.IsAcceptableGrpcCorsRequest(r) ||
			webrpc.IsGrpcWebSocketRequest(r) {
			webrpc.ServeHTTP(w, r)
		}
	})

	errc := make(chan error)
	go func() {
		errc <- t.proxy.ListenAndServe()
		close(errc)
	}()
	go func() {
		for err := range errc {
			if err != nil {
				if err == http.ErrServerClosed {
					break
				} else {
					log.Errorf("proxy error: %s", err)
				}
			}
		}
		log.Info("proxy was shutdown")
	}()

	go t.startPulling()

	return t, nil
}

// Close the service instance.
func (t *service) Close() (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := t.proxy.Shutdown(ctx); err != nil {
		log.Errorf("error shutting down proxy: %s", err)
	}

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
		return fmt.Errorf("failed while closing service; err(s): %q", errs)
	}

	t.pullLock.Lock()
	defer t.pullLock.Unlock()
	// Wait for all thread pulls to finish
	for _, semaph := range t.pullLocks {
		semaph <- struct{}{}
	}

	return nil
}

// Host returns the underlying libp2p host.
func (t *service) Host() host.Host {
	return t.host
}

// Store returns the threadstore.
func (t *service) Store() lstore.Logstore {
	return t.store
}

// AddThread from a multiaddress.
func (t *service) AddThread(
	ctx context.Context,
	addr ma.Multiaddr,
	opts ...core.AddOption,
) (info thread.Info, err error) {
	args := &core.AddOptions{}
	for _, opt := range opts {
		opt(args)
	}

	idstr, err := addr.ValueForProtocol(thread.Code)
	if err != nil {
		return
	}
	id, err := thread.Decode(idstr)
	if err != nil {
		return
	}
	p, err := addr.ValueForProtocol(ma.P_P2P)
	if err != nil {
		return
	}
	pid, err := peer.Decode(p)
	if err != nil {
		return
	}
	if pid.String() == t.host.ID().String() {
		return t.store.ThreadInfo(id)
	}

	if err = t.store.AddThread(thread.Info{
		ID:        id,
		FollowKey: args.FollowKey,
		ReadKey:   args.ReadKey,
	}); err != nil {
		return
	}

	threadMultiaddr, err := ma.NewComponent("thread", idstr)
	if err != nil {
		return
	}
	peerAddr := addr.Decapsulate(threadMultiaddr)
	addri, err := peer.AddrInfoFromP2pAddr(peerAddr)
	if err != nil {
		return
	}
	if err = t.Host().Connect(ctx, *addri); err != nil {
		return
	}
	lgs, err := t.server.getLogs(ctx, id, pid)
	if err != nil {
		return
	}

	// @todo: ensure not overwrite with newer info from owner?
	for _, l := range lgs {
		if err = t.createExternalLogIfNotExist(id, l.ID, l.PubKey, l.PrivKey, l.Addrs); err != nil {
			return
		}
	}

	go func() {
		if err := t.PullThread(t.ctx, id); err != nil {
			log.Errorf("error pulling thread %s: %s", id.String(), err)
		}
	}()

	return t.store.ThreadInfo(id)
}

func (t *service) getThreadSemaphore(id thread.ID) chan struct{} {
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

// PullThread for new records.
// Logs owned by this host are traversed locally.
// Remotely addressed logs are pulled from the network.
// Is thread-safe.
func (t *service) PullThread(ctx context.Context, id thread.ID) error {
	log.Debugf("pulling thread %s...", id.String())
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
		log.Warningf("pull thread %s ignored since already being pulled", id)
	}
	return nil
}

// pullThread for new records. It's internal and *not* thread-safe,
// it assumes we currently own the thread-lock.
func (t *service) pullThread(ctx context.Context, id thread.ID) error {
	lgs, err := t.getLogs(id)
	if err != nil {
		return err
	}

	// Gather offsets for each log
	offsets := make(map[peer.ID]cid.Cid)
	for _, lg := range lgs {
		offsets[lg.ID] = cid.Undef
		if len(lg.Heads) > 0 {
			has, err := t.bstore.Has(lg.Heads[0])
			if err != nil {
				return err
			}
			if has {
				offsets[lg.ID] = lg.Heads[0]
			}
		}
	}
	var fetchedRcs []map[peer.ID][]core.Record
	wg := sync.WaitGroup{}
	for _, lg := range lgs {
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

// Delete a thread (@todo).
func (t *service) DeleteThread(ctx context.Context, id thread.ID) error {
	panic("implement me")
}

// AddFollower to a thread.
func (t *service) AddFollower(ctx context.Context, id thread.ID, pid peer.ID) error {
	info, err := t.store.ThreadInfo(id)
	if err != nil {
		return err
	}
	if info.FollowKey == nil {
		return fmt.Errorf("thread not found")
	}

	for _, l := range info.Logs {
		if err = t.server.pushLog(ctx, id, l, pid, info.FollowKey, nil); err != nil {
			return err
		}
	}

	// Update local addresses
	pro := ma.ProtocolWithCode(ma.P_P2P).Name
	addr, err := ma.NewMultiaddr("/" + pro + "/" + pid.String())
	if err != nil {
		return err
	}
	ownlg, err := util.GetOwnLog(t, id)
	if err != nil {
		return err
	}
	if err = t.store.AddAddr(id, ownlg.ID, addr, pstore.PermanentAddrTTL); err != nil {
		return err
	}

	// Send the updated log to peers
	var addrs []ma.Multiaddr
	for _, l := range info.Logs {
		laddrs, err := t.store.Addrs(id, l)
		if err != nil {
			return err
		}
		addrs = append(addrs, laddrs...)
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

			if err = t.server.pushLog(ctx, id, ownlg.ID, pid, nil, nil); err != nil {
				log.Errorf("error pushing log %s to %s", ownlg.ID, p)
			}
		}(addr)
	}

	wg.Wait()
	return nil
}

// AddRecord with body. See AddOption for more.
func (t *service) AddRecord(ctx context.Context, id thread.ID, body format.Node) (r core.ThreadRecord, err error) {
	// Get or create a log for the new node
	lg, err := util.GetOrCreateOwnLog(t, id)
	if err != nil {
		return
	}

	// Write a record locally
	rec, err := t.createRecord(ctx, id, lg, body)
	if err != nil {
		return
	}

	log.Debugf("added record %s (thread=%s, log=%s)", rec.Cid().String(), id, lg.ID)

	// Notify local listeners
	r = &record{
		Record:   rec,
		threadID: id,
		logID:    lg.ID,
	}
	if err = t.bus.SendWithTimeout(r, time.Second*3); err != nil {
		return
	}

	// Push out the new record
	err = t.server.pushRecord(ctx, id, lg.ID, rec)
	if err != nil {
		return
	}

	return r, nil
}

// GetRecord returns the record at cid.
func (t *service) GetRecord(ctx context.Context, id thread.ID, rid cid.Cid) (core.Record, error) {
	fk, err := t.store.FollowKey(id)
	if err != nil {
		return nil, err
	}
	if fk == nil {
		return nil, fmt.Errorf("a follow-key is required to get records")
	}
	return cbor.GetRecord(ctx, t, rid, fk)
}

// record wraps a core.Record within a thread and log context.
type record struct {
	core.Record
	threadID thread.ID
	logID    peer.ID
}

// Value returns the underlying record.
func (r *record) Value() core.Record {
	return r
}

// ThreadID returns the record's thread ID.
func (r *record) ThreadID() thread.ID {
	return r.threadID
}

// LogID returns the record's log ID.
func (r *record) LogID() peer.ID {
	return r.logID
}

// subscription receives thread record updates.
type subscription struct {
	l  *broadcast.Listener
	ch chan core.ThreadRecord
}

// Discard closes the subscription, disabling the reception of further records.
func (l *subscription) Discard() {
	l.l.Discard()
}

// Channel returns the channel that receives records.
func (l *subscription) Channel() <-chan core.ThreadRecord {
	return l.ch
}

// Subscribe returns a read-only channel of records.
func (t *service) Subscribe(opts ...core.SubOption) core.Subscription {
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
	listener := &subscription{
		l:  t.bus.Listen(),
		ch: make(chan core.ThreadRecord),
	}
	go func() {
		for i := range listener.l.Channel() {
			r, ok := i.(*record)
			if ok {
				if len(filter) > 0 {
					if _, ok := filter[r.threadID]; ok {
						listener.ch <- r
					}
				} else {
					listener.ch <- r
				}
			} else {
				log.Warning("listener received a non-record value")
			}
		}
		close(listener.ch)
	}()
	return listener
}

// PutRecord adds an existing record. This method is thread-safe
func (t *service) PutRecord(ctx context.Context, id thread.ID, lid peer.ID, rec core.Record) error {
	tsph := t.getThreadSemaphore(id)
	tsph <- struct{}{}
	defer func() { <-tsph }()
	return t.putRecord(ctx, id, lid, rec)
}

// putRecord adds an existing record. See PutOption for more.This method
// *should be thread-guarded*
func (t *service) putRecord(ctx context.Context, id thread.ID, lid peer.ID, rec core.Record) error {
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
	lg, err := util.GetLog(t, id, lid)
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
		err = t.AddMany(ctx, []format.Node{r, event, header, body})
		if err != nil {
			return err
		}

		log.Debugf("put record %s (thread=%s, log=%s)", r.Cid().String(), id, lg.ID)

		// Notify local listeners
		err = t.bus.SendWithTimeout(&record{
			Record:   r,
			threadID: id,
			logID:    lg.ID,
		}, time.Second*5)
		if err != nil {
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
func (t *service) getPrivKey() crypto.PrivKey {
	return t.host.Peerstore().PrivKey(t.host.ID())
}

// createRecord creates a new record with the given body as a new event body.
func (t *service) createRecord(
	ctx context.Context,
	id thread.ID,
	lg thread.LogInfo,
	body format.Node,
) (core.Record, error) {
	fk, err := t.store.FollowKey(id)
	if err != nil {
		return nil, err
	}
	if fk == nil {
		return nil, fmt.Errorf("a follow-key is required to create records")
	}
	rk, err := t.store.ReadKey(id)
	if err != nil {
		return nil, err
	}
	if rk == nil {
		return nil, fmt.Errorf("a read-key is required to create records")
	}
	event, err := cbor.NewEvent(ctx, t, body, rk)
	if err != nil {
		return nil, err
	}

	prev := cid.Undef
	if len(lg.Heads) != 0 {
		prev = lg.Heads[0]
	}
	rec, err := cbor.NewRecord(ctx, t, event, prev, lg.PrivKey, fk)
	if err != nil {
		return nil, err
	}

	// Update head
	if err = t.store.SetHead(id, lg.ID, rec.Cid()); err != nil {
		return nil, err
	}

	return rec, nil
}

// getLogs returns info about the logs in the given thread.
func (t *service) getLogs(id thread.ID) ([]thread.LogInfo, error) {
	lgs := make([]thread.LogInfo, 0)
	ti, err := t.store.ThreadInfo(id)
	if err != nil {
		return nil, err
	}
	for _, lid := range ti.Logs {
		lg, err := t.store.LogInfo(id, lid)
		if err != nil {
			return nil, err
		}
		lgs = append(lgs, lg)
	}
	return lgs, nil
}

// getLocalRecords returns local records from the given thread that are ahead of
// offset but not farther than limit.
// It is possible to reach limit before offset, meaning that the caller
// will be responsible for the remaining traversal.
func (t *service) getLocalRecords(
	ctx context.Context,
	id thread.ID,
	lid peer.ID,
	offset cid.Cid,
	limit int,
) ([]core.Record, error) {
	lg, err := t.store.LogInfo(id, lid)
	if err != nil {
		return nil, err
	}
	if lg.PubKey == nil {
		return nil, fmt.Errorf("log not found")
	}
	fk, err := t.store.FollowKey(id)
	if err != nil {
		return nil, err
	}
	if fk == nil {
		return nil, fmt.Errorf("a follow-key is required to get records")
	}

	var recs []core.Record
	if limit <= 0 {
		return recs, nil
	}

	if len(lg.Heads) == 0 {
		log.Warning("pull found empty log")
		return []core.Record{}, nil
	}

	if len(lg.Heads) != 1 {
		return nil, fmt.Errorf("log head must reference exactly one node")
	}
	cursor := lg.Heads[0]
	for {
		if !cursor.Defined() || cursor.String() == offset.String() {
			break
		}
		r, err := cbor.GetRecord(ctx, t, cursor, fk) // Important invariant: heads are always in blockstore
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

// startPulling periodically pulls on all threads.
func (t *service) startPulling() {
	pull := func() {
		ts, err := t.store.Threads()
		if err != nil {
			log.Errorf("error listing threads: %s", err)
			return
		}
		for _, id := range ts {
			go func(id thread.ID) {
				if err := t.PullThread(t.ctx, id); err != nil {
					log.Errorf("error pulling thread %s: %s", id.String(), err)
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

// createExternalLogIfNotExist creates an external log if doesn't exists. The created
// log will have cid.Undef as the current head. Is thread-safe.
func (t *service) createExternalLogIfNotExist(tid thread.ID, lid peer.ID, pubKey crypto.PubKey,
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
			Heads:   []cid.Cid{},
		}
		if err := t.store.AddLog(tid, lginfo); err != nil {
			return err
		}
	}
	return nil
}

// updateRecordsFromLog will fetch lid addrs for new logs & records,
// and will add them in the local peer store. It assumes  Is thread-safe.
func (t *service) updateRecordsFromLog(tid thread.ID, lid peer.ID) {
	tsph := t.getThreadSemaphore(tid)
	tsph <- struct{}{}
	defer func() { <-tsph }()
	// Get log records for this new log
	recs, err := t.server.getRecords(
		t.ctx,
		tid,
		lid,
		map[peer.ID]cid.Cid{lid: cid.Undef}, // (jsign): shouldn't this be current head?
		MaxPullLimit)
	if err != nil {
		log.Error(err)
		return
	}
	for lid, rs := range recs { // ToDo: verify if they're ordered since this will optimize
		for _, r := range rs {
			if err = t.putRecord(t.ctx, tid, lid, r); err != nil {
				log.Error(err)
				return
			}
		}
	}
}
