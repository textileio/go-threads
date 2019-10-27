package threads

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/ipfs/go-cid"
	bs "github.com/ipfs/go-ipfs-blockstore"
	format "github.com/ipfs/go-ipld-format"
	logging "github.com/ipfs/go-log"
	ic "github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	pstore "github.com/libp2p/go-libp2p-core/peerstore"
	gostream "github.com/libp2p/go-libp2p-gostream"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/textileio/go-textile-core/broadcast"
	"github.com/textileio/go-textile-core/thread"
	tserv "github.com/textileio/go-textile-core/threadservice"
	tstore "github.com/textileio/go-textile-core/threadstore"
	"github.com/textileio/go-textile-threads/cbor"
	pb "github.com/textileio/go-textile-threads/pb"
	"github.com/textileio/go-textile-threads/util"
	logger "github.com/whyrusleeping/go-logging"
	"google.golang.org/grpc"
)

func init() {
	ma.SwapToP2pMultiaddrs() // /ipfs -> /p2p for peer addresses
}

var log = logging.Logger("threads")

// MaxPullLimit is the maximum page size for pulling records.
var MaxPullLimit = 10000

// PullInterval is the interval between automatic log pulls.
var PullInterval = time.Second * 10

// threads is an implementation of Threadservice.
type threads struct {
	format.DAGService
	host   host.Host
	bstore bs.Blockstore

	store tstore.Threadstore

	rpc     *grpc.Server
	service *service
	bus     *broadcast.Broadcaster

	ctx    context.Context
	cancel context.CancelFunc
}

// NewThreads creates an instance of threads from the given host and thread store.
func NewThreads(
	ctx context.Context,
	h host.Host,
	bstore bs.Blockstore,
	ds format.DAGService,
	ts tstore.Threadstore,
	writer io.Writer,
	debug bool,
) (tserv.Threadservice, error) {
	var err error
	if debug {
		err = setLogLevels(map[string]logger.Level{
			"threads":     logger.DEBUG,
			"threadstore": logger.DEBUG,
		}, writer, true)
		if err != nil {
			return nil, err
		}
	}

	ctx, cancel := context.WithCancel(ctx)
	t := &threads{
		DAGService: ds,
		host:       h,
		bstore:     bstore,
		store:      ts,
		rpc:        grpc.NewServer(),
		bus:        broadcast.NewBroadcaster(0),
		ctx:        ctx,
		cancel:     cancel,
	}
	t.service, err = newService(t)
	if err != nil {
		return nil, err
	}

	listener, err := gostream.Listen(h, ThreadProtocol)
	if err != nil {
		return nil, err
	}
	go t.rpc.Serve(listener)
	pb.RegisterThreadsServer(t.rpc, t.service)

	go t.startPulling()

	return t, nil
}

// Close the threads instance.
func (t *threads) Close() (err error) {
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
		return fmt.Errorf("failed while closing threads; err(s): %q", errs)
	}
	return nil
}

// Host returns the underlying libp2p host.
func (t *threads) Host() host.Host {
	return t.host
}

func (t *threads) Store() tstore.Threadstore {
	return t.store
}

