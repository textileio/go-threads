package threads

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	bserv "github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	format "github.com/ipfs/go-ipld-format"
	logging "github.com/ipfs/go-log"
	ic "github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	gostream "github.com/libp2p/go-libp2p-gostream"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/textileio/go-textile-core/broadcast"
	"github.com/textileio/go-textile-core/crypto"
	"github.com/textileio/go-textile-core/thread"
	tserv "github.com/textileio/go-textile-core/threadservice"
	tstore "github.com/textileio/go-textile-core/threadstore"
	"github.com/textileio/go-textile-threads/cbor"
	pb "github.com/textileio/go-textile-threads/pb"
	"github.com/textileio/go-textile-threads/util"
	logger "github.com/whyrusleeping/go-logging"
	"google.golang.org/grpc"
)

var log = logging.Logger("threads")

const (
	// IPEL is the protocol slug.
	IPEL = "ipel"
	// IPELCode is the protocol code.
	IPELCode = 406
	// IPELVersion is the current protocol version.
	IPELVersion = "0.0.1"
	// IPELProtocol is the threads protocol tag.
	IPELProtocol protocol.ID = "/" + IPEL + "/" + IPELVersion
)

var addrProtocol = ma.Protocol{
	Name:       IPEL,
	Code:       IPELCode,
	VCode:      ma.CodeToVarint(IPELCode),
	Size:       ma.LengthPrefixedVarSize,
	Transcoder: ma.TranscoderP2P,
}

func init() {
	if err := ma.AddProtocol(addrProtocol); err != nil {
		panic(err)
	}
}

// MaxPullLimit is the maximum page size for pulling records.
var MaxPullLimit = 10000

// PullInterval is the interval between automatic log pulls.
var PullInterval = time.Second * 10

// threads is an implementation of Threadservice.
type threads struct {
	host       host.Host
	blocks     bserv.BlockService
	dagService format.DAGService
	rpc        *grpc.Server
	service    *service
	bus        *broadcast.Broadcaster
	ctx        context.Context
	cancel     context.CancelFunc
	tstore.Threadstore
}

