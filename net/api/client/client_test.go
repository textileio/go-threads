package client_test

import (
	"context"
	crand "crypto/rand"
	"log"
	"sync"
	"testing"
	"time"

	"github.com/ipfs/go-cid"
	cbornode "github.com/ipfs/go-ipld-cbor"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"
	mh "github.com/multiformats/go-multihash"
	"github.com/textileio/go-threads/cbor"
	core "github.com/textileio/go-threads/core/net"
	"github.com/textileio/go-threads/core/thread"
	sym "github.com/textileio/go-threads/crypto/symmetric"
	"github.com/textileio/go-threads/net/api"
	. "github.com/textileio/go-threads/net/api/client"
	"github.com/textileio/go-threads/util"
	"google.golang.org/grpc"
)

func TestClient_GetHostID(t *testing.T) {
	t.Parallel()
	_, client, done := setup(t)
	defer done()

	t.Run("test get host ID", func(t *testing.T) {
		if _, err := client.GetHostID(context.Background()); err != nil {
			t.Fatalf("failed to get host ID: %v", err)
		}
	})
}

func TestClient_GetToken(t *testing.T) {
	t.Parallel()
	_, client, done := setup(t)
	defer done()

	identity := createIdentity(t)

	t.Run("test get token", func(t *testing.T) {
		tok, err := client.GetToken(context.Background(), identity)
		if err != nil {
			t.Fatalf("failed to get token: %v", err)
		}
		if tok == "" {
			t.Fatal("emtpy token")
		}
	})
}

func TestClient_CreateThread(t *testing.T) {
	t.Parallel()
	_, client, done := setup(t)
	defer done()

	t.Run("test create thread", func(t *testing.T) {
		id := thread.NewIDV1(thread.Raw, 32)
		info, err := client.CreateThread(context.Background(), id)
		if err != nil {
			t.Fatalf("failed to create thread: %v", err)
		}
		if !info.ID.Equals(id) {
			t.Fatal("got bad ID from create thread")
		}
		if info.Key.Read() == nil {
			t.Fatal("read key should not be nil")
		}
		if info.Key.Service() == nil {
			t.Fatal("service key should not be nil")
		}
	})
}

