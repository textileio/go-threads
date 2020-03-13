package service

import (
	"context"
	"testing"

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
	sym "github.com/textileio/go-threads/crypto/symmetric"
	tstore "github.com/textileio/go-threads/logstore/lstoremem"
	"github.com/textileio/go-threads/util"
)

func TestService_CreateRecord(t *testing.T) {
	t.Parallel()
	s := makeService(t)
	defer s.Close()

	t.Run("test create record", func(t *testing.T) {
		ctx := context.Background()
		info := createThread(t, ctx, s)

		body, err := cbornode.WrapObject(map[string]interface{}{
			"foo": "bar",
			"baz": []byte("howdy"),
		}, mh.SHA2_256, -1)
		if err != nil {
			t.Fatal(err)
		}

		r1, err := s.CreateRecord(ctx, info.ID, body)
		if err != nil {
			t.Fatal(err)
		}
		if r1.Value() == nil {
			t.Fatalf("expected node to not be nil")
		}

		r2, err := s.CreateRecord(ctx, info.ID, body)
		if err != nil {
			t.Fatal(err)
		}
		if r2.Value() == nil {
			t.Fatalf("expected node to not be nil")
		}

		if r1.LogID().String() != r2.LogID().String() {
			t.Fatalf("expected log IDs to match, got %s and %s", r1.LogID().String(), r2.LogID().String())
		}

		r1b, err := s.GetRecord(ctx, info.ID, r1.Value().Cid())
		if err != nil {
			t.Fatal(err)
		}

		event, err := cbor.GetEvent(ctx, s, r1b.BlockID())
		if err != nil {
			t.Fatal(err)
		}

		back, err := event.GetBody(ctx, s, info.ReadKey)
		if err != nil {
			t.Fatal(err)
		}

		if body.String() != back.String() {
			t.Fatalf("retrieved body does not equal input body")
		}
	})
}

func TestService_AddThread(t *testing.T) {
	t.Parallel()
	s1 := makeService(t)
	defer s1.Close()
	s2 := makeService(t)
	defer s2.Close()

	s1.Host().Peerstore().AddAddrs(s2.Host().ID(), s2.Host().Addrs(), peerstore.PermanentAddrTTL)
	s2.Host().Peerstore().AddAddrs(s1.Host().ID(), s1.Host().Addrs(), peerstore.PermanentAddrTTL)

	t.Run("test add thread", func(t *testing.T) {
		ctx := context.Background()
		info := createThread(t, ctx, s1)

		body, err := cbornode.WrapObject(map[string]interface{}{
			"msg": "yo!",
		}, mh.SHA2_256, -1)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = s1.CreateRecord(ctx, info.ID, body); err != nil {
			t.Fatal(err)
		}

		addr, err := ma.NewMultiaddr("/p2p/" + s1.Host().ID().String() + "/thread/" + info.ID.String())
		if err != nil {
			t.Fatal(err)
		}

		info2, err := s2.AddThread(ctx, addr, core.FollowKey(info.FollowKey), core.ReadKey(info.ReadKey))
		if err != nil {
			t.Fatal(err)
		}
		if len(info2.Logs) != 1 {
			t.Fatalf("expected 1 log got %d", len(info2.Logs))
		}

		body2, err := cbornode.WrapObject(map[string]interface{}{
			"msg": "yo back!",
		}, mh.SHA2_256, -1)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = s2.CreateRecord(ctx, info2.ID, body2); err != nil {
			t.Fatal(err)
		}

		info3, err := s1.GetThread(context.Background(), info.ID)
		if err != nil {
			t.Fatal(err)
		}
		if len(info3.Logs) != 2 {
			t.Fatalf("expected 2 logs got %d", len(info3.Logs))
		}
	})
}

func TestService_AddFollower(t *testing.T) {
	t.Parallel()
	s1 := makeService(t)
	defer s1.Close()
	s2 := makeService(t)
	defer s2.Close()

	s1.Host().Peerstore().AddAddrs(s2.Host().ID(), s2.Host().Addrs(), peerstore.PermanentAddrTTL)
	s2.Host().Peerstore().AddAddrs(s1.Host().ID(), s1.Host().Addrs(), peerstore.PermanentAddrTTL)

	t.Run("test add follower", func(t *testing.T) {
		ctx := context.Background()
		info := createThread(t, ctx, s1)

		body, err := cbornode.WrapObject(map[string]interface{}{
			"msg": "yo!",
		}, mh.SHA2_256, -1)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s1.CreateRecord(ctx, info.ID, body); err != nil {
			t.Fatal(err)
		}

		addr, err := ma.NewMultiaddr("/p2p/" + s2.Host().ID().String())
		if err != nil {
			t.Fatal(err)
		}
		if _, err = s1.AddFollower(ctx, info.ID, addr); err != nil {
			t.Fatal(err)
		}

		info2, err := s1.GetThread(context.Background(), info.ID)
		if err != nil {
			t.Fatal(err)
		}
		if len(info2.Logs) != 1 {
			t.Fatalf("expected 1 log got %d", len(info2.Logs))
		}
		if len(info2.Logs[0].Addrs) != 2 {
			t.Fatalf("expected 2 addresses got %d", len(info2.Logs[0].Addrs))
		}

		info3, err := s2.GetThread(context.Background(), info.ID)
		if err != nil {
			t.Fatal(err)
		}
		if len(info3.Logs) != 1 {
			t.Fatalf("expected 1 log got %d", len(info2.Logs))
		}
		if len(info3.Logs[0].Addrs) != 2 {
			t.Fatalf("expected 2 addresses got %d", len(info3.Logs[0].Addrs))
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

func createThread(t *testing.T, ctx context.Context, api core.API) thread.Info {
	id := thread.NewIDV1(thread.Raw, 32)
	info, err := api.CreateThread(ctx, id, core.FollowKey(sym.New()), core.ReadKey(sym.New()))
	if err != nil {
		t.Fatal(err)
	}
	return info
}
