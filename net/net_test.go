package net

import (
	"context"
	rand "crypto/rand"
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
	"github.com/textileio/go-threads/core/logstore"
	core "github.com/textileio/go-threads/core/net"
	"github.com/textileio/go-threads/core/thread"
	tstore "github.com/textileio/go-threads/logstore/lstoremem"
	"github.com/textileio/go-threads/util"
)

func TestNet_GetToken(t *testing.T) {
	t.Parallel()
	n := makeNetwork(t)
	defer n.Close()
	ctx := context.Background()

	sk, _, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tok, err := n.GetToken(ctx, thread.NewLibp2pIdentity(sk))
	if err != nil {
		t.Fatal(err)
	}
	if tok == "" {
		t.Fatal("bad token")
	}
}

func TestNet_CreateRecord(t *testing.T) {
	t.Parallel()
	n := makeNetwork(t)
	defer n.Close()
	ctx := context.Background()

	var info thread.Info
	t.Run("test create thread", func(t *testing.T) {
		info = createThread(t, ctx, n)
		if len(info.Addrs) == 0 {
			t.Fatalf("expected more than 0 addresses got %d", len(info.Addrs))
		}
	})

	t.Run("test create records", func(t *testing.T) {
		body, err := cbornode.WrapObject(map[string]interface{}{
			"foo": "bar",
			"baz": []byte("howdy"),
		}, mh.SHA2_256, -1)
		if err != nil {
			t.Fatal(err)
		}

		r1, err := n.CreateRecord(ctx, info.ID, body)
		if err != nil {
			t.Fatal(err)
		}
		if r1.Value() == nil {
			t.Fatalf("expected node to not be nil")
		}

		r2, err := n.CreateRecord(ctx, info.ID, body)
		if err != nil {
			t.Fatal(err)
		}
		if r2.Value() == nil {
			t.Fatalf("expected node to not be nil")
		}

		if r1.LogID().String() != r2.LogID().String() {
			t.Fatalf("expected log IDs to match, got %s and %s", r1.LogID(), r2.LogID())
		}

		r1b, err := n.GetRecord(ctx, info.ID, r1.Value().Cid())
		if err != nil {
			t.Fatal(err)
		}

		event, err := cbor.GetEvent(ctx, n, r1b.BlockID())
		if err != nil {
			t.Fatal(err)
		}

		back, err := event.GetBody(ctx, n, info.Key.Read())
		if err != nil {
			t.Fatal(err)
		}

		if body.String() != back.String() {
			t.Fatalf("retrieved body does not equal input body")
		}
	})
}