// AddThread from a multiaddress.
func (t *threads) AddThread(ctx context.Context, addr ma.Multiaddr) (info thread.Info, err error) {
	idstr, err := addr.ValueForProtocol(ThreadCode)
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
	pid, err := peer.IDB58Decode(p)
	if err != nil {
		return
	}
	if pid.String() == t.host.ID().String() {
		return t.store.ThreadInfo(id)
	}

	lgs, err := t.service.getLogs(ctx, id, pid)
	if err != nil {
		return
	}
	// @todo: ensure does not exist? or overwrite with newer info from owner?
	for _, l := range lgs {
		if err = t.store.AddLog(id, l); err != nil {
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

// PullThread for new records.
// Logs owned by this host are traversed locally.
// Remotely addressed logs are pulled from the network.
func (t *threads) PullThread(ctx context.Context, id thread.ID) error {
	log.Debugf("pulling thread %s...", id.String())

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

	wg := sync.WaitGroup{}
	for _, lg := range lgs {
		wg.Add(1)
		go func(lg thread.LogInfo) {
			defer wg.Done()
			// Pull from addresses
			recs, err := t.service.getRecords(
				ctx,
				id,
				lg.ID,
				offsets,
				MaxPullLimit)
			if err != nil {
				log.Error(err)
				return
			}
			for lid, rs := range recs {
				for _, r := range rs {
					err = t.putRecord(
						ctx,
						r,
						tserv.PutOpt.ThreadID(id),
						tserv.PutOpt.LogID(lid))
					if err != nil {
						log.Error(err)
						return
					}
				}
			}
		}(lg)
	}
	wg.Wait()
	return nil
}

// Delete a thread.
func (t *threads) DeleteThread(ctx context.Context, id thread.ID) error {
	panic("implement me")
}

// AddFollower to a thread.
func (t *threads) AddFollower(ctx context.Context, id thread.ID, pid peer.ID) error {
	info, err := t.store.ThreadInfo(id)
	if err != nil {
		return err
	}
	for _, l := range info.Logs {
		if err = t.service.pushLog(ctx, id, l, pid); err != nil {
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
			pid, err := peer.IDB58Decode(p)
			if err != nil {
				log.Error(err)
				return
			}
			if pid.String() == t.host.ID().String() {
				return
			}

			if err = t.service.pushLog(ctx, id, ownlg.ID, pid); err != nil {
				log.Errorf("error pushing log %s to %s", ownlg.ID, p)
			}
		}(addr)
	}

	wg.Wait()
	return nil
}

// AddRecord with body. See AddOption for more.
func (t *threads) AddRecord(
	ctx context.Context,
	body format.Node,
	opts ...tserv.AddOption,
) (r tserv.Record, err error) {
	// Get or create a log for the new node
	settings := tserv.AddOptions(opts...)
	lg, err := util.GetOrCreateOwnLog(t, settings.ThreadID)
	if err != nil {
		return
	}

	// Write a record locally
	rec, err := t.createRecord(ctx, body, lg, settings)
	if err != nil {
		return
	}

	log.Debugf("added record %s (thread=%s, log=%s)", rec.Cid().String(), settings.ThreadID, lg.ID)

	// Push out the new record
	err = t.service.pushRecord(ctx, rec, settings.ThreadID, lg.ID, settings)
	if err != nil {
		return
	}

	// Notify local listeners
	r = &record{
		Record:   rec,
		threadID: settings.ThreadID,
		logID:    lg.ID,
	}
	if err = t.bus.Send(r); err != nil {
		return
	}

	return r, nil
}

// GetRecord returns the record at cid.
func (t *threads) GetRecord(
	ctx context.Context,
	id thread.ID,
	lid peer.ID,
	rid cid.Cid,
) (thread.Record, error) {
	lg, err := t.store.LogInfo(id, lid)
	if err != nil {
		return nil, err
	}
	if lg.PubKey == nil {
		return nil, fmt.Errorf("log not found")
	}
	return cbor.GetRecord(ctx, t, rid, lg.FollowKey)
}

// record wraps a thread.Record within a thread and log context.
type record struct {
	thread.Record
	threadID thread.ID
	logID    peer.ID
}

// Value returns the underlying record.
func (r *record) Value() thread.Record {
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
	ch chan tserv.Record
}

// Discard closes the subscription, disabling the reception of further records.
func (l *subscription) Discard() {
	l.l.Discard()
}

// Channel returns the channel that receives records.
func (l *subscription) Channel() <-chan tserv.Record {
	return l.ch
}

// Subscribe returns a read-only channel of records.
func (t *threads) Subscribe(opts ...tserv.SubOption) tserv.Subscription {
	settings := tserv.SubOptions(opts...)
	filter := make(map[thread.ID]struct{})
	for _, id := range settings.ThreadIDs {
		if id.Defined() {
			filter[id] = struct{}{}
		}
	}
	listener := &subscription{
		l:  t.bus.Listen(),
		ch: make(chan tserv.Record),
	}
	go func() {
		for {
			select {
			case i, ok := <-listener.l.Channel():
				if !ok {
					close(listener.ch)
					return
				}
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
		}
	}()
	return listener
}

// putRecord adds an existing record. See PutOption for more.
func (t *threads) putRecord(ctx context.Context, rec thread.Record, opts ...tserv.PutOption) error {
	knownRecord, err := t.bstore.Has(rec.Cid())
	if err != nil {
		return err
	}
	if knownRecord {
		return nil
	}

	// Get or create a log for the new rec
	settings := tserv.PutOptions(opts...)
	lg, err := util.GetOrCreateLog(t, settings.ThreadID, settings.LogID)
	if err != nil {
		return err
	}

	// Save the record locally
	// Note: These get methods will return cached nodes.
	block, err := rec.GetBlock(ctx, t)
	if err != nil {
		return err
	}
	event, ok := block.(*cbor.Event)
	if !ok {
		return fmt.Errorf("invalid event")
	}
	header, err := event.GetHeader(ctx, t, nil)
	if err != nil {
		return err
	}
	body, err := event.GetBody(ctx, t, nil)
	if err != nil {
		return err
	}
	err = t.AddMany(ctx, []format.Node{rec, event, header, body})
	if err != nil {
		return err
	}

	// Update head
	if err = t.store.SetHead(settings.ThreadID, lg.ID, rec.Cid()); err != nil {
		return err
	}

	log.Debugf("put record %s (thread=%s, log=%s)", rec.Cid().String(), settings.ThreadID, lg.ID)

	// Notify local listeners
	return t.bus.Send(&record{
		Record:   rec,
		threadID: settings.ThreadID,
		logID:    lg.ID,
	})
}

// getPrivKey returns the host's private key.
func (t *threads) getPrivKey() ic.PrivKey {
	return t.host.Peerstore().PrivKey(t.host.ID())
}

// createLog calls util.CreateLog.
func (t *threads) createLog() (thread.LogInfo, error) {
	return util.CreateLog(t.host.ID())
}

// createRecord creates a new record with the given body as a new event body.
func (t *threads) createRecord(
	ctx context.Context,
	body format.Node,
	lg thread.LogInfo,
	settings *tserv.AddSettings,
) (thread.Record, error) {
	if settings.Key == nil {
		logKey, err := t.store.ReadKey(settings.ThreadID, settings.KeyLog)
		if err != nil {
			return nil, err
		}
		if logKey != nil {
			settings.Key = logKey
		} else {
			settings.Key = lg.ReadKey
		}
	}
	event, err := cbor.NewEvent(ctx, t, body, settings)
	if err != nil {
		return nil, err
	}

	prev := cid.Undef
	if len(lg.Heads) != 0 {
		prev = lg.Heads[0]
	}
	rec, err := cbor.NewRecord(ctx, t, event, prev, lg.PrivKey, lg.FollowKey)
	if err != nil {
		return nil, err
	}

	// Update head
	if err = t.store.SetHead(settings.ThreadID, lg.ID, rec.Cid()); err != nil {
		return nil, err
	}

	return rec, nil
}

// getLogs returns info about the logs in the given thread.
func (t *threads) getLogs(id thread.ID) ([]thread.LogInfo, error) {
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
func (t *threads) getLocalRecords(
	ctx context.Context,
	id thread.ID,
	lid peer.ID,
	offset cid.Cid,
	limit int,
) ([]thread.Record, error) {
	lg, err := t.store.LogInfo(id, lid)
	if err != nil {
		return nil, err
	}
	if lg.PubKey == nil {
		return nil, fmt.Errorf("log not found")
	}

	var recs []thread.Record
	if limit <= 0 {
		return recs, nil
	}

	if len(lg.Heads) != 1 {
		return nil, fmt.Errorf("log head must reference exactly one node")
	}
	cursor := lg.Heads[0]
	for {
		if cursor.String() == offset.String() {
			break
		}
		r, err := cbor.GetRecord(ctx, t, cursor, lg.FollowKey)
		if err != nil {
			return nil, err
		}
		recs = append([]thread.Record{r}, recs...)
		if len(recs) >= MaxPullLimit {
			break
		}
		cursor = r.PrevID()
	}

	return recs, nil
}

// startPulling periodically pulls on all threads.
// @todo: Ensure that a thread is not pulled concurrently (#26).
func (t *threads) startPulling() {
	tick := time.NewTicker(PullInterval)
	defer tick.Stop()

	for {
		select {
		case <-tick.C:
			ts, err := t.store.Threads()
			if err != nil {
				log.Errorf("error listing threads: %s", err)
				continue
			}
			for _, id := range ts {
				go func(id thread.ID) {
					if err := t.PullThread(t.ctx, id); err != nil {
						log.Errorf("error pulling thread %s: %s", id.String(), err)
					}
				}(id)
			}
		case <-t.ctx.Done():
			return
		}
	}
}

// setLogLevels sets the logging levels of the given log systems.
// color controls whether or not color codes are included in the output.
func setLogLevels(systems map[string]logger.Level, writer io.Writer, color bool) error {
	if writer != nil {
		backendFile := logger.NewLogBackend(writer, "", 0)
		logger.SetBackend(backendFile)
	}

	var form string
	if color {
		form = logging.LogFormats["color"]
	} else {
		form = logging.LogFormats["nocolor"]
	}
	logger.SetFormatter(logger.MustStringFormatter(form))
	logging.SetAllLoggers(logger.ERROR)

	var err error
	for sys, level := range systems {
		if sys == "*" {
			for _, s := range logging.GetSubsystems() {
				err = logging.SetLogLevel(s, level.String())
				if err != nil {
					return err
				}
			}
		}
		err = logging.SetLogLevel(sys, level.String())
		if err != nil {
			return err
		}
	}
	return nil
}
