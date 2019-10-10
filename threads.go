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
	IPEL                     = "ipel"
	IPELCode                 = 406
	IPELVersion              = "0.0.1"
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

type threads struct {
	host       host.Host
	blocks     bserv.BlockService
	service    *service
	dagService format.DAGService
	ctx        context.Context
	cancel     context.CancelFunc
	tstore.Threadstore
}

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

func (t *threads) Host() host.Host {
	return t.host
}

func (t *threads) DAGService() format.DAGService {
	return t.dagService
}

func (t *threads) Add(ctx context.Context, body format.Node, opts ...tserv.AddOption) (l peer.ID, r thread.Record, err error) {
	// Get or create a log for the new node
	settings := tserv.AddOptions(opts...)
	lg, err := t.getOrCreateOwnLog(settings.Thread)
	if err != nil {
		return
	}

	// Write a node locally
	coded, err := t.createNode(ctx, body, lg, settings)
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

func (t *threads) Pull(ctx context.Context, id thread.ID, lid peer.ID, offset cid.Cid, opts ...tserv.PullOption) ([]thread.Record, error) {
	settings := tserv.PullOptions(opts...)

	if t.PubKey(id, lid) == nil {
		return nil, fmt.Errorf("could not find log")
	}

	if t.PrivKey(id, lid) != nil { // This is our own log
		return t.pullLocal(ctx, id, lid, offset, settings.Limit)
	}

	// Pull from addresses
	return t.service.pull(ctx, id, lid, offset, settings)
}

func (t *threads) Logs(id thread.ID) []thread.LogInfo {
	lgs := make([]thread.LogInfo, 0)
	for _, lid := range t.ThreadInfo(id).Logs {
		lg := t.LogInfo(id, lid)
		lgs = append(lgs, lg)
	}
	return lgs
}

func (t *threads) Delete(ctx context.Context, id thread.ID) error {
	panic("implement me")
}

func (t *threads) getPrivKey() ic.PrivKey {
	return t.host.Peerstore().PrivKey(t.host.ID())
}

func (t *threads) createLog() (info thread.LogInfo, err error) {
	return util.CreateLog(t.host.ID())
}

func (t *threads) getOrCreateLog(id thread.ID, lid peer.ID) (info thread.LogInfo, err error) {
	info = t.LogInfo(id, lid)
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

func (t *threads) getOwnLog(id thread.ID) (info thread.LogInfo) {
	for _, lid := range t.LogsWithKeys(id) {
		if t.PrivKey(id, lid) != nil {
			info = t.LogInfo(id, lid)
			return
		}
	}
	return
}

func (t *threads) getOrCreateOwnLog(id thread.ID) (info thread.LogInfo, err error) {
	info = t.getOwnLog(id)
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

func (t *threads) createNode(ctx context.Context, body format.Node, lg thread.LogInfo, settings *tserv.AddSettings) (thread.Record, error) {
	if settings.Key == nil {
		var key []byte
		logKey := t.ReadKey(settings.Thread, settings.KeyLog)
		if logKey != nil {
			key = logKey
		} else {
			key = lg.ReadKey
		}
		var err error
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

func (t *threads) pullLocal(ctx context.Context, id thread.ID, lid peer.ID, offset cid.Cid, limit int) ([]thread.Record, error) {
	lg := t.LogInfo(id, lid)
	if lg.PubKey == nil {
		return nil, fmt.Errorf("could not find log")
	}
	fk, err := crypto.ParseDecryptionKey(lg.FollowKey)
	if err != nil {
		return nil, err
	}

	var recs []thread.Record
	if limit == 0 {
		return recs, nil
	}
	cursor := lg.Heads[0]
	for { // @todo: Max depth
		if cursor.String() == offset.String() {
			break
		}
		r, err := cbor.GetRecord(ctx, t.dagService, cursor, fk)
		if err != nil {
			return nil, err
		}
		recs = append([]thread.Record{r}, recs...)
		cursor = r.PrevID()
	}

	if limit > 0 && len(recs) > limit {
		return recs[:limit], nil
	}
	return recs, nil
}

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
