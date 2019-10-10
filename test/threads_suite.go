package test

import (
	"context"
	"testing"

	"github.com/ipfs/go-cid"

	bserv "github.com/ipfs/go-blockservice"
	ds "github.com/ipfs/go-datastore"
	syncds "github.com/ipfs/go-datastore/sync"
	bstore "github.com/ipfs/go-ipfs-blockstore"
	offline "github.com/ipfs/go-ipfs-exchange-offline"
	cbornode "github.com/ipfs/go-ipld-cbor"
	dag "github.com/ipfs/go-merkledag"
	"github.com/libp2p/go-libp2p"
	ic "github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peerstore"
	ma "github.com/multiformats/go-multiaddr"
	mh "github.com/multiformats/go-multihash"
	"github.com/textileio/go-textile-core/crypto"
	"github.com/textileio/go-textile-core/crypto/asymmetric"
	"github.com/textileio/go-textile-core/thread"
	tserv "github.com/textileio/go-textile-core/threadservice"
	threads "github.com/textileio/go-textile-threads"
	"github.com/textileio/go-textile-threads/cbor"
	tstore "github.com/textileio/go-textile-threads/tstoremem"
)

var threadsSuite = map[string]func(tserv.Threadservice, tserv.Threadservice) func(*testing.T){
	"AddPull":   testAddPull,
	"AddInvite": testAddInvite,
	"Close":     testClose,
}

func ThreadsTest(t *testing.T) {
	for name, test := range threadsSuite {
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
	sk, _, err := ic.GenerateKeyPair(ic.Ed25519, 0)
	if err != nil {
		t.Fatal(err)
	}
	host, err := libp2p.New(
		context.Background(),
		libp2p.ListenAddrs(listen),
		libp2p.Identity(sk),
	)
	if err != nil {
		t.Fatal(err)
	}

	bs := bstore.NewBlockstore(syncds.MutexWrap(ds.NewMapDatastore()))
	bsrv := bserv.New(bs, offline.Exchange(bs))
	ts, err := threads.NewThreads(
		context.Background(),
		host,
		bsrv,
		dag.NewDAGService(bsrv),
		tstore.NewThreadstore(),
		true)
	if err != nil {
		t.Fatal(err)
	}
	return ts
}

func testAddPull(ts1, _ tserv.Threadservice) func(t *testing.T) {
	return func(t *testing.T) {
		ctx := context.Background()
		tid := thread.NewIDV1(thread.Raw, 32)

		body, err := cbornode.WrapObject(map[string]interface{}{
			"foo": "bar",
			"baz": []byte("howdy"),
		}, mh.SHA2_256, -1)
		if err != nil {
			t.Fatal(err)
		}

		lid1, n1, err := ts1.Add(ctx, body, tserv.AddOpt.Thread(tid))
		if err != nil {
			t.Fatal(err)
		}
		if n1 == nil {
			t.Fatalf("expected node to not be nil")
		}

		lid2, n2, err := ts1.Add(ctx, body, tserv.AddOpt.Thread(tid))
		if err != nil {
			t.Fatal(err)
		}
		if n2 == nil {
			t.Fatalf("expected node to not be nil")
		}

		if lid2.String() != lid2.String() {
			t.Fatalf("expected log IDs to match, got %s and %s", lid1.String(), lid2.String())
		}

		// Pull from the log origin
		nodes, err := ts1.Pull(ctx, tid, lid1, cid.Undef)
		if err != nil {
			t.Fatal(err)
		}
		if len(nodes) != 2 {
			t.Fatalf("expected 2 nodes got %d", len(nodes))
		}

		event, err := cbor.GetEvent(ctx, ts1.DAGService(), nodes[0].BlockID())
		if err != nil {
			t.Fatal(err)
		}

		readKey, err := crypto.ParseDecryptionKey(ts1.ReadKey(tid, lid1))
		if err != nil {
			t.Fatal(err)
		}
		back, err := event.GetBody(ctx, ts1.DAGService(), readKey)
		if err != nil {
			t.Fatal(err)
		}
		if body.String() != back.String() {
			t.Fatalf("retrieved body does not equal input body")
		}
	}
}

func testAddInvite(ts1, ts2 tserv.Threadservice) func(t *testing.T) {
	return func(t *testing.T) {
		ctx := context.Background()
		tid := thread.NewIDV1(thread.Raw, 32)

		body, err := cbornode.WrapObject(map[string]interface{}{
			"msg": "yo!",
		}, mh.SHA2_256, -1)
		if err != nil {
			t.Fatal(err)
		}
		_, _, err = ts1.Add(ctx, body, tserv.AddOpt.Thread(tid))
		if err != nil {
			t.Fatal(err)
		}

		invite, err := cbor.NewInvite(ts1.Logs(tid), true)
		if err != nil {
			t.Fatal(err)
		}

		pk := ts1.Host().Peerstore().PubKey(ts2.Host().ID())
		if pk == nil {
			t.Fatal("public key not found")
		}
		ek, err := asymmetric.NewEncryptionKey(pk)
		if err != nil {
			t.Fatal(err)
		}

		a, err := ma.NewMultiaddr("/p2p/" + ts2.Host().ID().String())
		if err != nil {
			t.Fatal(err)
		}

		_, _, err = ts1.Add(
			context.Background(),
			invite,
			tserv.AddOpt.Thread(tid),
			tserv.AddOpt.Key(ek),
			tserv.AddOpt.Addrs([]ma.Multiaddr{a}))
		if err != nil {
			t.Fatal(err)
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
