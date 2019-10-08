package threads

import (
	"context"
	"fmt"
	"io"

	"github.com/ipfs/go-cid"
	format "github.com/ipfs/go-ipld-format"
	ic "github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	gostream "github.com/libp2p/go-libp2p-gostream"
	"github.com/libp2p/go-libp2p-pubsub"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/textileio/go-textile-core/crypto"
	"github.com/textileio/go-textile-core/thread"
	tserv "github.com/textileio/go-textile-core/threadservice"
	tstore "github.com/textileio/go-textile-core/threadstore"
	"github.com/textileio/go-textile-threads/cbor"
	pb "github.com/textileio/go-textile-threads/pb"
	"github.com/textileio/go-textile-threads/util"
	"google.golang.org/grpc"
)

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
	service    *service
	pubsub     *pubsub.PubSub
	dagService format.DAGService
	ctx        context.Context
	cancel     context.CancelFunc
	tstore.Threadstore
}

func NewThreadservice(ctx context.Context, h host.Host, ds format.DAGService, ts tstore.Threadstore) (tserv.Threadservice, error) {
	ctx, cancel := context.WithCancel(ctx)
	rpc := grpc.NewServer()
	t := &threads{
		host:        h,
		dagService:  ds,
		ctx:         ctx,
		cancel:      cancel,
		Threadstore: ts,
	}
	t.service = &service{threads: t}

	listener, err := gostream.Listen(h, IPELProtocol)
	if err != nil {
		return nil, err
	}
	go rpc.Serve(listener)

	pb.RegisterThreadsServer(rpc, t.service)

	//service.pubsub, err = pubsub.NewGossipSub(service.ctx, service.host)
	//if err != nil {
	//	return nil, err
	//}

	// @todo: ts.pubsub.RegisterTopicValidator()

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

	//weakClose("server", t.server)
	//weakClose("host", t.host)
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
	log, err := t.getOrCreateOwnLog(settings.Thread)
	if err != nil {
		return
	}

	// Write a node locally
	coded, err := t.createNode(ctx, body, log, settings)
	if err != nil {
		return
	}

	var addrs []ma.Multiaddr
	// Collect known writers
	for _, lid := range t.ThreadInfo(settings.Thread).Logs {
		if lid.String() == log.ID.String() {
			continue
		}
		addrs = append(addrs, t.Addrs(settings.Thread, lid)...)
	}

	// Add additional addresses
	addrs = append(addrs, settings.Addrs...)

	// Push our the new record
	err = t.service.pushAddrs(ctx, coded, settings.Thread, log.ID, addrs)
	if err != nil {
		return
	}

	return log.ID, coded, nil
}

func (t *threads) Put(ctx context.Context, rec thread.Record, opts ...tserv.PutOption) error {
	// Get or create a log for the new rec
	settings := tserv.PutOptions(opts...)
	log, err := t.getOrCreateLog(settings.Thread, settings.Log)
	if err != nil {
		return err
	}

	// Save the rec locally
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

	t.SetHead(settings.Thread, log.ID, rec.Cid())
	return nil
}

// if own log, return local values
// if not, call addresses
func (t *threads) Pull(ctx context.Context, id thread.ID, lid peer.ID, opts ...tserv.PullOption) ([]thread.Record, error) {
	log := t.LogInfo(id, lid)

	settings := tserv.PullOptions(opts...)
	if !settings.Offset.Defined() {
		if len(log.Heads) == 0 {
			return nil, nil
		}
		settings.Offset = log.Heads[0]
	}

	followKey, err := crypto.ParseDecryptionKey(log.FollowKey)
	if err != nil {
		return nil, err
	}

	var recs []thread.Record
	for i := 0; i < settings.Limit; i++ {
		node, err := cbor.GetRecord(ctx, t.dagService, settings.Offset, followKey)
		if err != nil {
			return nil, err
		}
		recs = append(recs, node)

		settings.Offset = node.PrevID()
		if !settings.Offset.Defined() {
			break
		}
	}
	return recs, nil
}

func (t *threads) Logs(id thread.ID) []thread.LogInfo {
	logs := make([]thread.LogInfo, 0)
	for _, l := range t.ThreadInfo(id).Logs {
		log := t.LogInfo(id, l)
		logs = append(logs, log)
	}
	return logs
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

func (t *threads) getOrCreateOwnLog(id thread.ID) (info thread.LogInfo, err error) {
	for _, lid := range t.LogsWithKeys(id) {
		if t.PrivKey(id, lid) != nil {
			info = t.LogInfo(id, lid)
			return
		}
	}
	info, err = t.createLog()
	if err != nil {
		return
	}
	err = t.AddLog(id, info)
	return
}

func (t *threads) createNode(ctx context.Context, body format.Node, log thread.LogInfo, settings *tserv.AddSettings) (thread.Record, error) {
	if settings.Key == nil {
		var err error
		settings.Key, err = crypto.ParseEncryptionKey(log.ReadKey)
		if err != nil {
			return nil, err
		}
	}
	event, err := cbor.NewEvent(ctx, t.dagService, body, settings)
	if err != nil {
		return nil, err
	}

	prev := cid.Undef
	if len(log.Heads) != 0 {
		prev = log.Heads[0]
	}
	followKey, err := crypto.ParseEncryptionKey(log.FollowKey)
	if err != nil {
		return nil, err
	}
	rec, err := cbor.NewRecord(ctx, t.dagService, event, prev, log.PrivKey, followKey)
	if err != nil {
		return nil, err
	}

	t.SetHead(settings.Thread, log.ID, rec.Cid())

	return rec, nil
}