func TestClient_AddThread(t *testing.T) {
	t.Parallel()
	hostAddr1, client1, done1 := setup(t)
	defer done1()
	_, client2, done2 := setup(t)
	defer done2()

	hostID1, err := client1.GetHostID(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	info1 := createThread(t, client1)
	addr := threadAddr(t, hostAddr1, hostID1, info1)

	t.Run("test add thread", func(t *testing.T) {
		info2, err := client2.AddThread(context.Background(), addr, core.WithThreadKey(info1.Key))
		if err != nil {
			t.Fatalf("failed to add thread: %v", err)
		}
		if !info2.ID.Equals(info1.ID) {
			t.Fatal("got bad ID from add thread")
		}
	})
}

func TestClient_GetThread(t *testing.T) {
	t.Parallel()
	_, client, done := setup(t)
	defer done()

	info := createThread(t, client)

	t.Run("test get thread", func(t *testing.T) {
		info2, err := client.GetThread(context.Background(), info.ID)
		if err != nil {
			t.Fatalf("failed to get thread: %v", err)
		}
		if !info2.ID.Equals(info.ID) {
			t.Fatal("got bad ID from get thread")
		}
	})
}

func TestClient_PullThread(t *testing.T) {
	t.Parallel()
	_, client, done := setup(t)
	defer done()

	info := createThread(t, client)

	t.Run("test pull thread", func(t *testing.T) {
		if err := client.PullThread(context.Background(), info.ID); err != nil {
			t.Fatalf("failed to pull thread: %v", err)
		}
	})
}

func TestClient_DeleteThread(t *testing.T) {
	t.Parallel()
	_, client, done := setup(t)
	defer done()

	info := createThread(t, client)

	t.Run("test delete thread", func(t *testing.T) {
		if err := client.DeleteThread(context.Background(), info.ID); err != nil {
			t.Fatalf("failed to delete thread: %v", err)
		}
		if _, err := client.GetThread(context.Background(), info.ID); err == nil {
			t.Fatal("thread was not deleted")
		}
	})
}

func TestClient_AddReplicator(t *testing.T) {
	t.Parallel()
	_, client1, done1 := setup(t)
	defer done1()
	hostAddr2, client2, done2 := setup(t)
	defer done2()

	info := createThread(t, client1)
	hostID2, err := client2.GetHostID(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	t.Run("test add replicator", func(t *testing.T) {
		addr := peerAddr(t, hostAddr2, hostID2)
		pid, err := client1.AddReplicator(context.Background(), info.ID, addr)
		if err != nil {
			t.Fatalf("failed to add replicator: %v", err)
		}
		if pid.String() == "" {
			log.Fatal("got bad ID from add replicator")
		}
	})
}

func TestClient_CreateRecord(t *testing.T) {
	t.Parallel()
	_, client, done := setup(t)
	defer done()

	info := createThread(t, client)
	body, err := cbornode.WrapObject(map[string]interface{}{
		"foo": "bar",
		"baz": []byte("howdy"),
	}, mh.SHA2_256, -1)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("test create record", func(t *testing.T) {
		rec, err := client.CreateRecord(context.Background(), info.ID, body)
		if err != nil {
			t.Fatalf("failed to create record: %v", err)
		}
		if !rec.ThreadID().Equals(info.ID) {
			t.Fatal("got bad thread ID from create record")
		}
		if rec.LogID().String() == "" {
			t.Fatal("got bad log ID from create record")
		}
	})
}

func TestClient_AddRecord(t *testing.T) {
	t.Parallel()
	_, client, done := setup(t)
	defer done()

	// Create a thread, keeping read key and log private key on the client
	id := thread.NewIDV1(thread.Raw, 32)
	tk := thread.NewRandomKey()
	logSk, logPk, err := crypto.GenerateEd25519Key(crand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	identity := createIdentity(t)
	tok, err := client.GetToken(context.Background(), identity)
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.CreateThread(
		context.Background(),
		id,
		core.WithThreadKey(thread.NewServiceKey(tk.Service())),
		core.WithLogKey(logPk),
		core.WithNewThreadToken(tok))
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

	t.Run("test add record", func(t *testing.T) {
		event, err := cbor.CreateEvent(context.Background(), nil, body, sym.New())
		if err != nil {
			t.Fatal(err)
		}
		rec, err := cbor.CreateRecord(context.Background(), nil, cbor.CreateRecordConfig{
			Block:      event,
			Prev:       cid.Undef,
			Key:        logSk,
			PubKey:     identity.GetPublic(),
			ServiceKey: tk.Service(),
		})
		if err != nil {
			t.Fatal(err)
		}
		logID, err := peer.IDFromPublicKey(logPk)
		if err != nil {
			t.Fatal(err)
		}
		if err = client.AddRecord(context.Background(), id, logID, rec); err != nil {
			t.Fatalf("failed to add record: %v", err)
		}

		rec2, err := client.GetRecord(context.Background(), id, rec.Cid())
		if err != nil {
			t.Fatalf("failed to get record back: %v", err)
		}
		if !rec2.Cid().Equals(rec.Cid()) {
			t.Fatal("got back bad record ID")
		}
		if !rec2.BlockID().Equals(rec.BlockID()) {
			t.Fatal("got back bad block ID")
		}
	})
}

func TestClient_GetRecord(t *testing.T) {
	t.Parallel()
	_, client, done := setup(t)
	defer done()

	info := createThread(t, client)
	body, err := cbornode.WrapObject(map[string]interface{}{
		"foo": "bar",
		"baz": []byte("howdy"),
	}, mh.SHA2_256, -1)
	if err != nil {
		t.Fatal(err)
	}
	rec, err := client.CreateRecord(context.Background(), info.ID, body)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("test get record", func(t *testing.T) {
		rec2, err := client.GetRecord(context.Background(), info.ID, rec.Value().Cid())
		if err != nil {
			t.Fatalf("failed to get record: %v", err)
		}
		if !rec2.Cid().Equals(rec.Value().Cid()) {
			t.Fatal("got bad record from get record")
		}
	})
}

func TestClient_Subscribe(t *testing.T) {
	t.Parallel()
	_, client1, done1 := setup(t)
	defer done1()
	hostAddr2, client2, done2 := setup(t)
	defer done2()

	info := createThread(t, client1)
	hostID2, err := client2.GetHostID(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	addr := peerAddr(t, hostAddr2, hostID2)
	if _, err := client1.AddReplicator(context.Background(), info.ID, addr); err != nil {
		t.Fatal(err)
	}

	t.Run("test subscribe", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		sub, err := client2.Subscribe(ctx, core.WithSubFilter(info.ID))
		if err != nil {
			t.Fatalf("failed to subscribe to thread: %v", err)
		}

		var rcount int
		var lock sync.Mutex
		go func() {
			for rec := range sub {
				lock.Lock()
				rcount++
				lock.Unlock()
				t.Logf("got record %s", rec.Value().Cid())
			}
		}()

		body, err := cbornode.WrapObject(map[string]interface{}{"foo": "bar1"}, mh.SHA2_256, -1)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = client1.CreateRecord(context.Background(), info.ID, body); err != nil {
			t.Fatal(err)
		}
		body2, err := cbornode.WrapObject(map[string]interface{}{"foo": "bar2"}, mh.SHA2_256, -1)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = client1.CreateRecord(context.Background(), info.ID, body2); err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Second * 10)

		lock.Lock()
		if rcount != 2 {
			t.Fatalf("expected 2 records got %d", rcount)
		}
		lock.Unlock()
	})
}

func TestClient_Close(t *testing.T) {
	t.Parallel()
	_, addr, shutdown, err := api.CreateTestService("", true)
	if err != nil {
		t.Fatal(err)
	}
	defer shutdown()
	target, err := util.TCPAddrFromMultiAddr(addr)
	if err != nil {
		t.Fatal(err)
	}
	client, err := NewClient(target, grpc.WithInsecure(), grpc.WithPerRPCCredentials(thread.Credentials{}))
	if err != nil {
		t.Fatal(err)
	}

	t.Run("test close", func(t *testing.T) {
		if err := client.Close(); err != nil {
			t.Fatal(err)
		}
	})
}

func setup(t *testing.T) (ma.Multiaddr, *Client, func()) {
	host, addr, shutdown, err := api.CreateTestService("", true)
	if err != nil {
		t.Fatal(err)
	}
	target, err := util.TCPAddrFromMultiAddr(addr)
	if err != nil {
		t.Fatal(err)
	}
	client, err := NewClient(target, grpc.WithInsecure(), grpc.WithPerRPCCredentials(thread.Credentials{}))
	if err != nil {
		t.Fatal(err)
	}

	return host, client, func() {
		shutdown()
		_ = client.Close()
	}
}

func createIdentity(t *testing.T) thread.Identity {
	sk, _, err := crypto.GenerateEd25519Key(crand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return thread.NewLibp2pIdentity(sk)
}

func createThread(t *testing.T, client *Client) thread.Info {
	id := thread.NewIDV1(thread.Raw, 32)
	info, err := client.CreateThread(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return info
}

func threadAddr(t *testing.T, hostAddr ma.Multiaddr, hostID peer.ID, info thread.Info) ma.Multiaddr {
	pa, err := ma.NewMultiaddr("/p2p/" + hostID.String())
	if err != nil {
		t.Fatal(err)
	}
	ta, err := ma.NewMultiaddr("/" + thread.Name + "/" + info.ID.String())
	if err != nil {
		t.Fatal(err)
	}
	return hostAddr.Encapsulate(pa.Encapsulate(ta))
}

func peerAddr(t *testing.T, hostAddr ma.Multiaddr, hostID peer.ID) ma.Multiaddr {
	pa, err := ma.NewMultiaddr("/p2p/" + hostID.String())
	if err != nil {
		t.Fatal(err)
	}
	return hostAddr.Encapsulate(pa)
}
