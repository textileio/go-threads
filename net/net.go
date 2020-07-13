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
	"github.com/textileio/go-threads/core/app"
	lstore "github.com/textileio/go-threads/core/logstore"
	core "github.com/textileio/go-threads/core/net"
	"github.com/textileio/go-threads/core/thread"
	sym "github.com/textileio/go-threads/crypto/symmetric"
	pb "github.com/textileio/go-threads/net/pb"
	"github.com/textileio/go-threads/util"
	"google.golang.org/grpc"
)

var (
	log = logging.Logger("net")

	// MaxPullLimit is the maximum page size for pulling records.
	MaxPullLimit = 10000

	// InitialPullInterval is the interval between automatic log pulls.
	InitialPullInterval = time.Second

	// PullInterval is the interval between automatic log pulls.
	PullInterval = time.Second * 10

	// notifyTimeout is the duration to wait for a subscriber to read a new record.
	notifyTimeout = time.Second * 5

	// tokenChallengeBytes is the byte length of token challenges.
	tokenChallengeBytes = 32

	// tokenChallengeTimeout is the duration of time given to an identity to complete a token challenge.
	tokenChallengeTimeout = time.Minute
)

// net is an implementation of core.DBNet.
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
	Debug  bool
	PubSub bool
}

// NewNetwork creates an instance of net from the given host and thread store.
func NewNetwork(ctx context.Context, h host.Host, bstore bs.Blockstore, ds format.DAGService, ls lstore.Logstore, conf Config, opts ...grpc.ServerOption) (app.Net, error) {
	var err error
	if conf.Debug {
		if err = util.SetLogLevels(map[string]logging.LogLevel{
			"net":      logging.LevelDebug,
			"logstore": logging.LevelDebug,
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
	t.server, err = newServer(t, conf.PubSub)
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

func (n *net) Close() (err error) {
	n.pullLock.Lock()
	defer n.pullLock.Unlock()
	// Wait for all thread pulls to finish
	for _, semaph := range n.pullLocks {
		semaph <- struct{}{}
	}

	// Close peer connections and shutdown the server
	n.server.Lock()
	defer n.server.Unlock()
	for _, c := range n.server.conns {
		if err = c.Close(); err != nil {
			log.Errorf("error closing connection: %v", err)
		}
	}
	n.rpc.GracefulStop()

	var errs []error
	weakClose := func(name string, c interface{}) {
		if cl, ok := c.(io.Closer); ok {
			if err = cl.Close(); err != nil {
				errs = append(errs, fmt.Errorf("%s error: %s", name, err))
			}
		}
	}
	weakClose("DAGService", n.DAGService)
	weakClose("host", n.host)
	weakClose("threadstore", n.store)
	if len(errs) > 0 {
		return fmt.Errorf("failed while closing net; err(s): %q", errs)
	}

	n.bus.Discard()
	n.cancel()
	return nil
}

func (n *net) Host() host.Host {
	return n.host
}

func (n *net) Store() lstore.Logstore {
	return n.store
}

func (n *net) GetHostID(_ context.Context) (peer.ID, error) {
	return n.host.ID(), nil
}

func (n *net) GetToken(ctx context.Context, identity thread.Identity) (tok thread.Token, err error) {
	msg := make([]byte, tokenChallengeBytes)
	if _, err = rand.Read(msg); err != nil {
		return
	}
	sctx, cancel := context.WithTimeout(ctx, tokenChallengeTimeout)
	defer cancel()
	sig, err := identity.Sign(sctx, msg)
	if err != nil {
		return
	}
	key := identity.GetPublic()
	if ok, err := key.Verify(msg, sig); !ok || err != nil {
		return tok, fmt.Errorf("bad signature")
	}
	return thread.NewToken(n.getPrivKey(), key)
}

func (n *net) CreateThread(_ context.Context, id thread.ID, opts ...core.NewThreadOption) (info thread.Info, err error) {
	if err = id.Validate(); err != nil {
		return
	}
	args := &core.NewThreadOptions{}
	for _, opt := range opts {
		opt(args)
	}
	// @todo: Check identity key against ACL.
	identity, err := args.Token.Validate(n.getPrivKey())
	if err != nil {
		return
	}
	if identity != nil {
		log.Debugf("creating thread with identity: %s", identity)
	}

	if err = n.ensureUnique(id, args.LogKey); err != nil {
		return
	}

	info = thread.Info{
		ID:  id,
		Key: args.ThreadKey,
	}
	if !info.Key.Defined() {
		info.Key = thread.NewRandomKey()
	}
	if err = n.store.AddThread(info); err != nil {
		return
	}
	linfo, err := createLog(n.host.ID(), args.LogKey)
	if err != nil {
		return
	}
	if err = n.store.AddLog(id, linfo); err != nil {
		return
	}
	if n.server.ps != nil {
		if err = n.server.ps.Add(id); err != nil {
			return
		}
	}
	return n.getThreadWithAddrs(id)
}

func (n *net) ensureUnique(id thread.ID, key crypto.Key) error {
	// Check if thread exists.
	thrd, err := n.store.GetThread(id)
	if errors.Is(err, lstore.ErrThreadNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	// Early out if no log key required.
	if key == nil {
		return nil
	}
	// Ensure we don't already have this log in our store.
	if _, ok := key.(crypto.PrivKey); ok {
		// Ensure we don't already have our 'own' log.
		if thrd.GetOwnLog().PrivKey != nil {
			return lstore.ErrThreadExists
		}
		return nil
	}
	pubKey, ok := key.(crypto.PubKey)
	if !ok {
		return fmt.Errorf("invalid log-key")
	}
	logID, err := peer.IDFromPublicKey(pubKey)
	if err != nil {
		return err
	}
	_, err = n.store.GetLog(id, logID)
	if err == nil {
		return lstore.ErrLogExists
	}
	if !errors.Is(err, lstore.ErrLogNotFound) {
		return err
	}
	return nil
}

func (n *net) AddThread(ctx context.Context, addr ma.Multiaddr, opts ...core.NewThreadOption) (info thread.Info, err error) {
	args := &core.NewThreadOptions{}
	for _, opt := range opts {
		opt(args)
	}
	if _, err = args.Token.Validate(n.getPrivKey()); err != nil {
		return
	}

	id, err := thread.FromAddr(addr)
	if err != nil {
		return
	}
	if err = id.Validate(); err != nil {
		return
	}
	if err = n.ensureUnique(id, args.LogKey); err != nil {
		return
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

	// Check if we're trying to dial ourselves (regardless of addr)
	addFromSelf := addri.ID == n.host.ID()
	if addFromSelf {
		// Error if we don't have the thread locally
		if _, err = n.store.GetThread(id); errors.Is(err, lstore.ErrThreadNotFound) {
			err = fmt.Errorf("cannot retrieve thread from self: %s", err)
			return
		}
	}

	// Even if we already have the thread locally, we might still need to add a new log
	if err = n.store.AddThread(thread.Info{
		ID:  id,
		Key: args.ThreadKey,
	}); err != nil {
		return
	}
	if args.ThreadKey.CanRead() || args.LogKey != nil {
		var linfo thread.LogInfo
		linfo, err = createLog(n.host.ID(), args.LogKey)
		if err != nil {
			return
		}
		if err = n.store.AddLog(id, linfo); err != nil {
			return
		}
	}

	// Skip if trying to dial ourselves (already have the logs)
	if !addFromSelf {
		if err = n.Host().Connect(ctx, *addri); err != nil {
			return
		}
		var lgs []thread.LogInfo
		lgs, err = n.server.getLogs(ctx, id, addri.ID)
		if err != nil {
			return
		}
		for _, l := range lgs {
			if err = n.createExternalLogIfNotExist(id, l.ID, l.PubKey, l.PrivKey, l.Addrs); err != nil {
				return
			}
		}
		if n.server.ps != nil {
			if err = n.server.ps.Add(id); err != nil {
				return
			}
		}
	}
	return n.getThreadWithAddrs(id)
}

func (n *net) GetThread(_ context.Context, id thread.ID, opts ...core.ThreadOption) (info thread.Info, err error) {
	if err = id.Validate(); err != nil {
		return
	}
	args := &core.ThreadOptions{}
	for _, opt := range opts {
		opt(args)
	}
	if _, err = args.Token.Validate(n.getPrivKey()); err != nil {
		return
	}
	return n.getThreadWithAddrs(id)
}

func (n *net) getThreadWithAddrs(id thread.ID) (info thread.Info, err error) {
	var tinfo thread.Info
	var peerID *ma.Component
	var threadID *ma.Component
	tinfo, err = n.store.GetThread(id)
	if err != nil {
		return
	}
	peerID, err = ma.NewComponent("p2p", n.host.ID().String())
	if err != nil {
		return
	}
	threadID, err = ma.NewComponent("thread", tinfo.ID.String())
	if err != nil {
		return
	}
	addrs := n.host.Addrs()
	res := make([]ma.Multiaddr, len(addrs))
	for i := range addrs {
		res[i] = addrs[i].Encapsulate(peerID).Encapsulate(threadID)
	}
	tinfo.Addrs = res
	return tinfo, nil
}

func (n *net) getThreadSemaphore(id thread.ID) chan struct{} {
	var ptl chan struct{}
	var ok bool
	n.pullLock.Lock()
	defer n.pullLock.Unlock()
	if ptl, ok = n.pullLocks[id]; !ok {
		ptl = make(chan struct{}, 1)
		n.pullLocks[id] = ptl
	}
	return ptl
}

func (n *net) PullThread(ctx context.Context, id thread.ID, opts ...core.ThreadOption) error {
	if err := id.Validate(); err != nil {
		return err
	}
	args := &core.ThreadOptions{}
	for _, opt := range opts {
		opt(args)
	}
	if _, err := args.Token.Validate(n.getPrivKey()); err != nil {
		return err
	}
	return n.pullThread(ctx, id)
}

func (n *net) pullThread(ctx context.Context, id thread.ID) error {
	log.Debugf("pulling thread %s...", id)
	ptl := n.getThreadSemaphore(id)
	select {
	case ptl <- struct{}{}:
		err := n.pullThreadUnsafe(ctx, id)
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

// pullThreadUnsafe for new records.
// This method is internal and *not* thread-safe. It assumes we currently own the thread-lock.
func (n *net) pullThreadUnsafe(ctx context.Context, id thread.ID) error {
	info, err := n.store.GetThread(id)
	if err != nil {
		return err
	}

	// Gather offsets for each log
	offsets := make(map[peer.ID]cid.Cid)
	for _, lg := range info.Logs {
		var has bool
		if lg.Head.Defined() {
			has, err = n.bstore.Has(lg.Head)
			if err != nil {
				return err
			}
		}
		if has {
			offsets[lg.ID] = lg.Head
		} else {
			offsets[lg.ID] = cid.Undef
		}
	}
	var lock sync.Mutex
	var fetchedRcs []map[peer.ID][]core.Record
	wg := sync.WaitGroup{}
	for _, lg := range info.Logs {
		wg.Add(1)
		go func(lg thread.LogInfo) {
			defer wg.Done()
			// Pull from addresses
			recs, err := n.server.getRecords(ctx, id, lg.ID, offsets, MaxPullLimit)
			if err != nil {
				log.Error(err)
				return
			}
			lock.Lock()
			fetchedRcs = append(fetchedRcs, recs)
			lock.Unlock()
		}(lg)
	}
	wg.Wait()
	for _, recs := range fetchedRcs {
		for lid, rs := range recs {
			for _, r := range rs {
				if err = n.putRecord(ctx, id, lid, r); err != nil {
					log.Error(err)
					return err
				}
			}
		}
	}

	return nil
}

func (n *net) DeleteThread(ctx context.Context, id thread.ID, opts ...core.ThreadOption) error {
	if err := id.Validate(); err != nil {
		return err
	}
	args := &core.ThreadOptions{}
	for _, opt := range opts {
		opt(args)
	}
	if _, err := args.Token.Validate(n.getPrivKey()); err != nil {
		return err
	}

	log.Debugf("deleting thread %s...", id)
	ptl := n.getThreadSemaphore(id)
	select {
	case ptl <- struct{}{}: // Must block in case the thread is being pulled
		err := n.deleteThread(ctx, id)
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
func (n *net) deleteThread(ctx context.Context, id thread.ID) error {
	if n.server.ps != nil {
		if err := n.server.ps.Remove(id); err != nil {
			return err
		}
	}

	info, err := n.store.GetThread(id)
	if err != nil {
		return err
	}
	for _, lg := range info.Logs { // Walk logs, removing record and event nodes
		head := lg.Head
		for head.Defined() {
			head, err = n.deleteRecord(ctx, head, info.Key.Service())
			if err != nil {
				return err
			}
		}
	}

	return n.store.DeleteThread(id) // Delete logstore keys, addresses, and heads
}

func (n *net) AddReplicator(ctx context.Context, id thread.ID, paddr ma.Multiaddr, opts ...core.ThreadOption) (pid peer.ID, err error) {
	if err = id.Validate(); err != nil {
		return
	}
	args := &core.ThreadOptions{}
	for _, opt := range opts {
		opt(args)
	}
	if _, err = args.Token.Validate(n.getPrivKey()); err != nil {
		return
	}

	info, err := n.store.GetThread(id)
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
	managedLogs, err := n.store.GetManagedLogs(info.ID)
	if err != nil {
		return
	}
	for _, lg := range managedLogs {
		if err = n.store.AddAddr(info.ID, lg.ID, addr, pstore.PermanentAddrTTL); err != nil {
			return
		}
	}
	info, err = n.store.GetThread(info.ID) // Update info
	if err != nil {
		return
	}

	// Check if we're dialing ourselves (regardless of addr)
	if pid != n.host.ID() {
		// If not, update peerstore address
		var dialable ma.Multiaddr
		dialable, err = getDialable(paddr)
		if err == nil {
			n.host.Peerstore().AddAddr(pid, dialable, pstore.PermanentAddrTTL)
		} else {
			log.Warnf("peer %s address requires a DHT lookup", pid)
		}

		// Send all logs to the new replicator
		for _, l := range info.Logs {
			if err = n.server.pushLog(ctx, info.ID, l, pid, info.Key.Service(), nil); err != nil {
				for _, lg := range managedLogs {
					// Rollback this log only and then bail
					if lg.ID == l.ID {
						if err := n.store.SetAddrs(info.ID, lg.ID, lg.Addrs, pstore.PermanentAddrTTL); err != nil {
							log.Errorf("error rolling back log address change: %s", err)
						}
						break
					}
				}
				return
			}
		}
	}

	// Send the updated log(s) to peers
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
			if pid.String() == n.host.ID().String() {
				return
			}
			for _, lg := range managedLogs {
				if err = n.server.pushLog(ctx, info.ID, lg, pid, nil, nil); err != nil {
					log.Errorf("error pushing log %s to %s", lg.ID, p)
				}
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

func (n *net) CreateRecord(ctx context.Context, id thread.ID, body format.Node, opts ...core.ThreadOption) (r core.ThreadRecord, err error) {
	if err = id.Validate(); err != nil {
		return
	}
	args := &core.ThreadOptions{}
	for _, opt := range opts {
		opt(args)
	}
	pk, err := args.Token.Validate(n.getPrivKey())
	if err != nil {
		return
	}

	lg, err := n.getOrCreateOwnLog(id)
	if err != nil {
		return
	}
	rec, err := n.newRecord(ctx, id, lg, body, pk)
	if err != nil {
		return
	}
	if err = n.store.SetHead(id, lg.ID, rec.Cid()); err != nil {
		return nil, err
	}

	log.Debugf("added record %s (thread=%s, log=%s)", rec.Cid(), id, lg.ID)

	r, err = NewRecord(rec, id, lg.ID)
	if err != nil {
		return
	}
	if err = n.bus.SendWithTimeout(r, notifyTimeout); err != nil {
		return
	}
	if err = n.server.pushRecord(ctx, id, lg.ID, rec); err != nil {
		return
	}
	return r, nil
}

func (n *net) AddRecord(ctx context.Context, id thread.ID, lid peer.ID, rec core.Record, opts ...core.ThreadOption) error {
	if err := id.Validate(); err != nil {
		return err
	}
	args := &core.ThreadOptions{}
	for _, opt := range opts {
		opt(args)
	}
	if _, err := args.Token.Validate(n.getPrivKey()); err != nil {
		return err
	}

	logpk, err := n.store.PubKey(id, lid)
	if err != nil {
		return err
	}
	if logpk == nil {
		return lstore.ErrLogNotFound
	}

	knownRecord, err := n.bstore.Has(rec.Cid())
	if err != nil {
		return err
	}
	if knownRecord {
		return nil
	}

	if err = rec.Verify(logpk); err != nil {
		return err
	}
	if err = n.PutRecord(ctx, id, lid, rec); err != nil {
		return err
	}
	return n.server.pushRecord(ctx, id, lid, rec)
}

func (n *net) GetRecord(ctx context.Context, id thread.ID, rid cid.Cid, opts ...core.ThreadOption) (core.Record, error) {
	if err := id.Validate(); err != nil {
		return nil, err
	}
	args := &core.ThreadOptions{}
	for _, opt := range opts {
		opt(args)
	}
	if _, err := args.Token.Validate(n.getPrivKey()); err != nil {
		return nil, err
	}
	return n.getRecord(ctx, id, rid)
}

func (n *net) getRecord(ctx context.Context, id thread.ID, rid cid.Cid) (core.Record, error) {
	sk, err := n.store.ServiceKey(id)
	if err != nil {
		return nil, err
	}
	if sk == nil {
		return nil, fmt.Errorf("a service-key is required to get records")
	}
	return cbor.GetRecord(ctx, n, rid, sk)
}

// Record implements core.Record. The most basic component of a Log.
type Record struct {
	core.Record
	threadID thread.ID
	logID    peer.ID
}

// NewRecord returns a record with the given values.
func NewRecord(r core.Record, id thread.ID, lid peer.ID) (core.ThreadRecord, error) {
	if err := id.Validate(); err != nil {
		return nil, err
	}
	return &Record{Record: r, threadID: id, logID: lid}, nil
}

func (r *Record) Value() core.Record {
	return r
}

func (r *Record) ThreadID() thread.ID {
	return r.threadID
}

func (r *Record) LogID() peer.ID {
	return r.logID
}

func (n *net) Subscribe(ctx context.Context, opts ...core.SubOption) (<-chan core.ThreadRecord, error) {
	args := &core.SubOptions{}
	for _, opt := range opts {
		opt(args)
	}
	if _, err := args.Token.Validate(n.getPrivKey()); err != nil {
		return nil, err
	}

	filter := make(map[thread.ID]struct{})
	for _, id := range args.ThreadIDs {
		if err := id.Validate(); err != nil {
			return nil, err
		}
		if id.Defined() {
			filter[id] = struct{}{}
		}
	}
	return n.subscribe(ctx, filter)
}

func (n *net) subscribe(ctx context.Context, filter map[thread.ID]struct{}) (<-chan core.ThreadRecord, error) {
	channel := make(chan core.ThreadRecord)
	go func() {
		defer close(channel)
		listener := n.bus.Listen()
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

func (n *net) ConnectApp(a app.App, threadID thread.ID) (*app.Connector, error) {
	if err := threadID.Validate(); err != nil {
		return nil, err
	}
	info, err := n.getThreadWithAddrs(threadID)
	if err != nil {
		return nil, fmt.Errorf("error getting thread %s: %v", threadID, err)
	}
	return app.NewConnector(a, n, info, func(ctx context.Context, id thread.ID) (<-chan core.ThreadRecord, error) {
		return n.subscribe(ctx, map[thread.ID]struct{}{id: {}})
	})
}

// PutRecord adds an existing record. This method is thread-safe
func (n *net) PutRecord(ctx context.Context, id thread.ID, lid peer.ID, rec core.Record) error {
	if err := id.Validate(); err != nil {
		return err
	}
	tsph := n.getThreadSemaphore(id)
	tsph <- struct{}{}
	defer func() { <-tsph }()
	return n.putRecord(ctx, id, lid, rec)
}

// putRecord adds an existing record. See PutOption for more.This method
// *should be thread-guarded*
func (n *net) putRecord(ctx context.Context, id thread.ID, lid peer.ID, rec core.Record) error {
	var unknownRecords []core.Record
	c := rec.Cid()
	for c.Defined() {
		exist, err := n.bstore.Has(c)
		if err != nil {
			return err
		}
		if exist {
			break
		}
		var r core.Record
		if c.String() != rec.Cid().String() {
			r, err = n.getRecord(ctx, id, c)
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
	lg, err := n.store.GetLog(id, lid)
	if err != nil {
		return err
	}

	for i := len(unknownRecords) - 1; i >= 0; i-- {
		r := unknownRecords[i]
		// Save the record locally
		// Note: These get methods will return cached nodes.
		block, err := r.GetBlock(ctx, n)
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
		header, err := event.GetHeader(ctx, n, nil)
		if err != nil {
			return err
		}
		body, err := event.GetBody(ctx, n, nil)
		if err != nil {
			return err
		}
		if err = n.AddMany(ctx, []format.Node{r, event, header, body}); err != nil {
			return err
		}

		log.Debugf("put record %s (thread=%s, log=%s)", r.Cid(), id, lg.ID)

		if err = n.store.SetHead(id, lg.ID, r.Cid()); err != nil {
			return err
		}
		record, err := NewRecord(r, id, lg.ID)
		if err != nil {
			return err
		}
		if err = n.bus.SendWithTimeout(record, notifyTimeout); err != nil {
			return err
		}
	}
	return nil
}

// newRecord creates a new record with the given body as a new event body.
func (n *net) newRecord(ctx context.Context, id thread.ID, lg thread.LogInfo, body format.Node, pk thread.PubKey) (core.Record, error) {
	if lg.PrivKey == nil {
		return nil, fmt.Errorf("a private-key is required to create records")
	}
	sk, err := n.store.ServiceKey(id)
	if err != nil {
		return nil, err
	}
	if sk == nil {
		return nil, fmt.Errorf("a service-key is required to create records")
	}
	rk, err := n.store.ReadKey(id)
	if err != nil {
		return nil, err
	}
	if rk == nil {
		return nil, fmt.Errorf("a read-key is required to create records")
	}
	event, err := cbor.CreateEvent(ctx, n, body, rk)
	if err != nil {
		return nil, err
	}

	if pk == nil {
		pk = thread.NewLibp2pPubKey(n.getPrivKey().GetPublic())
	}
	return cbor.CreateRecord(ctx, n, cbor.CreateRecordConfig{
		Block:      event,
		Prev:       lg.Head,
		Key:        lg.PrivKey,
		PubKey:     pk,
		ServiceKey: sk,
	})
}

// getPrivKey returns the host's private key.
func (n *net) getPrivKey() crypto.PrivKey {
	return n.host.Peerstore().PrivKey(n.host.ID())
}

// getLocalRecords returns local records from the given thread that are ahead of
// offset but not farther than limit.
// It is possible to reach limit before offset, meaning that the caller
// will be responsible for the remaining traversal.
func (n *net) getLocalRecords(ctx context.Context, id thread.ID, lid peer.ID, offset cid.Cid, limit int) ([]core.Record, error) {
	lg, err := n.store.GetLog(id, lid)
	if err != nil {
		return nil, err
	}
	sk, err := n.store.ServiceKey(id)
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
		r, err := cbor.GetRecord(ctx, n, cursor, sk) // Important invariant: heads are always in blockstore
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
func (n *net) deleteRecord(ctx context.Context, rid cid.Cid, sk *sym.Key) (prev cid.Cid, err error) {
	rec, err := cbor.GetRecord(ctx, n, rid, sk)
	if err != nil {
		return
	}
	if err = cbor.RemoveRecord(ctx, n, rec); err != nil {
		return
	}
	event, err := cbor.EventFromRecord(ctx, n, rec)
	if err != nil {
		return
	}
	if err = cbor.RemoveEvent(ctx, n, event); err != nil {
		return
	}
	return rec.PrevID(), nil
}

// startPulling periodically pulls on all threads.
func (n *net) startPulling() {
	pull := func() {
		ts, err := n.store.Threads()
		if err != nil {
			log.Errorf("error listing threads: %s", err)
			return
		}
		for _, id := range ts {
			go func(id thread.ID) {
				if err := n.pullThread(n.ctx, id); err != nil {
					log.Errorf("error pulling thread %s: %s", id, err)
				}
			}(id)
		}
	}
	timer := time.NewTimer(InitialPullInterval)
	select {
	case <-timer.C:
		pull()
	case <-n.ctx.Done():
		return
	}

	tick := time.NewTicker(PullInterval)
	defer tick.Stop()

	for {
		select {
		case <-tick.C:
			pull()
		case <-n.ctx.Done():
			return
		}
	}
}

// getOrCreateOwnLoad returns the first log 'owned' by the host under the given thread.
// If no log exists, a new one is created under the given thread and returned.
// This is a strict 'ownership' check vs returning managed logs.
func (n *net) getOrCreateOwnLog(id thread.ID) (info thread.LogInfo, err error) {
	thread, err := n.store.GetThread(id)
	if err != nil {
		return
	}
	ownLog := thread.GetOwnLog()
	if ownLog != nil {
		info = *ownLog
		return
	}
	info, err = createLog(n.host.ID(), nil)
	if err != nil {
		return
	}
	err = n.store.AddLog(id, info)
	return
}

// createExternalLogIfNotExist creates an external log if doesn't exists. The created
// log will have cid.Undef as the current head. Is thread-safe.
func (n *net) createExternalLogIfNotExist(tid thread.ID, lid peer.ID, pubKey crypto.PubKey,
	privKey crypto.PrivKey, addrs []ma.Multiaddr) error {
	tsph := n.getThreadSemaphore(tid)
	tsph <- struct{}{}
	defer func() { <-tsph }()
	currHeads, err := n.Store().Heads(tid, lid)
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
		if err := n.store.AddLog(tid, lginfo); err != nil {
			return err
		}
	}
	return nil
}

// updateRecordsFromLog will fetch lid addrs for new logs & records,
// and will add them in the local peer store. It assumes  Is thread-safe.
func (n *net) updateRecordsFromLog(tid thread.ID, lid peer.ID) {
	tsph := n.getThreadSemaphore(tid)
	tsph <- struct{}{}
	defer func() { <-tsph }()
	// Get log records for this new log
	recs, err := n.server.getRecords(
		n.ctx,
		tid,
		lid,
		map[peer.ID]cid.Cid{lid: cid.Undef},
		MaxPullLimit)
	if err != nil {
		log.Error(err)
		return
	}
	for lid, rs := range recs {
		for _, r := range rs {
			if err = n.putRecord(n.ctx, tid, lid, r); err != nil {
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
	// If we're creating the log, we're 'managing' it
	info.Managed = true
	return info, nil
}
