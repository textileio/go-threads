package test

import (
	"context"
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/ipfs/go-ipfs/dagutils"
	cbornode "github.com/ipfs/go-ipld-cbor"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/peerstore"
	ma "github.com/multiformats/go-multiaddr"
	mh "github.com/multiformats/go-multihash"
	"github.com/textileio/go-textile-core/thread"
	tserv "github.com/textileio/go-textile-core/threadservice"
	threads "github.com/textileio/go-textile-threads"
	tstore "github.com/textileio/go-textile-threads/tstoremem"
)

var threadserviceSuite = map[string]func(tserv.Threadservice, tserv.Threadservice) func(*testing.T){
	"API":     testAPI,
	"PutPull": testPutPull,
	"Close":   testClose,
}

func ThreadserviceTest(t *testing.T) {
	for name, test := range threadserviceSuite {
		// Create two thread services.
		m1, _ := ma.NewMultiaddr("/ip4/127.0.0.1/tcp/10000")
		m2, _ := ma.NewMultiaddr("/ip4/127.0.0.1/tcp/10001")
		ts1 := newService(t, m1)
		ts2 := newService(t, m2)

		ts1.Host().Peerstore().AddAddrs(ts2.Host().ID(), ts2.Host().Addrs(), peerstore.PermanentAddrTTL)
		ts2.Host().Peerstore().AddAddrs(ts1.Host().ID(), ts1.Host().Addrs(), peerstore.PermanentAddrTTL)

		// Run the test.
		t.Run(name, test(ts1, ts2))
	}
}

func newService(t *testing.T, listen ma.Multiaddr) tserv.Threadservice {
	host, err := libp2p.New(
		context.Background(),
		libp2p.ListenAddrs(listen),
	)
	if err != nil {
		t.Fatal(err)
	}
	ts, err := threads.NewThreadservice(host, dagutils.NewMemoryDagService(), tstore.NewThreadstore())
	if err != nil {
		t.Fatal(err)
	}
	return ts
}

func testAPI(ts1, ts2 tserv.Threadservice) func(*testing.T) {
	return func(t *testing.T) {

		t.Run("get head", func(t *testing.T) {
			addr := fmt.Sprintf("libp2p://%s/hello", ts2.Host().ID().Pretty())
			res, err := ts1.Client().Get(addr)
			if err != nil {
				t.Fatal(err)
			}
			defer res.Body.Close()
			text, err := ioutil.ReadAll(res.Body)
			if err != nil {
				t.Fatal(err)
			}
			if string(text) != "Hi!" {
				t.Errorf("expected Hi! but got %s", text)
			}
		})

	}
}

func testPutPull(ts1, ts2 tserv.Threadservice) func(t *testing.T) {
	return func(t *testing.T) {
		body, err := cbornode.WrapObject(map[string]interface{}{
			"foo": "bar",
			"baz": []byte("howdy"),
		}, mh.SHA2_256, -1)
		if err != nil {
			t.Fatal(err)
		}

		tid := thread.NewIDV1(thread.Raw, 32)
		nodes, err := ts1.Put(context.Background(), body, tid)
		if err != nil {
			t.Fatal(err)
		}
		if len(nodes) != 1 {
			t.Errorf("expected 1 node got %d", len(nodes))
		}

		events, err := ts1.Pull(context.Background(), "", 0, tid)
		if err != nil {
			t.Fatal(err)
		}
		if len(events) != 1 {
			t.Errorf("expected 1 event got %d", len(events))
		}

		back, err := events[0].Decrypt()
		if err != nil {
			t.Fatal(err)
		}
		if body.String() != back.String() {
			t.Errorf("retrieved body does not equal input body")
		}
	}
}

func testClose(ts1, ts2 tserv.Threadservice) func(t *testing.T) {
	return func(t *testing.T) {
		err := ts1.Close()
		if err != nil {
			t.Fatal(err)
		}
		err = ts2.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
}
