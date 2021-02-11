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
	"github.com/textileio/go-threads/net/util"
	tu "github.com/textileio/go-threads/util"
	"google.golang.org/grpc"
)

var (
	log = logging.Logger("net")

	// MaxPullLimit is the maximum page size for pulling records.
	MaxPullLimit = 10000

	// PullStartAfter is the pause before thread pulling process starts.
	PullStartAfter = time.Second

	// InitialPullInterval is the interval for the first iteration of log pulls.
	InitialPullInterval = time.Second

	// PullInterval is the interval between automatic log pulls.
	PullInterval = time.Second * 10

	// EventBusCapacity is the buffer size of local event bus listeners.
	EventBusCapacity = 1

	// notifyTimeout is the duration to wait for a subscriber to read a new record.
	notifyTimeout = time.Second * 5

	// tokenChallengeBytes is the byte length of token challenges.
	tokenChallengeBytes = 32

	// tokenChallengeTimeout is the duration of time given to an identity to complete a token challenge.
	tokenChallengeTimeout = time.Minute
)

var (
	_ util.SemaphoreKey = (*semaThreadPull)(nil)
	_ util.SemaphoreKey = (*semaThreadUpdate)(nil)
)

// semaphore protecting thread info updates
type semaThreadUpdate thread.ID

func (t semaThreadUpdate) Key() string {
	return "tu:" + string(t)
}

// semaphore eliminating concurrent pulls
type semaThreadPull thread.ID

func (t semaThreadPull) Key() string {
	return "tp:" + string(t)
}

// net is an implementation of core.DBNet.
type net struct {
	format.DAGService
	host   host.Host
	bstore bs.Blockstore

	store lstore.Logstore

	rpc    *grpc.Server
	server *server
	bus    *broadcast.Broadcaster

	connectors map[thread.ID]*app.Connector
	connLock   sync.Mutex

	semaphores *util.SemaphorePool

	ctx    context.Context
	cancel context.CancelFunc
}

// Config is used to specify thread instance options.
type Config struct {
	Debug  bool
	PubSub bool
}

