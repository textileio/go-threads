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
	"github.com/textileio/go-textile-core/thread"
	tserv "github.com/textileio/go-textile-core/threadservice"
	threads "github.com/textileio/go-textile-threads"
	"github.com/textileio/go-textile-threads/cbor"
	tstore "github.com/textileio/go-textile-threads/tstoremem"
)

var threadsSuite = map[string]func(tserv.Threadservice, tserv.Threadservice) func(*testing.T){
	"AddPull":     testAddPull,
	"AddPeer":     testAddPeer,
	"AddFollower": testAddFollower,
	"Close":       testClose,
}

func ThreadsTest(t *testing.T) {
	for name, test := range threadsSuite {
		// Create two thread services.
		m1, _ := ma.NewMultiaddr("/ip4/127.0.0.1/tcp/10000")
		ts1 := newService(t, m1)
		m2, _ := ma.NewMultiaddr("/ip4/127.0.0.1/tcp/10001")
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
		bsrv.Blockstore(),
		dag.NewDAGService(bsrv),
		tstore.NewThreadstore(),
		nil,
		true)
	check(t, err)
	return ts
}

func testAddPull(ts1, _ tserv.Threadservice) func(t *testing.T) {
	return func(t *testing.T) {
		sub := ts1.Subscribe()
		var rcount int
		go func() {
			for r := range sub.Channel() {
				rcount++
				t.Logf("got record %s", r.Value().Cid())
			}
		}()

		ctx := context.Background()
		id := thread.NewIDV1(thread.Raw, 32)

		body, err := cbornode.WrapObject(map[string]interface{}{
			"foo": "bar",
			"baz": []byte("howdy"),
		}, mh.SHA2_256, -1)
		check(t, err)

		r1, err := ts1.AddRecord(ctx, body, tserv.AddOpt.ThreadID(id))
		check(t, err)
		if r1.Value() == nil {
			t.Fatalf("expected node to not be nil")
		}

		r2, err := ts1.AddRecord(ctx, body, tserv.AddOpt.ThreadID(id))
		check(t, err)
		if r2.Value() == nil {
			t.Fatalf("expected node to not be nil")
		}

		if r1.LogID().String() != r2.LogID().String() {
			t.Fatalf("expected log IDs to match, got %s and %s", r1.LogID().String(), r2.LogID().String())
		}

		// Pull from the origin
		err = ts1.PullThread(ctx, id)
		check(t, err)
		time.Sleep(time.Second)
		if rcount != 2 {
			t.Fatalf("expected 2 records got %d", rcount)
		}

		r1b, err := ts1.GetRecord(ctx, id, r1.LogID(), r1.Value().Cid())
		check(t, err)

		event, err := cbor.GetEvent(ctx, ts1, r1b.BlockID())
		check(t, err)

		lg, err := ts1.Store().LogInfo(id, r1.LogID())
		check(t, err)
		back, err := event.GetBody(ctx, ts1, lg.ReadKey)
		check(t, err)

		if body.String() != back.String() {
			t.Fatalf("retrieved body does not equal input body")
		}
	}
}

func testAddPeer(ts1, ts2 tserv.Threadservice) func(t *testing.T) {
	return func(t *testing.T) {
		ctx := context.Background()
		id := thread.NewIDV1(thread.Raw, 32)

		body, err := cbornode.WrapObject(map[string]interface{}{
			"msg": "yo!",
		}, mh.SHA2_256, -1)
		check(t, err)
		_, err = ts1.AddRecord(ctx, body, tserv.AddOpt.ThreadID(id))
		check(t, err)

		addr, err := ma.NewMultiaddr("/p2p/" + ts1.Host().ID().String() + "/thread/" + id.String())
		check(t, err)

		info, err := ts2.AddThread(ctx, addr)
		check(t, err)
		if info.Logs.Len() != 1 {
			t.Fatalf("expected 1 log got %d", info.Logs.Len())
		}

		body2, err := cbornode.WrapObject(map[string]interface{}{
			"msg": "yo back!",
		}, mh.SHA2_256, -1)
		check(t, err)
		_, err = ts2.AddRecord(ctx, body2, tserv.AddOpt.ThreadID(id))
		check(t, err)

		info2, err := ts1.Store().ThreadInfo(id)
		check(t, err)
		if info2.Logs.Len() != 2 {
			t.Fatalf("expected 2 logs got %d", info2.Logs.Len())
		}
	}
}

func testAddFollower(ts1, ts2 tserv.Threadservice) func(t *testing.T) {
	return func(t *testing.T) {
		ctx := context.Background()
		id := thread.NewIDV1(thread.Raw, 32)

		body, err := cbornode.WrapObject(map[string]interface{}{
			"msg": "yo!",
		}, mh.SHA2_256, -1)
		check(t, err)
		r, err := ts1.AddRecord(ctx, body, tserv.AddOpt.ThreadID(id))
		check(t, err)

		t.Logf("adding follower %s", ts2.Host().ID().String())
		err = ts1.AddFollower(ctx, id, ts2.Host().ID())
		check(t, err)

		info, err := ts2.Store().ThreadInfo(id)
		check(t, err)
		if info.Logs.Len() != 1 {
			t.Fatalf("expected 1 log got %d", info.Logs.Len())
		}

		addrs, err := ts1.Store().Addrs(id, r.LogID())
		check(t, err)
		if len(addrs) != 2 {
			t.Fatalf("expected 2 addresses got %d", len(addrs))
		}

		addrs2, err := ts2.Store().Addrs(id, r.LogID())
		check(t, err)
		if len(addrs2) != 2 {
			t.Fatalf("expected 2 addresses got %d", len(addrs2))
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