func TestNet_AddThread(t *testing.T) {
	t.Parallel()
	n1 := makeNetwork(t)
	defer n1.Close()
	n2 := makeNetwork(t)
	defer n2.Close()

	n1.Host().Peerstore().AddAddrs(n2.Host().ID(), n2.Host().Addrs(), peerstore.PermanentAddrTTL)
	n2.Host().Peerstore().AddAddrs(n1.Host().ID(), n1.Host().Addrs(), peerstore.PermanentAddrTTL)

	ctx := context.Background()
	info := createThread(t, ctx, n1)

	body, err := cbornode.WrapObject(map[string]interface{}{
		"msg": "yo!",
	}, mh.SHA2_256, -1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = n1.CreateRecord(ctx, info.ID, body); err != nil {
		t.Fatal(err)
	}

	addr, err := ma.NewMultiaddr("/p2p/" + n1.Host().ID().String() + "/thread/" + info.ID.String())
	if err != nil {
		t.Fatal(err)
	}

	info2, err := n2.AddThread(ctx, addr, core.WithThreadKey(info.Key))
	if err != nil {
		t.Fatal(err)
	}
	if err := n2.PullThread(ctx, info2.ID); err != nil {
		t.Fatal(err)
	}
	if len(info2.Logs) != 2 {
		t.Fatalf("expected 2 logs got %d", len(info2.Logs))
	}
	if len(info2.Addrs) == 0 {
		t.Fatalf("expected more than 0 addresses got %d", len(info2.Addrs))
	}

	body2, err := cbornode.WrapObject(map[string]interface{}{
		"msg": "yo back!",
	}, mh.SHA2_256, -1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = n2.CreateRecord(ctx, info2.ID, body2); err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Second)

	info3, err := n1.GetThread(context.Background(), info.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(info3.Logs) != 2 {
		t.Fatalf("expected 2 logs got %d", len(info3.Logs))
	}
	if len(info3.Addrs) == 0 {
		t.Fatalf("expected more than 0 addresses got %d", len(info3.Addrs))
	}
}

func TestNet_CreateThreadManaged(t *testing.T) {
	t.Parallel()
	n := makeNetwork(t)
	defer n.Close()

	ctx := context.Background()
	info, err := n.CreateThread(ctx, thread.NewIDV1(thread.Raw, 32))
	if err != nil {
		t.Fatal(err)
	}
	sk, pk, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	// Should fail if trying to re-create locally 'owned' thread
	_, err = n.CreateThread(ctx, info.ID, core.WithLogKey(sk), core.WithThreadKey(info.Key))
	if err == nil {
		t.Fatalf("expected creating thread with second private key to fail")
	}
	// Should fail if trying to re-create thread with wrong read/service keys
	_, err = n.CreateThread(ctx, info.ID, core.WithLogKey(pk))
	if err == nil {
		t.Fatalf("expected to fail when using wrong thread key(s)")
	}
	// Should work if only going to 'manage' re-created thread/log
	_, err = n.CreateThread(ctx, info.ID, core.WithLogKey(pk), core.WithThreadKey(info.Key))
	if err != nil {
		t.Fatal(err)
	}
}

func TestNet_AddThreadManaged(t *testing.T) {
	t.Parallel()
	n1 := makeNetwork(t)
	defer n1.Close()
	n2 := makeNetwork(t)
	defer n2.Close()

	n1.Host().Peerstore().AddAddrs(n2.Host().ID(), n2.Host().Addrs(), peerstore.PermanentAddrTTL)
	n2.Host().Peerstore().AddAddrs(n1.Host().ID(), n1.Host().Addrs(), peerstore.PermanentAddrTTL)

	ctx := context.Background()
	info, err := n1.CreateThread(ctx, thread.NewIDV1(thread.Raw, 32))
	if err != nil {
		t.Fatal(err)
	}

	addr, err := ma.NewMultiaddr("/p2p/" + n1.Host().ID().String() + "/thread/" + info.ID.String())
	if err != nil {
		t.Fatal(err)
	}
	sk, pk, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	_, err = n2.AddThread(ctx, addr, core.WithThreadKey(info.Key))
	if err != nil {
		t.Fatal(err)
	}
	// Should fail if trying to re-create locally 'owned' thread
	_, err = n2.AddThread(ctx, addr, core.WithLogKey(sk), core.WithThreadKey(info.Key))
	if err == nil {
		t.Fatalf("expected creating thread with second private key to fail")
	}
	// Should fail if trying to re-create thread with wrong/missing read/service keys
	_, err = n2.AddThread(ctx, addr, core.WithLogKey(pk))
	if err == nil {
		t.Fatalf("expected to fail when using wrong thread key(s)")
	}
	// Should work if only going to 'manage' re-created thread/log
	_, err = n2.AddThread(ctx, addr, core.WithLogKey(pk), core.WithThreadKey(info.Key))
	if err != nil {
		t.Fatal(err)
	}
}

func TestNet_AddReplicator(t *testing.T) {
	t.Parallel()
	n1 := makeNetwork(t)
	defer n1.Close()
	n2 := makeNetwork(t)
	defer n2.Close()

	n1.Host().Peerstore().AddAddrs(n2.Host().ID(), n2.Host().Addrs(), peerstore.PermanentAddrTTL)
	n2.Host().Peerstore().AddAddrs(n1.Host().ID(), n1.Host().Addrs(), peerstore.PermanentAddrTTL)

	ctx := context.Background()
	info := createThread(t, ctx, n1)

	body, err := cbornode.WrapObject(map[string]interface{}{
		"msg": "yo!",
	}, mh.SHA2_256, -1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := n1.CreateRecord(ctx, info.ID, body); err != nil {
		t.Fatal(err)
	}

	addr, err := ma.NewMultiaddr("/p2p/" + n2.Host().ID().String())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = n1.AddReplicator(ctx, info.ID, addr); err != nil {
		t.Fatal(err)
	}

	info2, err := n1.GetThread(context.Background(), info.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(info2.Logs) != 1 {
		t.Fatalf("expected 1 log got %d", len(info2.Logs))
	}
	if len(info2.Logs[0].Addrs) != 2 {
		t.Fatalf("expected 2 addresses got %d", len(info2.Logs[0].Addrs))
	}

	info3, err := n2.GetThread(context.Background(), info.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(info3.Logs) != 1 {
		t.Fatalf("expected 1 log got %d", len(info2.Logs))
	}
	if len(info3.Logs[0].Addrs) != 2 {
		t.Fatalf("expected 2 addresses got %d", len(info3.Logs[0].Addrs))
	}
}

func TestNet_AddReplicatorManaged(t *testing.T) {
	t.Parallel()
	n1 := makeNetwork(t)
	defer n1.Close()
	n2 := makeNetwork(t)
	defer n2.Close()

	n1.Host().Peerstore().AddAddrs(n2.Host().ID(), n2.Host().Addrs(), peerstore.PermanentAddrTTL)
	n2.Host().Peerstore().AddAddrs(n1.Host().ID(), n1.Host().Addrs(), peerstore.PermanentAddrTTL)

	// Create managed thread
	tid := thread.NewIDV1(thread.Raw, 32)
	ctx := context.Background()
	_, pk, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	info, err := n1.CreateThread(ctx, tid, core.WithLogKey(pk))
	if err != nil {
		t.Fatal(err)
	}

	addr, err := ma.NewMultiaddr("/p2p/" + n2.Host().ID().String())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = n1.AddReplicator(ctx, info.ID, addr); err != nil {
		t.Fatal(err)
	}

	info2, err := n1.GetThread(context.Background(), info.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(info2.Logs) != 1 {
		t.Fatalf("expected 1 log got %d", len(info2.Logs))
	}
	if len(info2.Logs[0].Addrs) != 2 {
		t.Fatalf("expected 2 addresses got %d", len(info2.Logs[0].Addrs))
	}

	info3, err := n2.GetThread(context.Background(), info.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(info3.Logs) != 1 {
		t.Fatalf("expected 1 log got %d", len(info2.Logs))
	}
	if len(info3.Logs[0].Addrs) != 2 {
		t.Fatalf("expected 2 addresses got %d", len(info3.Logs[0].Addrs))
	}
}

func TestNet_DeleteThread(t *testing.T) {
	t.Parallel()
	n := makeNetwork(t)
	defer n.Close()

	ctx := context.Background()
	info := createThread(t, ctx, n)

	body, err := cbornode.WrapObject(map[string]interface{}{
		"foo": "bar",
		"baz": []byte("howdy"),
	}, mh.SHA2_256, -1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := n.CreateRecord(ctx, info.ID, body); err != nil {
		t.Fatal(err)
	}
	if _, err := n.CreateRecord(ctx, info.ID, body); err != nil {
		t.Fatal(err)
	}

	if err = n.DeleteThread(ctx, info.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := n.GetThread(ctx, info.ID); err != logstore.ErrThreadNotFound {
		t.Fatal("thread was not deleted")
	}
}

func TestClose(t *testing.T) {
	t.Parallel()
	n := makeNetwork(t)
	defer n.Close()

	t.Run("test close", func(t *testing.T) {
		if err := n.Close(); err != nil {
			t.Fatal(err)
		}
	})
}

func makeNetwork(t *testing.T) core.Net {
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
	n, err := NewNetwork(
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
	return n
}

func createThread(t *testing.T, ctx context.Context, api core.API) thread.Info {
	info, err := api.CreateThread(ctx, thread.NewIDV1(thread.Raw, 32))
	if err != nil {
		t.Fatal(err)
	}
	return info
}
