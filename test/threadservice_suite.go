package test

import (
	"context"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-ipfs/dagutils"
	cbornode "github.com/ipfs/go-ipld-cbor"
	"github.com/libp2p/go-libp2p"
	ic "github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peerstore"
	ma "github.com/multiformats/go-multiaddr"
	mh "github.com/multiformats/go-multihash"
	"github.com/textileio/go-textile-core/crypto/asymmetric"
	"github.com/textileio/go-textile-core/thread"
	tserv "github.com/textileio/go-textile-core/threadservice"
	threads "github.com/textileio/go-textile-threads"
	tstore "github.com/textileio/go-textile-threads/tstoremem"
)

var threadserviceSuite = map[string]func(tserv.Threadservice, tserv.Threadservice) func(*testing.T){
	"PutPull":   testPutPull,
	"PutInvite": testPutInvite,
	"Close":     testClose,
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

func testPutPull(ts1, ts2 tserv.Threadservice) func(t *testing.T) {
	return func(t *testing.T) {
		tid := thread.NewIDV1(thread.Raw, 32)

		body, err := cbornode.WrapObject(map[string]interface{}{
			"foo": "bar",
			"baz": []byte("howdy"),
		}, mh.SHA2_256, -1)
		if err != nil {
			t.Fatal(err)
		}

		lid1, nid1, err := ts1.Put(context.Background(), body, tserv.PutOpt.Thread(tid))
		if err != nil {
			t.Fatal(err)
		}
		if !nid1.Defined() {
			t.Errorf("expected node id to be defined")
		}

		lid2, nid2, err := ts1.Put(context.Background(), body, tserv.PutOpt.Thread(tid))
		if err != nil {
			t.Fatal(err)
		}
		if !nid2.Defined() {
			t.Errorf("expected node id to be defined")
		}

		if lid2.String() != lid2.String() {
			t.Errorf("expected log IDs to match, got %s and %s", lid1.String(), lid2.String())
		}

		events, err := ts1.Pull(context.Background(), cid.Undef, 2, ts1.LogInfo(tid, lid1))
		if err != nil {
			t.Fatal(err)
		}
		if len(events) != 2 {
			t.Errorf("expected 2 event got %d", len(events))
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

func testPutInvite(ts1, ts2 tserv.Threadservice) func(t *testing.T) {
	return func(t *testing.T) {
		tid := thread.NewIDV1(thread.Raw, 32)
		opts := tserv.PutOpt.Thread(tid)

		invite, err := ts1.NewInvite(tid, true)
		if err != nil {
			t.Fatal(err)
		}

		pk := ts2.Host().Peerstore().PubKey(ts2.Host().ID())
		if pk == nil {
			t.Fatal("public key not found")
		}
		pkb, err := ic.MarshalPublicKey(pk)
		if err != nil {
			t.Fatal(err)
		}
		ek, err := asymmetric.NewEncryptionKey(pkb)
		if err != nil {
			t.Fatal(err)
		}
		opts = opts.Key(ek)

		a, err := ma.NewMultiaddr("/p2p/" + ts2.Host().ID().String())
		if err != nil {
			t.Fatal(err)
		}
		opts = opts.Addrs([]ma.Multiaddr{a})

		_, _, err = ts1.Put(context.Background(), invite, opts)
		if err != nil {
			t.Fatal(err)
		}

	}
}

//func testAPI(ts1, ts2 tserv.Threadservice) func(*testing.T) {
//	return func(t *testing.T) {
//
//		body, err := cbornode.WrapObject(map[string]interface{}{
//			"foo": "bar",
//			"baz": []byte("howdy"),
//		}, mh.SHA2_256, -1)
//		if err != nil {
//			t.Fatal(err)
//		}
//
//		tid := thread.NewIDV1(thread.Raw, 32)
//		lid1, nid1, err := ts1.Put(context.Background(), body, tserv.PutOpt.Thread(tid))
//		if err != nil {
//			t.Fatal(err)
//		}
//
//		// invite other peer
//		// get head on either peer
//
//		t.Run("get head", func(t *testing.T) {
//			addr := fmt.Sprintf("ipel://%s/%s", ts2.Host().ID().Pretty(), "pull/foo")
//
//			// ts.Pull()
//
//			res, err := ts1.Client().Get(addr)
//			if err != nil {
//				t.Fatal(err)
//			}
//			defer res.Body.Close()
//			text, err := ioutil.ReadAll(res.Body)
//			if err != nil {
//				t.Fatal(err)
//			}
//			if string(text) != "Hi!" {
//				//t.Errorf("expected Hi! but got %s", string(text))
//			}
//		})
//
//	}
//}

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