// NewThreads creates an instance of threads from the given host and thread store.
func NewThreads(
	ctx context.Context,
	h host.Host,
	bs bserv.BlockService,
	ds format.DAGService,
	ts tstore.Threadstore,
	debug bool,
) (tserv.Threadservice, error) {
	var err error
	if debug {
		err = setLogLevels(map[string]logger.Level{
			"threads":     logger.DEBUG,
			"threadstore": logger.DEBUG,
		}, true)
		if err != nil {
			return nil, err
		}
	}

	ctx, cancel := context.WithCancel(ctx)
	t := &threads{
		host:        h,
		blocks:      bs,
		dagService:  ds,
		rpc:         grpc.NewServer(),
		bus:         broadcast.NewBroadcaster(0),
		ctx:         ctx,
		cancel:      cancel,
		Threadstore: ts,
	}
	t.service, err = newService(t)
	if err != nil {
		return nil, err
	}

	listener, err := gostream.Listen(h, IPELProtocol)
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

	weakClose("host", t.host)
	weakClose("dagservice", t.dagService)
	weakClose("threadstore", t.Threadstore)

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

// DAGService returns the underlying dag service.
func (t *threads) DAGService() format.DAGService {
	return t.dagService
}

// Add a new record by wrapping body. See AddOption for more.
func (t *threads) Add(
	ctx context.Context,
	body format.Node,
	opts ...tserv.AddOption,
) (r tserv.Record, err error) {
	// Get or create a log for the new node
	settings := tserv.AddOptions(opts...)
	lg, err := t.getOrCreateOwnLog(settings.ThreadID)
	if err != nil {
		return
	}

	// Write a record locally
	rec, err := t.createRecord(ctx, body, lg, settings)
	if err != nil {
		return
	}

	// Push out the new record
	err = t.service.push(ctx, rec, settings.ThreadID, lg.ID, settings)
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

// Put an existing record. See PutOption for more.
func (t *threads) Put(ctx context.Context, rec thread.Record, opts ...tserv.PutOption) error {
	knownRecord, err := t.blocks.Blockstore().Has(rec.Cid())
	if err != nil {
		return err
	}
	if knownRecord {
		return nil
	}

	// Get or create a log for the new rec
	settings := tserv.PutOptions(opts...)
	lg, err := t.getOrCreateLog(settings.ThreadID, settings.LogID)
	if err != nil {
		return err
	}

	// Save the record locally
	// Note: These get methods will return cached nodes.
	block, err := rec.GetBlock(ctx, t.dagService)
	if err != nil {
		return err
	}
	event, ok := block.(*cbor.Event)
	if !ok {
		return fmt.Errorf("invalid event")
	}
	header, err := event.GetHeader(ctx, t.dagService, nil)
	if err != nil {
		return err
	}
	body, err := event.GetBody(ctx, t.dagService, nil)
	if err != nil {
		return err
	}
	err = t.dagService.AddMany(ctx, []format.Node{rec, event, header, body})
	if err != nil {
		return err
	}

	// Update head
	if err = t.SetHead(settings.ThreadID, lg.ID, rec.Cid()); err != nil {
		return err
	}

	// Notify local listeners
	return t.bus.Send(&record{
		Record:   rec,
		threadID: settings.ThreadID,
		logID:    lg.ID,
	})
}

// Get returns the record at cid.
func (t *threads) Get(
	ctx context.Context,
	id thread.ID,
	lid peer.ID,
	rid cid.Cid,
) (thread.Record, error) {
	lg, err := t.LogInfo(id, lid)
	if err != nil {
		return nil, err
	}
	if lg.PubKey == nil {
		return nil, fmt.Errorf("log not found")
	}
	fk, err := crypto.ParseDecryptionKey(lg.FollowKey)
	if err != nil {
		return nil, err
	}
	return cbor.GetRecord(ctx, t.dagService, rid, fk)
}

// Pull for new records from the given thread.
// Logs owned by this host are traversed locally.
// Remotely addressed logs are pulled from the network.
func (t *threads) Pull(ctx context.Context, id thread.ID) error {
	wg := sync.WaitGroup{}
	lgs, err := t.GetLogs(id)
	if err != nil {
		return err
	}
	for _, lg := range lgs {
		wg.Add(1)
		go func(lg thread.LogInfo) {
			defer wg.Done()
			var offset cid.Cid
			if len(lg.Heads) > 0 {
				offset = lg.Heads[0]
			}
			var recs []thread.Record
			var err error
			if lg.PrivKey != nil { // This is our own log
				recs, err = t.pullLocal(ctx, id, lg.ID, offset, MaxPullLimit)
			} else {
				// Pull from addresses
				recs, err = t.service.pull(ctx, id, lg.ID, offset, MaxPullLimit)
			}
			if err != nil {
				log.Error(err)
				return
			}
			for _, r := range recs {
				err = t.Put(ctx, r, tserv.PutOpt.ThreadID(id), tserv.PutOpt.LogID(lg.ID))
				if err != nil {
					log.Error(err)
					return
				}
			}
		}(lg)
	}
	wg.Wait()
	return nil
}

// record wraps a thread.Record with thread and log context.
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

// recordListener receives record updates.
type recordListener struct {
	l  *broadcast.Listener
	ch chan tserv.Record
}

// Discard closes the RecordListener, disabling the reception of further records.
func (l *recordListener) Discard() {
	l.l.Discard()
}

// Channel returns the channel that receives broadcast records.
func (l *recordListener) Channel() <-chan tserv.Record {
	return l.ch
}

// Listen returns a read-only channel of records.
func (t *threads) Listen(opts ...tserv.ListenOption) tserv.RecordListener {
	settings := tserv.ListenOptions(opts...)
	filter := make(map[thread.ID]struct{})
	for _, tid := range settings.ThreadIDs {
		if tid.Defined() {
			filter[tid] = struct{}{}
		}
	}
	listener := &recordListener{
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

// GetLogs returns info about the logs in the given thread.
func (t *threads) GetLogs(id thread.ID) ([]thread.LogInfo, error) {
	lgs := make([]thread.LogInfo, 0)
	ti, err := t.ThreadInfo(id)
	if err != nil {
		return nil, err
	}
	for _, lid := range ti.Logs {
		lg, err := t.LogInfo(id, lid)
		if err != nil {
			return nil, err
		}
		lgs = append(lgs, lg)
	}
	return lgs, nil
}

// Delete the given thread.
func (t *threads) Delete(ctx context.Context, id thread.ID) error {
	panic("implement me")
}

// getPrivKey returns the host's private key.
func (t *threads) getPrivKey() ic.PrivKey {
	return t.host.Peerstore().PrivKey(t.host.ID())
}

// createLog call util.CreateLog.
func (t *threads) createLog() (info thread.LogInfo, err error) {
	return util.CreateLog(t.host.ID())
}

// getOrCreateLog returns the log with the given thread and log id
// If no log exists, a new one is created under the given thread.
func (t *threads) getOrCreateLog(id thread.ID, lid peer.ID) (info thread.LogInfo, err error) {
	info, err = t.LogInfo(id, lid)
	if err != nil {
		return
	}
	if info.PubKey != nil {
		return
	}
	info, err = t.createLog()
	if err != nil {
		return
	}
	err = t.AddLog(id, info)
	return
}

// getOwnLoad returns the log owned by the host under the given thread.
func (t *threads) getOwnLog(id thread.ID) (info thread.LogInfo, err error) {
	logs, err := t.LogsWithKeys(id)
	if err != nil {
		return info, err
	}
	for _, lid := range logs {
		sk, err := t.PrivKey(id, lid)
		if err != nil {
			return info, err
		}
		if sk != nil {
			li, err := t.LogInfo(id, lid)
			if err != nil {
				return info, err
			}
			return li, nil
		}
	}
	return info, nil
}

// getOrCreateOwnLoad returns the log owned by the host under the given thread.
// If no log exists, a new one is created under the given thread.
func (t *threads) getOrCreateOwnLog(id thread.ID) (info thread.LogInfo, err error) {
	info, err = t.getOwnLog(id)
	if err != nil {
		return info, err
	}
	if info.PubKey != nil {
		return
	}
	info, err = t.createLog()
	if err != nil {
		return
	}
	err = t.AddLog(id, info)
	return
}

// createRecord creates a new record with the given body as a new event body.
func (t *threads) createRecord(
	ctx context.Context,
	body format.Node,
	lg thread.LogInfo,
	settings *tserv.AddSettings,
) (thread.Record, error) {
	if settings.Key == nil {
		var key []byte
		logKey, err := t.ReadKey(settings.ThreadID, settings.KeyLog)
		if err != nil {
			return nil, err
		}
		if logKey != nil {
			key = logKey
		} else {
			key = lg.ReadKey
		}
		settings.Key, err = crypto.ParseEncryptionKey(key)
		if err != nil {
			return nil, err
		}
	}
	event, err := cbor.NewEvent(ctx, t.dagService, body, settings)
	if err != nil {
		return nil, err
	}

	prev := cid.Undef
	if len(lg.Heads) != 0 {
		prev = lg.Heads[0]
	}
	fk, err := crypto.ParseEncryptionKey(lg.FollowKey)
	if err != nil {
		return nil, err
	}
	rec, err := cbor.NewRecord(ctx, t.dagService, event, prev, lg.PrivKey, fk)
	if err != nil {
		return nil, err
	}

	// Update head
	if err = t.SetHead(settings.ThreadID, lg.ID, rec.Cid()); err != nil {
		return nil, err
	}

	return rec, nil
}

// pullLocal returns local records from the given thread that are ahead of
// offset but not farther than limit.
// It is possible to reach limit before offset, meaning that the caller
// will be responsible for the remaining traversal.
func (t *threads) pullLocal(
	ctx context.Context,
	id thread.ID,
	lid peer.ID,
	offset cid.Cid,
	limit int,
) ([]thread.Record, error) {
	lg, err := t.LogInfo(id, lid)
	if err != nil {
		return nil, err
	}
	if lg.PubKey == nil {
		return nil, fmt.Errorf("log not found")
	}
	fk, err := crypto.ParseDecryptionKey(lg.FollowKey)
	if err != nil {
		return nil, err
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
		r, err := cbor.GetRecord(ctx, t.dagService, cursor, fk)
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
			ts, err := t.Threads()
			if err != nil {
				log.Errorf("error listing threads: %s", err)
				continue
			}
			for _, tid := range ts {
				go func(id thread.ID) {
					if err := t.Pull(t.ctx, id); err != nil {
						log.Errorf("error pulling thread %s: %s", tid.String(), err)
					}
				}(tid)
			}
		case <-t.ctx.Done():
			return
		}
	}
}

// setLogLevels sets the logging levels of the given log systems.
// color controls whether or not color codes are included in the output.
func setLogLevels(systems map[string]logger.Level, color bool) error {
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