// NewNetwork creates an instance of net from the given host and thread store.
func NewNetwork(
	ctx context.Context,
	h host.Host,
	bstore bs.Blockstore,
	ds format.DAGService,
	ls lstore.Logstore,
	conf Config,
	serverOptions []grpc.ServerOption,
	dialOptions []grpc.DialOption,
) (app.Net, error) {
	var err error
	if conf.Debug {
		if err = tu.SetLogLevels(map[string]logging.LogLevel{
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
		rpc:        grpc.NewServer(serverOptions...),
		bus:        broadcast.NewBroadcaster(EventBusCapacity),
		connectors: make(map[thread.ID]*app.Connector),
		ctx:        ctx,
		cancel:     cancel,
		semaphores: util.NewSemaphorePool(1),
	}

	t.server, err = newServer(t, conf.PubSub, dialOptions...)
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
	// Wait for all thread pulls to finish
	n.semaphores.Stop()

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
				errs = append(errs, fmt.Errorf("%s error: %v", name, err))
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

func (n *net) CreateThread(
	_ context.Context,
	id thread.ID,
	opts ...core.NewThreadOption,
) (info thread.Info, err error) {
	args := &core.NewThreadOptions{}
	for _, opt := range opts {
		opt(args)
	}
	// @todo: Check identity key against ACL.
	identity, err := n.Validate(id, args.Token, false)
	if err != nil {
		return
	}
	if identity != nil {
		log.Debugf("creating thread with identity: %s", identity)
	} else {
		identity = thread.NewLibp2pPubKey(n.getPrivKey().GetPublic())
	}

	if err = n.ensureUniqueLog(id, args.LogKey, identity); err != nil {
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
	if _, err = n.createLog(id, args.LogKey, identity); err != nil {
		return
	}
	if n.server.ps != nil {
		if err = n.server.ps.Add(id); err != nil {
			return
		}
	}
	return n.getThreadWithAddrs(id)
}

func (n *net) AddThread(
	ctx context.Context,
	addr ma.Multiaddr,
	opts ...core.NewThreadOption,
) (info thread.Info, err error) {
	args := &core.NewThreadOptions{}
	for _, opt := range opts {
		opt(args)
	}

	id, err := thread.FromAddr(addr)
	if err != nil {
		return
	}
	identity, err := n.Validate(id, args.Token, false)
	if err != nil {
		return
	}
	if identity != nil {
		log.Debugf("adding thread with identity: %s", identity)
	} else {
		identity = thread.NewLibp2pPubKey(n.getPrivKey().GetPublic())
	}

	if err = n.ensureUniqueLog(id, args.LogKey, identity); err != nil {
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
			err = fmt.Errorf("cannot retrieve thread from self: %v", err)
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
		if _, err = n.createLog(id, args.LogKey, identity); err != nil {
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
	args := &core.ThreadOptions{}
	for _, opt := range opts {
		opt(args)
	}
	if _, err = n.Validate(id, args.Token, true); err != nil {
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

func (n *net) PullThread(ctx context.Context, id thread.ID, opts ...core.ThreadOption) error {
	args := &core.ThreadOptions{}
	for _, opt := range opts {
		opt(args)
	}
	if _, err := n.Validate(id, args.Token, true); err != nil {
		return err
	}
	return n.pullThread(ctx, id)
}

// pullThread for the new records. This method is thread-safe.
func (n *net) pullThread(ctx context.Context, tid thread.ID) error {
	tps := n.semaphores.Get(semaThreadPull(tid))
	if !tps.TryAcquire() {
		log.Debugf("skip pulling thread %s: concurrent pull in progress", tid)
		return nil
	}

	offsets, err := n.threadOffsets(tid)
	if err != nil {
		tps.Release()
		return err
	}

	// Pull from addresses
	recs, err := n.server.getRecords(ctx, tid, offsets, MaxPullLimit)
	tps.Release()
	if err != nil {
		return err
	}

	for lid, rs := range recs {
		for _, r := range rs {
			if err = n.putRecord(ctx, tid, lid, r); err != nil {
				return err
			}
		}
	}

	return nil
}

func (n *net) DeleteThread(ctx context.Context, id thread.ID, opts ...core.ThreadOption) error {
	args := &core.ThreadOptions{}
	for _, opt := range opts {
		opt(args)
	}
	if _, err := n.Validate(id, args.Token, false); err != nil {
		return err
	}
	if _, ok := n.getConnectorProtected(id, args.APIToken); !ok {
		return fmt.Errorf("cannot delete thread: %w", app.ErrThreadInUse)
	}

	log.Debugf("deleting thread %s...", id)
	ts := n.semaphores.Get(semaThreadUpdate(id))

	// Must block in case the thread is being pulled
	ts.Acquire()
	err := n.deleteThread(ctx, id)
	ts.Release()

	return err
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

	return n.store.DeleteThread(id) // Delete logstore keys, addresses, heads, and metadata
}

func (n *net) AddReplicator(
	ctx context.Context,
	id thread.ID,
	paddr ma.Multiaddr,
	opts ...core.ThreadOption,
) (pid peer.ID, err error) {
	args := &core.ThreadOptions{}
	for _, opt := range opts {
		opt(args)
	}
	if _, err = n.Validate(id, args.Token, true); err != nil {
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
	peers, err := n.uniquePeers(addrs)
	if err != nil {
		return
	}

	var wg sync.WaitGroup
	for _, p := range peers {
		wg.Add(1)
		go func(pid peer.ID) {
			defer wg.Done()
			for _, lg := range managedLogs {
				if err = n.server.pushLog(ctx, info.ID, lg, pid, nil, nil); err != nil {
					log.Errorf("error pushing log %s to %s: %v", lg.ID, pid, err)
				}
			}
		}(p)
	}

	wg.Wait()
	return pid, nil
}

func (n *net) uniquePeers(addrs []ma.Multiaddr) ([]peer.ID, error) {
	var pm = make(map[peer.ID]struct{}, len(addrs))
	for _, addr := range addrs {
		pid, ok, err := n.callablePeer(addr)
		if err != nil {
			return nil, err
		} else if !ok {
			// skip calling itself
			continue
		}
		pm[pid] = struct{}{}
	}
	var ps = make([]peer.ID, 0, len(pm))
	for pid := range pm {
		ps = append(ps, pid)
	}
	return ps, nil
}

// callablePeer attempts to obtain external peer ID from the multiaddress.
func (n *net) callablePeer(addr ma.Multiaddr) (peer.ID, bool, error) {
	p, err := addr.ValueForProtocol(ma.P_P2P)
	if err != nil {
		return "", false, err
	}

	pid, err := peer.Decode(p)
	if err != nil {
		return "", false, err
	}

	if pid.String() == n.host.ID().String() {
		return pid, false, nil
	}

	return pid, true, nil
}

func getDialable(addr ma.Multiaddr) (ma.Multiaddr, error) {
	parts := strings.Split(addr.String(), "/"+ma.ProtocolWithCode(ma.P_P2P).Name)
	return ma.NewMultiaddr(parts[0])
}

func (n *net) CreateRecord(
	ctx context.Context,
	id thread.ID,
	body format.Node,
	opts ...core.ThreadOption,
) (tr core.ThreadRecord, err error) {
	args := &core.ThreadOptions{}
	for _, opt := range opts {
		opt(args)
	}
	identity, err := n.Validate(id, args.Token, false)
	if err != nil {
		return
	}
	if identity == nil {
		identity = thread.NewLibp2pPubKey(n.getPrivKey().GetPublic())
	}
	con, ok := n.getConnectorProtected(id, args.APIToken)
	if !ok {
		return nil, fmt.Errorf("cannot create record: %w", app.ErrThreadInUse)
	} else if con != nil {
		if err = con.ValidateNetRecordBody(ctx, body, identity); err != nil {
			return
		}
	}

	lg, err := n.getOrCreateLog(id, identity)
	if err != nil {
		return
	}
	r, err := n.newRecord(ctx, id, lg, body, identity)
	if err != nil {
		return
	}
	tr = NewRecord(r, id, lg.ID)
	if err = n.store.SetHead(id, lg.ID, tr.Value().Cid()); err != nil {
		return
	}
	log.Debugf("created record %s (thread=%s, log=%s)", tr.Value().Cid(), id, lg.ID)
	if err = n.bus.SendWithTimeout(tr, notifyTimeout); err != nil {
		return
	}
	if err = n.server.pushRecord(ctx, id, lg.ID, tr.Value()); err != nil {
		return
	}
	return tr, nil
}

func (n *net) AddRecord(
	ctx context.Context,
	id thread.ID,
	lid peer.ID,
	rec core.Record,
	opts ...core.ThreadOption,
) error {
	args := &core.ThreadOptions{}
	for _, opt := range opts {
		opt(args)
	}
	if _, err := n.Validate(id, args.Token, false); err != nil {
		return err
	}

	logpk, err := n.store.PubKey(id, lid)
	if err != nil {
		return err
	}
	if logpk == nil {
		return lstore.ErrLogNotFound
	}

	if knownRecord, err := n.isKnown(rec.Cid()); err != nil {
		return err
	} else if knownRecord {
		return nil
	}

	if err = rec.Verify(logpk); err != nil {
		return err
	}
	if err = n.putRecord(ctx, id, lid, rec); err != nil {
		return err
	}
	return n.server.pushRecord(ctx, id, lid, rec)
}

func (n *net) GetRecord(
	ctx context.Context,
	id thread.ID,
	rid cid.Cid,
	opts ...core.ThreadOption,
) (core.Record, error) {
	args := &core.ThreadOptions{}
	for _, opt := range opts {
		opt(args)
	}
	if _, err := n.Validate(id, args.Token, true); err != nil {
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
func NewRecord(r core.Record, id thread.ID, lid peer.ID) core.ThreadRecord {
	return &Record{Record: r, threadID: id, logID: lid}
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

	filter := make(map[thread.ID]struct{})
	for _, id := range args.ThreadIDs {
		if err := id.Validate(); err != nil {
			return nil, err
		}
		if id.Defined() {
			if _, err := n.Validate(id, args.Token, true); err != nil {
				return nil, err
			}
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

func (n *net) ConnectApp(a app.App, id thread.ID) (*app.Connector, error) {
	if err := id.Validate(); err != nil {
		return nil, err
	}
	info, err := n.getThreadWithAddrs(id)
	if err != nil {
		return nil, fmt.Errorf("error getting thread %s: %v", id, err)
	}
	con, err := app.NewConnector(a, n, info)
	if err != nil {
		return nil, fmt.Errorf("error making connector %s: %v", id, err)
	}
	n.addConnector(id, con)
	return con, nil
}

// @todo: Handle thread ACL checks against ID and readOnly.
func (n *net) Validate(id thread.ID, token thread.Token, readOnly bool) (thread.PubKey, error) {
	if err := id.Validate(); err != nil {
		return nil, err
	}
	return token.Validate(n.getPrivKey())
}

func (n *net) addConnector(id thread.ID, conn *app.Connector) {
	n.connLock.Lock()
	n.connectors[id] = conn
	n.connLock.Unlock()
}

func (n *net) getConnector(id thread.ID) (*app.Connector, bool) {
	n.connLock.Lock()
	defer n.connLock.Unlock()

	conn, exist := n.connectors[id]
	return conn, exist
}

// getConnectorProtected returns the connector tied to the thread if it exists
// and whether or not the token is valid.
func (n *net) getConnectorProtected(id thread.ID, token core.Token) (*app.Connector, bool) {
	c, exist := n.getConnector(id)
	if !exist {
		return nil, true // thread is not owned by a connector
	}
	if !token.Equal(c.Token()) {
		return nil, false
	}
	return c, true
}

// PutRecord adds an existing record. This method is thread-safe.
func (n *net) PutRecord(ctx context.Context, id thread.ID, lid peer.ID, rec core.Record) error {
	if err := id.Validate(); err != nil {
		return err
	}
	return n.putRecord(ctx, id, lid, rec)
}

// putRecord adds an existing record. This method is thread-safe.
func (n *net) putRecord(ctx context.Context, tid thread.ID, lid peer.ID, rec core.Record) error {
	unknown, head, err := n.loadUnknownRecords(ctx, tid, lid, rec)
	if err != nil {
		return fmt.Errorf("loading records failed: %w", err)
	} else if len(unknown) == 0 {
		return nil
	}

	ts := n.semaphores.Get(semaThreadUpdate(tid))
	ts.Acquire()

	// check head again to detect if some other process concurrently have changed the log
	if current, err := n.currentHead(tid, lid); err != nil {
		ts.Release()
		return fmt.Errorf("fetching head failed: %w", err)
	} else if current != head {
		ts.Release()
		return n.putRecord(ctx, tid, lid, rec)
	}

	defer ts.Release()

	connector, appConnected := n.getConnector(tid)
	for _, record := range unknown {
		if err := n.store.SetHead(tid, lid, record.Value().Cid()); err != nil {
			return fmt.Errorf("setting log head failed: %w", err)
		}

		if appConnected {
			if err := connector.HandleNetRecord(ctx, record); err != nil {
				// Future improvement notes.
				// If record handling fails there are two options available:
				// 1. Just interrupt and return error (current behaviour). Log head remains moved and some events
				//    from the record possibly won't reach reducers/listeners or even get dispatched.
				// 2. Rollback log head to the previous record. In this case record handling will be retried until
				//    success, but reducers must guarantee its idempotence and there is a chance of getting stuck
				//    with bad event and not making any progress at all.
				return fmt.Errorf("handling record failed: %w", err)
			}
		}

		// Generally broadcasting should not block for too long, i.e. we have to run it
		// under the semaphore to ensure consistent order seen by the listeners. Record
		// bursts could be overcome by adjusting listener buffers (EventBusCapacity).
		if err = n.bus.SendWithTimeout(record, notifyTimeout); err != nil {
			return err
		}
	}

	return nil
}

// Load, validate and cache all records in log between currentHead and last.
func (n *net) loadUnknownRecords(
	ctx context.Context,
	tid thread.ID,
	lid peer.ID,
	last core.Record,
) ([]core.ThreadRecord, cid.Cid, error) {
	// check if last record was already loaded and processed
	if exist, err := n.isKnown(last.Cid()); err != nil {
		return nil, cid.Undef, err
	} else if exist || !last.Cid().Defined() {
		return nil, cid.Undef, nil
	}

	var (
		c       = last.PrevID()
		unknown = []core.Record{last}
	)

	head, err := n.currentHead(tid, lid)
	if err != nil {
		return nil, head, err
	}

	// load record chain between the last and current head
	for c.Defined() {
		if c == head {
			break
		}

		r, err := n.getRecord(ctx, tid, c)
		if err != nil {
			return nil, head, err
		}

		unknown = append(unknown, r)
		c = r.PrevID()
	}

	if len(unknown) == 0 {
		return nil, head, nil
	}

	var (
		connector, appConnected = n.getConnector(tid)
		identity                = &thread.Libp2pPubKey{}
		tRecords                = make([]core.ThreadRecord, 0, len(unknown))
		readKey                 *sym.Key
		validate                bool
	)

	if appConnected {
		var err error
		if readKey, err = n.store.ReadKey(tid); err != nil {
			return nil, head, err
		} else if readKey != nil {
			validate = true
		}
	}

	for i := len(unknown) - 1; i >= 0; i-- {
		var r = unknown[i]
		block, err := r.GetBlock(ctx, n)
		if err != nil {
			return nil, head, err
		}

		event, ok := block.(*cbor.Event)
		if !ok {
			event, err = cbor.EventFromNode(block)
			if err != nil {
				return nil, head, fmt.Errorf("invalid event: %w", err)
			}
		}

		header, err := event.GetHeader(ctx, n, nil)
		if err != nil {
			return nil, head, err
		}

		body, err := event.GetBody(ctx, n, nil)
		if err != nil {
			return nil, head, err
		}

		if validate {
			dbody, err := event.GetBody(ctx, n, readKey)
			if err != nil {
				return nil, head, err
			}

			if err = identity.UnmarshalBinary(r.PubKey()); err != nil {
				return nil, head, err
			}

			if err = connector.ValidateNetRecordBody(ctx, dbody, identity); err != nil {
				return nil, head, err
			}
		}

		// store blocks locally
		if err = n.AddMany(ctx, []format.Node{r, event, header, body}); err != nil {
			return nil, head, err
		}

		tRecords = append(tRecords, NewRecord(r, tid, lid))
	}

	return tRecords, head, nil
}

func (n *net) isKnown(rec cid.Cid) (bool, error) {
	return n.bstore.Has(rec)
}

func (n *net) currentHead(tid thread.ID, lid peer.ID) (cid.Cid, error) {
	var head cid.Cid
	heads, err := n.store.Heads(tid, lid)
	if err != nil {
		return head, err
	}

	if len(heads) > 0 {
		head = heads[0]
	} else {
		head = cid.Undef
	}

	return head, nil
}

// newRecord creates a new record with the given body as a new event body.
func (n *net) newRecord(
	ctx context.Context,
	id thread.ID,
	lg thread.LogInfo,
	body format.Node,
	pk thread.PubKey,
) (core.Record, error) {
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
func (n *net) getLocalRecords(
	ctx context.Context,
	id thread.ID,
	lid peer.ID,
	offset cid.Cid,
	limit int,
) ([]core.Record, error) {
	if offset != cid.Undef {
		// ensure that we know about requested offset
		if knownRecord, err := n.isKnown(offset); err != nil {
			return nil, err
		} else if !knownRecord {
			return nil, nil
		}
	}

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

	var (
		cursor = lg.Head
		recs   []core.Record
	)

	for len(recs) < limit {
		if !cursor.Defined() || cursor.String() == offset.String() {
			break
		}
		r, err := cbor.GetRecord(ctx, n, cursor, sk) // Important invariant: heads are always in blockstore
		if err != nil {
			return nil, err
		}
		recs = append([]core.Record{r}, recs...)
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
	select {
	case <-time.After(PullStartAfter):
	case <-n.ctx.Done():
		return
	}

	// set pull cycle interval into initial value,
	// it will be redefined on the next iteration
	var interval = InitialPullInterval

PullCycle:
	for {
		ts, err := n.store.Threads()
		if err != nil {
			log.Errorf("error listing threads: %s", err)
			return
		}

		if len(ts) == 0 {
			// if there are no threads served, just wait and retry
			select {
			case <-time.After(interval):
				interval = PullInterval
				continue PullCycle
			case <-n.ctx.Done():
				return
			}
		}

		var (
			period = interval / time.Duration(len(ts))
			ticker = time.NewTicker(period)
			idx    = 0
		)

		for {
			select {
			case <-ticker.C:
				go func(tid thread.ID) {
					if err := n.pullThread(n.ctx, tid); err != nil {
						log.Errorf("error pulling thread %s: %s", tid, err)
					}
				}(ts[idx])
				idx++
				if idx >= len(ts) {
					ticker.Stop()
					interval = PullInterval
					continue PullCycle
				}

			case <-n.ctx.Done():
				ticker.Stop()
				return
			}
		}
	}
}

// createLog creates a new log with the given peer as host.
func (n *net) createLog(id thread.ID, key crypto.Key, identity thread.PubKey) (info thread.LogInfo, err error) {
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
	addr, err := ma.NewMultiaddr("/" + ma.ProtocolWithCode(ma.P_P2P).Name + "/" + n.host.ID().String())
	if err != nil {
		return
	}
	info.Addrs = []ma.Multiaddr{addr}
	// If we're creating the log, we're 'managing' it
	info.Managed = true

	// Add to thread
	if err = n.store.AddLog(id, info); err != nil {
		return info, err
	}
	lidb, err := info.ID.MarshalBinary()
	if err != nil {
		return info, err
	}
	if err = n.store.PutBytes(id, identity.String(), lidb); err != nil {
		return info, err
	}
	return info, nil
}

// getOrCreateLog returns a log for identity under the given thread.
// If no log exists, a new one is created.
func (n *net) getOrCreateLog(id thread.ID, identity thread.PubKey) (info thread.LogInfo, err error) {
	if identity == nil {
		identity = thread.NewLibp2pPubKey(n.getPrivKey().GetPublic())
	}
	lidb, err := n.store.GetBytes(id, identity.String())
	if err != nil {
		return info, err
	}
	// Check if we have an old-style "own" (unindexed) log
	if lidb == nil && identity.Equals(thread.NewLibp2pPubKey(n.getPrivKey().GetPublic())) {
		thrd, err := n.store.GetThread(id)
		if err != nil {
			return info, err
		}
		ownLog := thrd.GetFirstPrivKeyLog()
		if ownLog != nil {
			return *ownLog, nil
		}
	} else if lidb != nil {
		lid, err := peer.IDFromBytes(*lidb)
		if err != nil {
			return info, err
		}
		return n.store.GetLog(id, lid)
	}
	return n.createLog(id, nil, identity)
}

// createExternalLogIfNotExist creates an external log if doesn't exists. The created
// log will have cid.Undef as the current head. Is thread-safe.
func (n *net) createExternalLogIfNotExist(
	tid thread.ID,
	lid peer.ID,
	pubKey crypto.PubKey,
	privKey crypto.PrivKey,
	addrs []ma.Multiaddr,
) error {
	ts := n.semaphores.Get(semaThreadUpdate(tid))
	ts.Acquire()
	defer ts.Release()

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
		return n.store.AddLog(tid, lginfo)
	}
	return nil
}

// ensureUniqueLog returns a non-nil error if a log with key already exists,
// or if a log for identity already exists for the given thread.
func (n *net) ensureUniqueLog(id thread.ID, key crypto.Key, identity thread.PubKey) (err error) {
	thrd, err := n.store.GetThread(id)
	if errors.Is(err, lstore.ErrThreadNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	var lid peer.ID
	if key != nil {
		switch key.(type) {
		case crypto.PubKey:
			lid, err = peer.IDFromPublicKey(key.(crypto.PubKey))
			if err != nil {
				return err
			}
		case crypto.PrivKey:
			lid, err = peer.IDFromPrivateKey(key.(crypto.PrivKey))
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("invalid log key")
		}
	} else {
		lidb, err := n.store.GetBytes(id, identity.String())
		if err != nil {
			return err
		}
		if lidb == nil {
			// Check if we have an old-style "own" (unindexed) log
			if identity.Equals(thread.NewLibp2pPubKey(n.getPrivKey().GetPublic())) {
				if thrd.GetFirstPrivKeyLog().PrivKey != nil {
					return lstore.ErrThreadExists
				}
			}
			return nil
		}
		lid, err = peer.IDFromBytes(*lidb)
		if err != nil {
			return err
		}
	}
	_, err = n.store.GetLog(id, lid)
	if err == nil {
		return lstore.ErrLogExists
	}
	if !errors.Is(err, lstore.ErrLogNotFound) {
		return err
	}
	return nil
}

// updateRecordsFromLog will fetch lid addrs for new logs & records,
// and will add them in the local peer store. Method is thread-safe.
func (n *net) updateRecordsFromLog(tid thread.ID, lid peer.ID) {
	tps := n.semaphores.Get(semaThreadPull(tid))
	if !tps.TryAcquire() {
		log.Debugf("skip updating thread %s (log %s): concurrent pull in progress", tid, lid)
		return
	}

	// TODO after protocol change request only new log
	offsets, err := n.threadOffsets(tid)
	if err != nil {
		tps.Release()
		log.Errorf("getting offsets for thread %s failed: %v", tid, err)
		return
	}

	// request records for the new log
	offsets[lid] = cid.Undef

	recs, err := n.server.getRecords(n.ctx, tid, offsets, MaxPullLimit)
	tps.Release()
	if err != nil {
		log.Errorf("getting records from new log %s (thread %s) failed: %v", lid, tid, err)
		return
	}

	for lid, rs := range recs {
		for _, r := range rs {
			if err = n.putRecord(n.ctx, tid, lid, r); err != nil {
				log.Errorf("putting records from new log %s (thread %s) failed: %v", lid, tid, err)
				return
			}
		}
	}
}

// get offsets for all known thread's logs
func (n *net) threadOffsets(tid thread.ID) (map[peer.ID]cid.Cid, error) {
	info, err := n.store.GetThread(tid)
	if err != nil {
		return nil, err
	}
	var offsets = make(map[peer.ID]cid.Cid, len(info.Logs))
	for _, lg := range info.Logs {
		var has bool
		if lg.Head.Defined() {
			has, err = n.isKnown(lg.Head)
			if err != nil {
				return nil, err
			}
		}
		if has {
			offsets[lg.ID] = lg.Head
		} else {
			offsets[lg.ID] = cid.Undef
		}
	}
	return offsets, nil
}
