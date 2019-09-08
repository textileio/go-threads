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
	host     host.Host
	listener net.Listener
	client   *http.Client
	format.DAGService
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
		DAGService:  ds,
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
	weakClose("dagservice", ts.DAGService)
	weakClose("threadstore", ts.Threadstore)

	if len(errs) > 0 {
		return fmt.Errorf("failed while closing threadservice; err(s): %q", errs)
	}
	return nil
}

func (ts *threadservice) Host() host.Host {
	return ts.host
}

func (ts *threadservice) Listener() net.Listener {
	return ts.listener
}

func (ts *threadservice) Client() *http.Client {
	return ts.client
}

/*
	1. do we have a thread? if not, create one
	2. check ACL to see if we can even write to this thread
	3. if no log for us, create one
	4. create event: encrypt body with a new key, save key in header, encrypt the whole thing with read key
	5. get latest head for this log
	6. append event to last head
	7. save to dag service
*/
func (ts *threadservice) Put(ctx context.Context, body format.Node, threads ...thread.ID) ([]cid.Cid, error) {
	var cids []cid.Cid
	for _, t := range threads {
		c, err := ts.put(ctx, body, t)
		if err != nil {
			return cids, err
		}
		cids = append(cids, c)
	}
	return cids, nil
}

func (ts *threadservice) put(ctx context.Context, body format.Node, t thread.ID) (cid.Cid, error) {
	var id peer.ID
	var sk ic.PrivKey
	for _, p := range ts.LogsWithKeys(t) {
		sk = ts.LogPrivKey(t, p)
		if sk != nil {
			id = p
			break
		}
	}
	if sk == nil {
		var err error
		id, err = ts.newLog(t)
		if err != nil {
			return cid.Undef, err
		}
		sk = ts.LogPrivKey(t, id)
	}

	prev := cid.Undef
	heads := ts.LogHeads(t, id)
	if len(heads) != 0 {
		prev = heads[0]
	}

	event, err := cbor.NewEvent(body, time.Now())
	if err != nil {
		return cid.Undef, err
	}
	ecoded, err := cbor.EncodeEvent(event, ts.LogReadKey(t, id))
	if err != nil {
		return cid.Undef, err
	}
	node, err := cbor.NewNode(ecoded, prev, sk)
	if err != nil {
		return cid.Undef, err
	}
	coded, err := cbor.EncodeNode(node, ts.LogFollowKey(t, id))
	if err != nil {
		return cid.Undef, err
	}

	err = ts.AddMany(ctx, []format.Node{event.Header(), event.Body(), ecoded, coded})
	if err != nil {
		return cid.Undef, err
	}

	ts.SetLogHead(t, id, coded.Cid())

	return coded.Cid(), nil
}

func (ts *threadservice) Pull(ctx context.Context, offset string, size int, t thread.ID) ([]thread.Event, error) {
	var events []thread.Event
	for _, id := range ts.ThreadInfo(t).Logs {
		heads := ts.LogHeads(t, id)
		if len(heads) == 0 {
			continue
		}
		fk := ts.LogFollowKey(t, id)
		if fk == nil {
			continue
		}
		node, err := cbor.DecodeNode(ctx, ts.DAGService, heads[0], fk)
		if err != nil {
			return nil, err
		}
		rk := ts.LogReadKey(t, id)
		if rk == nil {
			continue
		}
		event, err := cbor.DecodeEvent(ctx, ts.DAGService, node.Block().Cid(), rk)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
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

func (ts *threadservice) newLog(t thread.ID) (peer.ID, error) {
	sk, pk, err := ic.GenerateEd25519Key(rand.Reader)
	if err != nil {
		return "", err
	}
	id, err := peer.IDFromPublicKey(pk)
	if err != nil {
		return "", err
	}

	err = ts.AddLogPrivKey(t, id, sk)
	if err != nil {
		return "", err
	}
	err = ts.AddLogPubKey(t, id, pk)
	if err != nil {
		return "", err
	}

	read, err := crypto.GenerateAESKey()
	if err != nil {
		return "", err
	}
	err = ts.AddLogReadKey(t, id, read)
	if err != nil {
		return "", err
	}

	follow, err := crypto.GenerateAESKey()
	if err != nil {
		return "", err
	}
	err = ts.AddLogFollowKey(t, id, follow)
	if err != nil {
		return "", err
	}

	ts.AddLogAddrs(t, id, ts.host.Addrs(), peerstore.PermanentAddrTTL)

	return id, nil
}
