package test

import (
	"context"
	"testing"

	"github.com/textileio/go-textile-core/crypto"
	"github.com/textileio/go-textile-threads/cbor"

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
	"AddPull":   testAddPull,
	"AddInvite": testAddInvite,
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

func testAddPull(ts1, ts2 tserv.Threadservice) func(t *testing.T) {
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

		nodes, err := ts1.Pull(ctx, tid, lid1, tserv.PullOpt.Limit(2))
		if err != nil {
			t.Fatal(err)
		}
		if len(nodes) != 2 {
			t.Fatalf("expected 2 nodes got %d", len(nodes))
		}

		event, err := cbor.GetEvent(ctx, ts1.DAGService(), nodes[0].Block().Cid())
		if err != nil {
			t.Fatal(err)
		}

		readKey, err := crypto.ParseDecryptionKey(ts1.ReadKey(tid, lid1))
		if err != nil {
			t.Fatal(err)
		}
		back, err := event.Body(ctx, ts1.DAGService(), readKey)
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
		tid := thread.NewIDV1(thread.Raw, 32)
		opts := tserv.AddOpt.Thread(tid)

		invite, err := cbor.NewInvite(ts1.Logs(tid), true)
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

		_, _, err = ts1.Add(context.Background(), invite, opts)
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
