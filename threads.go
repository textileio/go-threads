package threads

import (
	"context"
	"fmt"
	"io"

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

// threads is an implementation of Threadservice.
type threads struct {
	host       host.Host
	blocks     bserv.BlockService
	service    *service
	dagService format.DAGService
	ctx        context.Context
	cancel     context.CancelFunc
	tstore.Threadstore
}

// NewThreads creates an instance of threads from the given host and thread store.
func NewThreads(ctx context.Context, h host.Host, bs bserv.BlockService, ds format.DAGService, ts tstore.Threadstore, debug bool) (tserv.Threadservice, error) {
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
	rpc := grpc.NewServer()
	go rpc.Serve(listener)
	pb.RegisterThreadsServer(rpc, t.service)

	return t, nil
}

// Close the threads instance.
func (t *threads) Close() (err error) {
	var errs []error
	weakClose := func(name string, c interface{}) {
		if cl, ok := c.(io.Closer); ok {
			if err = cl.Close(); err != nil {
				errs = append(errs, fmt.Errorf("%s error: %s", name, err))
			}
		}
	}

	//weakClose("host", t.host) @todo: fix panic on close
	weakClose("dagservice", t.dagService)
	weakClose("threadstore", t.Threadstore)

	if len(errs) > 0 {
		return fmt.Errorf("failed while closing threads; err(s): %q", errs)
	}

	t.cancel()
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
func (t *threads) Add(ctx context.Context, body format.Node, opts ...tserv.AddOption) (l peer.ID, r thread.Record, err error) {
	// Get or create a log for the new node
	settings := tserv.AddOptions(opts...)
	lg, err := t.getOrCreateOwnLog(settings.Thread)
	if err != nil {
		return
	}

	// Write a node locally
	coded, err := t.createRecord(ctx, body, lg, settings)
	if err != nil {
		return
	}

	// Push out the new record
	err = t.service.push(ctx, coded, settings.Thread, lg.ID, settings)
	if err != nil {
		return
	}

	return lg.ID, coded, nil
}

// Put an existing record. See PutOption for more.
func (t *threads) Put(ctx context.Context, rec thread.Record, opts ...tserv.PutOption) error {
	// Get or create a log for the new rec
	settings := tserv.PutOptions(opts...)
	lg, err := t.getOrCreateLog(settings.Thread, settings.Log)
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
	t.SetHead(settings.Thread, lg.ID, rec.Cid())
	return nil
}

// Pull for new records from the given log. See PullOption for more.
// Local results are returned if the log is owned by this host.
// Otherwise, records are pulled from the network at the log's addresses.
func (t *threads) Pull(ctx context.Context, id thread.ID, lid peer.ID, offset cid.Cid, opts ...tserv.PullOption) ([]thread.Record, error) {
	settings := tserv.PullOptions(opts...)

	pk, err := t.PubKey(id, lid)
	if err != nil {
		return nil, fmt.Errorf("couldn't fetch public key from book: %v", err)
	}
	if pk == nil {
		return nil, fmt.Errorf("could not find log")
	}

	sk, err := t.PrivKey(id, lid)
	if err != nil {
		return nil, fmt.Errorf("couldn't fetch private key from book: %v", err)
	}
	if sk != nil { // This is our own log
		return t.pullLocal(ctx, id, lid, offset, settings.Limit)
	}

	// Pull from addresses
	return t.service.pull(ctx, id, lid, offset, settings)
}

// Logs returns info about the logs in the given thread.
func (t *threads) Logs(id thread.ID) ([]thread.LogInfo, error) {
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
		return info, fmt.Errorf("couldn't fetch logs with keys: %v", err)
	}
	for _, lid := range logs {
		sk, err := t.PrivKey(id, lid)
		if err != nil {
			return info, fmt.Errorf("couldn't fetch private key from book: %v", err)
		}
		if sk != nil {
			li, err := t.LogInfo(id, lid)
			if err != nil {
				return info, fmt.Errorf("error when getting own log for thread %s: %v", id, err)
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
func (t *threads) createRecord(ctx context.Context, body format.Node, lg thread.LogInfo, settings *tserv.AddSettings) (thread.Record, error) {
	if settings.Key == nil {
		var key []byte
		logKey, err := t.ReadKey(settings.Thread, settings.KeyLog)
		if err != nil {
			return nil, fmt.Errorf("couldn't fetch read key from book: %v", err)
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

	t.SetHead(settings.Thread, lg.ID, rec.Cid())

	return rec, nil
}

// pullLocal returns local records from the given thread that are ahead of
// offset but not farther than limit.
// It is possible to reach limit before offset, meaning that the caller
// will be responsible for the remaining traversal.
func (t *threads) pullLocal(ctx context.Context, id thread.ID, lid peer.ID, offset cid.Cid, limit int) ([]thread.Record, error) {
	lg, err := t.LogInfo(id, lid)
	if err != nil {
		return nil, fmt.Errorf("error when pulling local (%s, %s): %v", id, lid, err)
	}
	if lg.PubKey == nil {
		return nil, fmt.Errorf("could not find log")
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
