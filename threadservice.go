package threads

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"

	"github.com/ipfs/go-cid"
	format "github.com/ipfs/go-ipld-format"
	ic "github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	gostream "github.com/libp2p/go-libp2p-gostream"
	p2phttp "github.com/libp2p/go-libp2p-http"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/textileio/go-textile-core/crypto"
	"github.com/textileio/go-textile-core/crypto/symmetric"
	"github.com/textileio/go-textile-core/thread"
	tserv "github.com/textileio/go-textile-core/threadservice"
	tstore "github.com/textileio/go-textile-core/threadstore"
	"github.com/textileio/go-textile-threads/cbor"
	tserver "github.com/textileio/go-textile-threads/threadserver"
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

type threadservice struct {
	host       host.Host
	listener   net.Listener
	server     *tserver.Threadserver
	client     *http.Client
	dagService format.DAGService
	tstore.Threadstore
}

func NewThreadservice(h host.Host, ds format.DAGService, ts tstore.Threadstore) (tserv.Threadservice, error) {
	listener, err := gostream.Listen(h, IPELProtocol)
	if err != nil {
		return nil, err
	}

	tr := &http.Transport{}
	tr.RegisterProtocol(IPEL, p2phttp.NewTransport(h, p2phttp.ProtocolOption(IPELProtocol)))

	service := &threadservice{
		host:        h,
		listener:    listener,
		client:      &http.Client{Transport: tr},
		dagService:  ds,
		Threadstore: ts,
	}

	service.server = tserver.NewThreadserver(func() tserv.Threadservice {
		return service
	})
	service.server.Open(listener)

	return service, nil
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

	weakClose("server", ts.server)
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

func (ts *threadservice) Put(ctx context.Context, body format.Node, opts ...tserv.PutOption) (peer.ID, cid.Cid, error) {
	// Get or create a log for the new event
	settings := tserv.PutOptions(opts...)
	log := ts.getOwnLog(settings.Thread)
	if log == nil {
		var err error
		log, err = ts.generateLog()
		if err != nil {
			return "", cid.Undef, err
		}
		err = ts.AddLog(settings.Thread, *log)
		if err != nil {
			return "", cid.Undef, err
		}
	}

	// Write the event locally
	coded, err := ts.writeEvent(ctx, body, log, settings)
	if err != nil {
		return "", cid.Undef, err
	}

	// Send log to known writers
	for _, l := range ts.ThreadInfo(settings.Thread).Logs {
		if l.String() == log.ID.String() {
			continue
		}
		for _, a := range ts.Addrs(settings.Thread, l) {
			err = ts.send(ctx, coded, settings.Thread, log.ID, a)
			if err != nil {
				return "", cid.Undef, err
			}
		}
	}

	// Send to options addresses
	for _, a := range settings.Addrs {
		err = ts.send(ctx, coded, settings.Thread, log.ID, a)
		if err != nil {
			return "", cid.Undef, err
		}
	}

	// Publish to network
	// @todo

	return log.ID, coded.Cid(), nil
}

// if own log, return local values
// if not, call addresses
func (ts *threadservice) Pull(ctx context.Context, offset cid.Cid, limit int, log thread.LogInfo) ([]thread.Event, error) {
	if !offset.Defined() {
		if len(log.Heads) == 0 {
			return nil, nil
		}
		offset = log.Heads[0]
	}

	followKey, err := crypto.ParseDecryptionKey(log.FollowKey)
	if err != nil {
		return nil, err
	}
	readKey, err := crypto.ParseDecryptionKey(log.ReadKey)
	if err != nil {
		return nil, err
	}

	var events []thread.Event
	for i := 0; i < limit; i++ {
		node, err := cbor.DecodeNode(ctx, ts.dagService, offset, followKey)
		if err != nil {
			return nil, err
		}

		event, err := cbor.DecodeEvent(ctx, ts.dagService, node.Block().Cid(), readKey)
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

func (ts *threadservice) NewInvite(t thread.ID, reader bool) (format.Node, error) {
	invite := cbor.Invite{
		Logs: make(map[string]thread.LogInfo),
	}
	for _, l := range ts.ThreadInfo(t).Logs {
		log := ts.LogInfo(t, l)
		log.PrivKey = nil
		if !reader {
			log.ReadKey = nil
		}
		invite.Logs[l.String()] = log
	}
	return cbor.NewInvite(invite)
}

func (ts *threadservice) send(ctx context.Context, node format.Node, t thread.ID, l peer.ID, addr ma.Multiaddr) error {
	p, err := addr.ValueForProtocol(ma.P_P2P)
	if err != nil {
		return err
	}
	uri := fmt.Sprintf("%s://%s/threads/v0/%s/%s", IPEL, p, t.String(), l.String())
	res, err := ts.client.Post(uri, "application/octet-stream", bytes.NewReader(node.RawData()))
	if err != nil {
		return err
	}
	switch res.StatusCode {
	case http.StatusCreated:
		defer res.Body.Close()
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return err
		}
		fmt.Println("created!")
		fmt.Println(string(body))
	case http.StatusOK:
		fmt.Println("ok!")
	}
	return nil
}

func (ts *threadservice) Delete(ctx context.Context, t thread.ID) error {
	panic("implement me")
}

func (ts *threadservice) generateLog() (*thread.LogInfo, error) {
	sk, pk, err := ic.GenerateEd25519Key(rand.Reader)
	if err != nil {
		return nil, err
	}
	id, err := peer.IDFromPublicKey(pk)
	if err != nil {
		return nil, err
	}
	rk, err := symmetric.CreateKey()
	if err != nil {
		return nil, err
	}
	fk, err := symmetric.CreateKey()
	if err != nil {
		return nil, err
	}
	addr, err := ma.NewMultiaddr(fmt.Sprintf("/p2p/%s", ts.host.ID().String()))
	if err != nil {
		return nil, err
	}
	return &thread.LogInfo{
		ID:        id,
		PubKey:    pk,
		PrivKey:   sk,
		ReadKey:   rk.Bytes(),
		FollowKey: fk.Bytes(),
		Addrs:     []ma.Multiaddr{addr},
	}, nil
}

func (ts *threadservice) getOwnLog(t thread.ID) *thread.LogInfo {
	for _, id := range ts.LogsWithKeys(t) {
		sk := ts.PrivKey(t, id)
		if sk != nil {
			log := ts.LogInfo(t, id)
			return &log
		}
	}
	return nil
}

func (ts *threadservice) writeEvent(ctx context.Context, body format.Node, log *thread.LogInfo, settings *tserv.PutSettings) (format.Node, error) {
	if settings.Key == nil {
		var err error
		settings.Key, err = crypto.ParseEncryptionKey(log.ReadKey)
		if err != nil {
			return nil, err
		}
	}

	event, err := cbor.NewEvent(body, settings)
	if err != nil {
		return nil, err
	}
	ecoded, err := cbor.EncodeEvent(event, settings.Key)
	if err != nil {
		return nil, err
	}
	prev := cid.Undef
	if len(log.Heads) != 0 {
		prev = log.Heads[0]
	}
	node, err := cbor.NewNode(ecoded, prev, log.PrivKey)
	if err != nil {
		return nil, err
	}
	followKey, err := crypto.ParseEncryptionKey(log.FollowKey)
	if err != nil {
		return nil, err
	}
	coded, err := cbor.EncodeNode(node, followKey)
	if err != nil {
		return nil, err
	}

	err = ts.dagService.AddMany(ctx, []format.Node{event.Header(), event.Body(), ecoded, coded})
	if err != nil {
		return nil, err
	}

	ts.SetHead(settings.Thread, log.ID, coded.Cid())

	return coded, nil
}
