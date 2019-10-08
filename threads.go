package threads

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"

	"github.com/ipfs/go-cid"
	format "github.com/ipfs/go-ipld-format"
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
	rpc        *grpc.Server
	pubsub     *pubsub.PubSub
	dagService format.DAGService
	ctx        context.Context
	cancel     context.CancelFunc
	tstore.Threadstore
}

func NewThreadservice(ctx context.Context, h host.Host, ds format.DAGService, ts tstore.Threadstore) (tserv.Threadservice, error) {
	ctx, cancel := context.WithCancel(ctx)
	t := &threads{
		host:        h,
		rpc:         grpc.NewServer(),
		dagService:  ds,
		ctx:         ctx,
		cancel:      cancel,
		Threadstore: ts,
	}

	listener, err := gostream.Listen(h, IPELProtocol)
	if err != nil {
		return nil, err
	}
	go t.rpc.Serve(listener)
	pb.RegisterThreadsServer(t.rpc, &service{PeerID: t.host.ID()})

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

	t.cancel()

	//weakClose("server", t.server)
	//weakClose("host", t.host)
	weakClose("dagservice", t.dagService)
	weakClose("threadstore", t.Threadstore)

	if len(errs) > 0 {
		return fmt.Errorf("failed while closing threads; err(s): %q", errs)
	}
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

	// Send log to known writers
	for _, i := range t.ThreadInfo(settings.Thread).Logs {
		if i.String() == log.ID.String() {
			continue
		}
		for _, a := range t.Addrs(settings.Thread, i) {
			err = t.send(ctx, coded, settings.Thread, log.ID, a)
			if err != nil {
				return
			}
		}
	}

	// Send to additional addresses
	for _, a := range settings.Addrs {
		err = t.send(ctx, coded, settings.Thread, log.ID, a)
		if err != nil {
			return
		}
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

func (t *threads) send(ctx context.Context, rec thread.Record, id thread.ID, lid peer.ID, addr ma.Multiaddr) error {
	p, err := addr.ValueForProtocol(ma.P_P2P)
	if err != nil {
		return err
	}
	uri := fmt.Sprintf("%s://%s/threads/v0/%s/%s", IPEL, p, id.String(), lid.String())
	payload, err := cbor.Marshal(ctx, t.dagService, rec)
	if err != nil {
		return err
	}

	pid, err := peer.IDB58Decode(p)
	if err != nil {
		return err
	}
	conn, err := t.dial(ctx, pid, grpc.WithInsecure(), grpc.WithBlock())
	client := pb.NewThreadsClient(conn)

	reply, err := client.Push(ctx, &pb.EchoRequest{Message: "hey"})
	if err != nil {
		return err
	}
	fmt.Println(reply.String())

	req, err := http.NewRequest(http.MethodPost, uri, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/cbor")

	sk := t.Host().Peerstore().PrivKey(t.Host().ID())
	if sk == nil {
		return fmt.Errorf("could not find key for host")
	}
	pk, err := sk.GetPublic().Bytes()
	if err != nil {
		return err
	}
	req.Header.Set("X-Identity", base64.StdEncoding.EncodeToString(pk))
	sig, err := sk.Sign(payload)
	if err != nil {
		return err
	}
	req.Header.Set("X-Signature", base64.StdEncoding.EncodeToString(sig))

	fk := t.FollowKey(t, lid)
	if fk == nil {
		return fmt.Errorf("could not find follow key")
	}
	req.Header.Set("X-FollowKey", base64.StdEncoding.EncodeToString(fk))

	res, err := t.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}
	switch res.StatusCode {
	case http.StatusCreated:
		fk, err := base64.StdEncoding.DecodeString(res.Header.Get("X-FollowKey"))
		if err != nil {
			return err
		}
		followKey, err := crypto.ParseDecryptionKey(fk)
		if err != nil {
			return err
		}
		rnode, err := cbor.Unmarshal(body, followKey)
		if err != nil {
			return err
		}
		event, err := cbor.GetEventFromNode(ctx, t.dagService, rnode)
		if err != nil {
			return err
		}

		rk, err := base64.StdEncoding.DecodeString(res.Header.Get("X-ReadKey"))
		if err != nil {
			return err
		}
		readKey, err := crypto.ParseDecryptionKey(rk)
		if err != nil {
			return err
		}
		body, err := event.GetBody(ctx, t.dagService, readKey)
		if err != nil {
			return err
		}
		logs, _, err := cbor.DecodeInvite(body)
		if err != nil {
			return err
		}
		for _, log := range logs {
			err = t.AddLog(t, log)
			if err != nil {
				return err
			}
		}
	case http.StatusNoContent:
	default:
		var msg map[string]string
		err = json.Unmarshal(body, &msg)
		if err != nil {
			return err
		}
		return fmt.Errorf(msg["error"])
	}
	return nil
}

// getDialOption returns the WithDialer option to dial via libp2p.
func (t *threads) getDialOption() grpc.DialOption {
	return grpc.WithContextDialer(func(ctx context.Context, peerIdStr string) (net.Conn, error) {
		id, err := peer.IDB58Decode(peerIdStr)
		if err != nil {
			return nil, fmt.Errorf("grpc tried to dial non peer-id: %s", err)
		}
		c, err := gostream.Dial(ctx, t.host, id, IPELProtocol)
		return c, err
	})
}

// dial attempts to open a GRPC connection over libp2p to a peer.
func (t *threads) dial(
	ctx context.Context,
	peerID peer.ID,
	dialOpts ...grpc.DialOption,
) (*grpc.ClientConn, error) {
	dialOpsPrepended := append([]grpc.DialOption{t.getDialOption()}, dialOpts...)
	return grpc.DialContext(ctx, peerID.Pretty(), dialOpsPrepended...)
}

// service implements the Threads RPC service.
type service struct {
	PeerID peer.ID
}

// Echo asks a node to respond with a message.
func (s *service) Push(ctx context.Context, req *pb.RecordRequest) (*pb.RecordReply, error) {
	return &pb.RecordReply{
		Ok: true,
	}, nil
}
