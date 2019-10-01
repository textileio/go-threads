package threads

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
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

func (ts *threadservice) Add(ctx context.Context, body format.Node, opts ...tserv.AddOption) (l peer.ID, c cid.Cid, err error) {
	// Get or create a log for the new node
	settings := tserv.AddOptions(opts...)
	log, err := ts.getOrCreateOwnLog(settings.Thread)
	if err != nil {
		return
	}

	// Write a node locally
	coded, err := ts.createNode(ctx, body, log, settings)
	if err != nil {
		return
	}

	// Send log to known writers
	for _, l := range ts.ThreadInfo(settings.Thread).Logs {
		if l.String() == log.ID.String() {
			continue
		}
		for _, a := range ts.Addrs(settings.Thread, l) {
			err = ts.send(ctx, coded, settings.Thread, log.ID, a)
			if err != nil {
				return
			}
		}
	}

	// Send to options addresses
	for _, a := range settings.Addrs {
		err = ts.send(ctx, coded, settings.Thread, log.ID, a)
		if err != nil {
			return
		}
	}

	// Publish to network
	// @todo

	return log.ID, coded.Cid(), nil
}

func (ts *threadservice) Put(ctx context.Context, node thread.Node, opts ...tserv.PutOption) error {
	// Get or create a log for the new node
	settings := tserv.PutOptions(opts...)
	log, err := ts.getOrCreateLog(settings.Thread, settings.Log)
	if err != nil {
		return err
	}

	// Save the node locally
	err = ts.dagService.AddMany(ctx, []format.Node{node, node.Block()})
	if err != nil {
		return err
	}

	ts.SetHead(settings.Thread, log.ID, node.Cid())
	return nil
}

// if own log, return local values
// if not, call addresses
func (ts *threadservice) Pull(ctx context.Context, t thread.ID, l peer.ID, opts ...tserv.PullOption) ([]thread.Node, error) {
	log := ts.LogInfo(t, l)

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

	var nodes []thread.Node
	for i := 0; i < settings.Limit; i++ {
		node, err := cbor.GetNode(ctx, ts.dagService, settings.Offset, followKey)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, node)

		settings.Offset = node.Prev()
		if !settings.Offset.Defined() {
			break
		}
	}
	return nodes, nil
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

func (ts *threadservice) Delete(ctx context.Context, t thread.ID) error {
	panic("implement me")
}

func (ts *threadservice) createLog() (info thread.LogInfo, err error) {
	sk, pk, err := ic.GenerateEd25519Key(rand.Reader)
	if err != nil {
		return
	}
	id, err := peer.IDFromPublicKey(pk)
	if err != nil {
		return
	}
	rk, err := symmetric.CreateKey()
	if err != nil {
		return
	}
	fk, err := symmetric.CreateKey()
	if err != nil {
		return
	}
	addr, err := ma.NewMultiaddr(fmt.Sprintf("/p2p/%s", ts.host.ID().String()))
	if err != nil {
		return
	}
	return thread.LogInfo{
		ID:        id,
		PubKey:    pk,
		PrivKey:   sk,
		ReadKey:   rk.Bytes(),
		FollowKey: fk.Bytes(),
		Addrs:     []ma.Multiaddr{addr},
	}, nil
}

func (ts *threadservice) getOrCreateLog(t thread.ID, l peer.ID) (info thread.LogInfo, err error) {
	info = ts.LogInfo(t, l)
	if info.PubKey != nil {
		return
	}
	info, err = ts.createLog()
	if err != nil {
		return
	}
	err = ts.AddLog(t, info)
	return
}

func (ts *threadservice) getOrCreateOwnLog(t thread.ID) (info thread.LogInfo, err error) {
	for _, id := range ts.LogsWithKeys(t) {
		if ts.PrivKey(t, id) != nil {
			info = ts.LogInfo(t, id)
			return
		}
	}
	info, err = ts.createLog()
	if err != nil {
		return
	}
	err = ts.AddLog(t, info)
	return
}

func (ts *threadservice) createNode(ctx context.Context, body format.Node, log thread.LogInfo, settings *tserv.AddSettings) (format.Node, error) {
	if settings.Key == nil {
		var err error
		settings.Key, err = crypto.ParseEncryptionKey(log.ReadKey)
		if err != nil {
			return nil, err
		}
	}
	event, err := cbor.NewEvent(ctx, ts.dagService, body, settings)
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
	node, err := cbor.NewNode(ctx, ts.dagService, event, prev, log.PrivKey, followKey)
	if err != nil {
		return nil, err
	}

	ts.SetHead(settings.Thread, log.ID, node.Cid())

	return node, nil
}

func (ts *threadservice) send(ctx context.Context, node format.Node, t thread.ID, l peer.ID, addr ma.Multiaddr) error {
	p, err := addr.ValueForProtocol(ma.P_P2P)
	if err != nil {
		return err
	}
	uri := fmt.Sprintf("%s://%s/threads/v0/%s/%s", IPEL, p, t.String(), l.String())
	reader := bytes.NewReader(node.RawData())

	req, err := http.NewRequest(http.MethodPost, uri, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/octet-stream")

	sk := ts.Host().Peerstore().PrivKey(ts.Host().ID())
	if sk == nil {
		return fmt.Errorf("could not find key for host")
	}
	pk, err := sk.GetPublic().Bytes()
	if err != nil {
		return err
	}
	req.Header.Set("Identity", base64.StdEncoding.EncodeToString(pk))
	sig, err := sk.Sign(node.RawData())
	if err != nil {
		return err
	}
	req.Header.Set("Signature", base64.StdEncoding.EncodeToString(sig))

	fk := ts.FollowKey(t, l)
	if fk == nil {
		return fmt.Errorf("could not find follow key")
	}
	req.Header.Set("Authorization", base64.StdEncoding.EncodeToString(fk))

	res, err := ts.client.Do(req)
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
