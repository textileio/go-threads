package threads

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/ipfs/go-cid"
	format "github.com/ipfs/go-ipld-format"
	ic "github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"
	"github.com/libp2p/go-libp2p-core/protocol"
	gostream "github.com/libp2p/go-libp2p-gostream"
	p2phttp "github.com/libp2p/go-libp2p-http"
	"github.com/textileio/go-textile-core/crypto"
	"github.com/textileio/go-textile-core/thread"
	tserv "github.com/textileio/go-textile-core/threadservice"
	tstore "github.com/textileio/go-textile-core/threadstore"
	"github.com/textileio/go-textile-threads/cbor"
)

const (
	LogProtocol protocol.ID = "/log/1.0.0"
)

type threadservice struct {
	host       host.Host
	listener   net.Listener
	client     *http.Client
	dagService format.DAGService
	tstore.Threadstore
}

func NewThreadservice(h host.Host, ds format.DAGService, ts tstore.Threadstore) (tserv.Threadservice, error) {
	listener, err := gostream.Listen(h, LogProtocol)
	if err != nil {
		return nil, err
	}

	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
			//defer r.Body.Close()
			//body, err := ioutil.ReadAll(r.Body)
			//if err != nil {
			//	http.Error(w, err.Error(), 500)
			//	return
			//}
			_, _ = w.Write([]byte("Hi!"))
		})
		server := &http.Server{Handler: mux}
		_ = server.Serve(listener)
	}()

	tr := &http.Transport{}
	tr.RegisterProtocol("libp2p", p2phttp.NewTransport(h, p2phttp.ProtocolOption(LogProtocol)))

	return &threadservice{
		listener:    listener,
		host:        h,
		client:      &http.Client{Transport: tr},
		dagService:  ds,
		Threadstore: ts,
	}, nil
}

func (ts *threadservice) Close() (err error) {
	var errs []error
	weakClose := func(name string, c interface{}) {
		if cl, ok := c.(io.Closer); ok {
			if err = cl.Close(); err != nil {
				errs = append(errs, fmt.Errorf("%s error: %s", name, err))
			}
		}
	}

	//weakClose("host", ts.host)
	weakClose("listener", ts.listener)
	weakClose("dagservice", ts.dagService)
	weakClose("threadstore", ts.Threadstore)

	if len(errs) > 0 {
		return fmt.Errorf("failed while closing threadservice; err(s): %q", errs)
	}
	return nil
}

func (ts *threadservice) Host() host.Host {
	return ts.host
}

func (ts *threadservice) DAGService() format.DAGService {
	return ts.dagService
}

func (ts *threadservice) Listener() net.Listener {
	return ts.listener
}

func (ts *threadservice) Client() *http.Client {
	return ts.client
}

func (ts *threadservice) Put(ctx context.Context, body format.Node, opts ...tserv.PutOption) (peer.ID, cid.Cid, error) {
	settings := tserv.PutOptions(opts...)

	id, sk := ts.getOwnLog(settings.Thread)
	if sk == nil {
		var err error
		id, sk, err = ts.addOwnLog(settings.Thread)
		if err != nil {
			return "", cid.Undef, err
		}
	}

	prev := cid.Undef
	heads := ts.Heads(settings.Thread, id)
	if len(heads) != 0 {
		prev = heads[0]
	}

	event, err := cbor.NewEvent(body, time.Now())
	if err != nil {
		return "", cid.Undef, err
	}
	ecoded, err := cbor.EncodeEvent(event, ts.ReadKey(settings.Thread, id))
	if err != nil {
		return "", cid.Undef, err
	}
	node, err := cbor.NewNode(ecoded, prev, sk)
	if err != nil {
		return "", cid.Undef, err
	}
	coded, err := cbor.EncodeNode(node, ts.FollowKey(settings.Thread, id))
	if err != nil {
		return "", cid.Undef, err
	}

	err = ts.dagService.AddMany(ctx, []format.Node{event.Header(), event.Body(), ecoded, coded})
	if err != nil {
		return "", cid.Undef, err
	}

	ts.SetHead(settings.Thread, id, coded.Cid())

	return id, coded.Cid(), nil
}

func (ts *threadservice) Pull(ctx context.Context, offset cid.Cid, limit int, t thread.ID, id peer.ID) ([]thread.Event, error) {
	fk := ts.FollowKey(t, id)
	if fk == nil {
		return nil, nil
	}

	if !offset.Defined() {
		heads := ts.Heads(t, id)
		if len(heads) == 0 {
			return nil, nil
		}
		offset = heads[0]
	}

	var events []thread.Event
	for i := 0; i < limit; i++ {
		node, err := cbor.DecodeNode(ctx, ts.dagService, offset, fk)
		if err != nil {
			return nil, err
		}
		rk := ts.ReadKey(t, id)
		if rk == nil {
			return events, nil
		}
		event, err := cbor.DecodeEvent(ctx, ts.dagService, node.Block().Cid(), rk)
		if err != nil {
			return nil, err
		}
		events = append(events, event)

		offset = node.Prev()
		if !offset.Defined() {
			break
		}
	}

	return events, nil
}

func (ts *threadservice) Invite(ctx context.Context, actor peer.ID, t thread.ID) error {
	panic("implement me")
}

func (ts *threadservice) Leave(ctx context.Context, t thread.ID) error {
	panic("implement me")
}

func (ts *threadservice) Delete(ctx context.Context, t thread.ID) error {
	panic("implement me")
}

func (ts *threadservice) getOwnLog(t thread.ID) (peer.ID, ic.PrivKey) {
	for _, log := range ts.LogsWithKeys(t) {
		sk := ts.PrivKey(t, log)
		if sk != nil {
			return log, sk
		}
	}
	return "", nil
}

func (ts *threadservice) addOwnLog(t thread.ID) (peer.ID, ic.PrivKey, error) {
	sk, pk, err := ic.GenerateEd25519Key(rand.Reader)
	if err != nil {
		return "", nil, err
	}
	id, err := peer.IDFromPublicKey(pk)
	if err != nil {
		return "", nil, err
	}

	err = ts.AddPrivKey(t, id, sk)
	if err != nil {
		return "", nil, err
	}
	err = ts.AddPubKey(t, id, pk)
	if err != nil {
		return "", nil, err
	}

	read, err := crypto.GenerateAESKey()
	if err != nil {
		return "", nil, err
	}
	err = ts.AddReadKey(t, id, read)
	if err != nil {
		return "", nil, err
	}

	follow, err := crypto.GenerateAESKey()
	if err != nil {
		return "", nil, err
	}
	err = ts.AddFollowKey(t, id, follow)
	if err != nil {
		return "", nil, err
	}

	ts.AddAddrs(t, id, ts.host.Addrs(), peerstore.PermanentAddrTTL)

	return id, sk, nil
}
