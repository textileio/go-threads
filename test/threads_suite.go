package test

import (
	"context"
	"testing"
	"time"

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
	check(t, err)
	host, err := libp2p.New(
		context.Background(),
		libp2p.ListenAddrs(listen),
		libp2p.Identity(sk),
	)
	check(t, err)

	bs := bstore.NewBlockstore(syncds.MutexWrap(ds.NewMapDatastore()))
	bsrv := bserv.New(bs, offline.Exchange(bs))
	ts, err := threads.NewThreads(
		context.Background(),
		host,
		bsrv,
		dag.NewDAGService(bsrv),
		tstore.NewThreadstore(),
		true)
	check(t, err)
	return ts
}

func testAddPull(ts1, _ tserv.Threadservice) func(t *testing.T) {
	return func(t *testing.T) {
		listener := ts1.Listen()
		var rcount int
		go func() {
			for r := range listener.Channel() {
				rcount++
				t.Logf("got record %s", r.Value().Cid())
			}
		}()

		ctx := context.Background()
		tid := thread.NewIDV1(thread.Raw, 32)

		body, err := cbornode.WrapObject(map[string]interface{}{
			"foo": "bar",
			"baz": []byte("howdy"),
		}, mh.SHA2_256, -1)
		check(t, err)

		r1, err := ts1.Add(ctx, body, tserv.AddOpt.ThreadID(tid))
		check(t, err)
		if r1.Value() == nil {
			t.Fatalf("expected node to not be nil")
		}

		r2, err := ts1.Add(ctx, body, tserv.AddOpt.ThreadID(tid))
		check(t, err)
		if r2.Value() == nil {
			t.Fatalf("expected node to not be nil")
		}

		if r1.LogID().String() != r2.LogID().String() {
			t.Fatalf("expected log IDs to match, got %s and %s", r1.LogID().String(), r2.LogID().String())
		}

		// Pull from the log origin
		err = ts1.Pull(ctx, tid)
		check(t, err)
		time.Sleep(time.Second)
		if rcount != 2 {
			t.Fatalf("expected 2 records got %d", rcount)
		}

		r1b, err := ts1.Get(ctx, tid, r1.LogID(), r1.Value().Cid())
		check(t, err)

		event, err := cbor.GetEvent(ctx, ts1.DAGService(), r1b.BlockID())
		check(t, err)

		rk, err := ts1.ReadKey(tid, r1.LogID())
		check(t, err)
		readKey, err := crypto.ParseDecryptionKey(rk)
		check(t, err)
		back, err := event.GetBody(ctx, ts1.DAGService(), readKey)
		check(t, err)

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
		check(t, err)
		r1, err := ts1.Add(ctx, body, tserv.AddOpt.ThreadID(tid))
		check(t, err)

		lgs, err := ts1.GetLogs(tid)
		check(t, err)
		invite, err := cbor.NewInvite(lgs, true)
		check(t, err)

		pk := ts1.Host().Peerstore().PubKey(ts2.Host().ID())
		if pk == nil {
			t.Fatal("public key not found")
		}
		ek, err := asymmetric.NewEncryptionKey(pk)
		check(t, err)

		a, err := ma.NewMultiaddr("/p2p/" + ts2.Host().ID().String())
		check(t, err)

		r2, err := ts1.Add(
			context.Background(),
			invite,
			tserv.AddOpt.ThreadID(tid),
			tserv.AddOpt.Key(ek),
			tserv.AddOpt.Addrs([]ma.Multiaddr{a}))
		check(t, err)

		info1, err := ts1.ThreadInfo(tid)
		check(t, err)
		if len(info1.Logs) != 2 {
			t.Fatalf("expected 2 logs got %d", len(info1.Logs))
		}
		for _, lid := range info1.Logs {
			if lid.String() == r1.LogID().String() {
				// Peer 1 should have 2 records in its own log (one msg
				// and one invite record)
				_, err = ts1.Get(ctx, tid, lid, r1.Value().Cid())
				check(t, err)
				_, err := ts1.Get(ctx, tid, lid, r2.Value().Cid())
				check(t, err)
			} else {
				// Peer 1 should have 1 record in its log for peer 2 (one invite record)
				heads, err := ts1.Heads(tid, lid)
				check(t, err)
				if len(heads) != 1 { // double check we only have one head
					t.Fatalf("expected 1 head got %d", len(heads))
				}
				_, err = ts1.Get(ctx, tid, lid, heads[0])
				check(t, err)
			}
		}

		info2, err := ts2.ThreadInfo(tid)
		check(t, err)
		if len(info2.Logs) != 2 {
			t.Fatalf("expected 2 logs got %d", len(info2.Logs))
		}
		for _, lid := range info2.Logs {
			if lid.String() == r1.LogID().String() {
				// Peer 2 should have 2 records in its log for peer 1 (one msg
				// and one invite record)
				_, err = ts2.Get(ctx, tid, lid, r1.Value().Cid())
				check(t, err)
				_, err := ts2.Get(ctx, tid, lid, r2.Value().Cid())
				check(t, err)
			} else {
				// Peer 2 should have 1 record in its own log (one invite record)
				heads, err := ts2.Heads(tid, lid)
				check(t, err)
				if len(heads) != 1 { // double check we only have one head
					t.Fatalf("expected 1 head got %d", len(heads))
				}
				_, err = ts2.Get(ctx, tid, lid, heads[0])
				check(t, err)
			}
		}
	}
}

func testClose(ts1, ts2 tserv.Threadservice) func(t *testing.T) {
	return func(t *testing.T) {
		err := ts1.Close()
		check(t, err)
		err = ts2.Close()
		check(t, err)
	}
}
