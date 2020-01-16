package service

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
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peerstore"
	ma "github.com/multiformats/go-multiaddr"
	mh "github.com/multiformats/go-multihash"
	"github.com/textileio/go-threads/cbor"
	core "github.com/textileio/go-threads/core/service"
	"github.com/textileio/go-threads/core/thread"
	tstore "github.com/textileio/go-threads/logstore/lstoremem"
	"github.com/textileio/go-threads/util"
)

func TestAddPull(t *testing.T) {
	t.Parallel()
	s := makeService(t)
	defer s.Close()

	t.Run("test add and pull record", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		sub, err := s.Subscribe(ctx)
		if err != nil {
			t.Fatal(err)
		}
		var rcount int
		go func() {
			for r := range sub {
				rcount++
				t.Logf("got record %s", r.Value().Cid())
			}
		}()

		th, err := util.CreateThread(s, thread.NewIDV1(thread.Raw, 32))
		if err != nil {
			t.Fatal(err)
		}

		body, err := cbornode.WrapObject(map[string]interface{}{
			"foo": "bar",
			"baz": []byte("howdy"),
		}, mh.SHA2_256, -1)
		if err != nil {
			t.Fatal(err)
		}

		r1, err := s.AddRecord(ctx, th.ID, body)
		if err != nil {
			t.Fatal(err)
		}
		if r1.Value() == nil {
			t.Fatalf("expected node to not be nil")
		}

		r2, err := s.AddRecord(ctx, th.ID, body)
		if err != nil {
			t.Fatal(err)
		}
		if r2.Value() == nil {
			t.Fatalf("expected node to not be nil")
		}

		if r1.LogID().String() != r2.LogID().String() {
			t.Fatalf("expected log IDs to match, got %s and %s", r1.LogID().String(), r2.LogID().String())
		}

		// Pull from the origin
		err = s.PullThread(ctx, th.ID)
		if err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Second)
		if rcount != 2 {
			t.Fatalf("expected 2 records got %d", rcount)
		}

		r1b, err := s.GetRecord(ctx, th.ID, r1.Value().Cid())
		if err != nil {
			t.Fatal(err)
		}

		event, err := cbor.GetEvent(ctx, s, r1b.BlockID())
		if err != nil {
			t.Fatal(err)
		}

		back, err := event.GetBody(ctx, s, th.ReadKey)
		if err != nil {
			t.Fatal(err)
		}

		if body.String() != back.String() {
			t.Fatalf("retrieved body does not equal input body")
		}
	})
}

func TestAddPeer(t *testing.T) {
	t.Parallel()
	s1 := makeService(t)
	defer s1.Close()
	s2 := makeService(t)
	defer s2.Close()

	s1.Host().Peerstore().AddAddrs(s2.Host().ID(), s2.Host().Addrs(), peerstore.PermanentAddrTTL)
	s2.Host().Peerstore().AddAddrs(s1.Host().ID(), s1.Host().Addrs(), peerstore.PermanentAddrTTL)

	t.Run("test add thread peer", func(t *testing.T) {
		ctx := context.Background()
		th, err := util.CreateThread(s1, thread.NewIDV1(thread.Raw, 32))
		if err != nil {
			t.Fatal(err)
		}

		body, err := cbornode.WrapObject(map[string]interface{}{
			"msg": "yo!",
		}, mh.SHA2_256, -1)
		if err != nil {
			t.Fatal(err)
		}
		_, err = s1.AddRecord(ctx, th.ID, body)
		if err != nil {
			t.Fatal(err)
		}

		addr, err := ma.NewMultiaddr("/p2p/" + s1.Host().ID().String() + "/thread/" + th.ID.String())
		if err != nil {
			t.Fatal(err)
		}

		info, err := s2.AddThread(ctx, addr, core.FollowKey(th.FollowKey), core.ReadKey(th.ReadKey))
		if err != nil {
			t.Fatal(err)
		}
		if info.Logs.Len() != 1 {
			t.Fatalf("expected 1 log got %d", info.Logs.Len())
		}

		body2, err := cbornode.WrapObject(map[string]interface{}{
			"msg": "yo back!",
		}, mh.SHA2_256, -1)
		if err != nil {
			t.Fatal(err)
		}
		_, err = s2.AddRecord(ctx, th.ID, body2)
		if err != nil {
			t.Fatal(err)
		}

		info2, err := s1.Store().ThreadInfo(th.ID)
		if err != nil {
			t.Fatal(err)
		}
		if info2.Logs.Len() != 2 {
			t.Fatalf("expected 2 logs got %d", info2.Logs.Len())
		}
	})
}

func TestAddFollower(t *testing.T) {
	t.Parallel()
	s1 := makeService(t)
	defer s1.Close()
	s2 := makeService(t)
	defer s2.Close()

	s1.Host().Peerstore().AddAddrs(s2.Host().ID(), s2.Host().Addrs(), peerstore.PermanentAddrTTL)
	s2.Host().Peerstore().AddAddrs(s1.Host().ID(), s1.Host().Addrs(), peerstore.PermanentAddrTTL)

	t.Run("test add thread follower", func(t *testing.T) {
		ctx := context.Background()
		th, err := util.CreateThread(s1, thread.NewIDV1(thread.Raw, 32))
		if err != nil {
			t.Fatal(err)
		}

		body, err := cbornode.WrapObject(map[string]interface{}{
			"msg": "yo!",
		}, mh.SHA2_256, -1)
		if err != nil {
			t.Fatal(err)
		}
		r, err := s1.AddRecord(ctx, th.ID, body)
		if err != nil {
			t.Fatal(err)
		}

		t.Logf("adding follower %s", s2.Host().ID().String())
		err = s1.AddFollower(ctx, th.ID, s2.Host().ID())
		if err != nil {
			t.Fatal(err)
		}

		info, err := s2.Store().ThreadInfo(th.ID)
		if err != nil {
			t.Fatal(err)
		}
		if info.Logs.Len() != 1 {
			t.Fatalf("expected 1 log got %d", info.Logs.Len())
		}

		addrs, err := s1.Store().Addrs(th.ID, r.LogID())
		if err != nil {
			t.Fatal(err)
		}
		if len(addrs) != 2 {
			t.Fatalf("expected 2 addresses got %d", len(addrs))
		}

		addrs2, err := s2.Store().Addrs(th.ID, r.LogID())
		if err != nil {
			t.Fatal(err)
		}
		if len(addrs2) != 2 {
			t.Fatalf("expected 2 addresses got %d", len(addrs2))
		}
	})
}

func TestClose(t *testing.T) {
	t.Parallel()
	s := makeService(t)
	defer s.Close()

	t.Run("test close", func(t *testing.T) {
		if err := s.Close(); err != nil {
			t.Fatal(err)
		}
	})
}

func makeService(t *testing.T) core.Service {
	sk, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 0)
	if err != nil {
		t.Fatal(err)
	}
	addr := util.MustParseAddr("/ip4/127.0.0.1/tcp/0")

	host, err := libp2p.New(
		context.Background(),
		libp2p.ListenAddrs(addr),
		libp2p.Identity(sk),
	)
	if err != nil {
		t.Fatal(err)
	}
	bs := bstore.NewBlockstore(syncds.MutexWrap(ds.NewMapDatastore()))
	bsrv := bserv.New(bs, offline.Exchange(bs))
	ts, err := NewService(
		context.Background(),
		host,
		bsrv.Blockstore(),
		dag.NewDAGService(bsrv),
		tstore.NewLogstore(),
		Config{
			Debug: true,
		})
	if err != nil {
		t.Fatal(err)
	}
	return ts
}
