package client

import (
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"testing"
	"time"

	core "github.com/textileio/go-threads/core/service"

	"github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/phayes/freeport"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/go-threads/service/api"
	"github.com/textileio/go-threads/store"
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
		if info.ID.String() != id.String() {
			t.Fatal("got bad ID from create thread")
		}
		if info.ReadKey == nil {
			t.Fatal("read key should not be nil")
		}
		if info.FollowKey == nil {
			t.Fatal("follow key should not be nil")
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
		info2, err := client2.AddThread(
			context.Background(),
			addr,
			core.ReadKey(info1.ReadKey),
			core.FollowKey(info1.FollowKey))
		if err != nil {
			t.Fatalf("failed to add thread: %v", err)
		}
		if info2.ID.String() != info1.ID.String() {
			t.Fatal("got bad ID from add thread")
		}
	})
}

//func TestClient_PullThread(t *testing.T) {
//	t.Parallel()
//	client, done := setup(t)
//	defer done()
//
//	t.Run("test pull thread", func(t *testing.T) {
//		if _, err := client.GetHostID(context.Background()); err != nil {
//			t.Fatalf("failed to get host ID: %v", err)
//		}
//	})
//}
//
//func TestClient_DeleteThread(t *testing.T) {
//	t.Parallel()
//	client, done := setup(t)
//	defer done()
//
//	t.Run("test delete thread", func(t *testing.T) {
//		if _, err := client.GetHostID(context.Background()); err != nil {
//			t.Fatalf("failed to get host ID: %v", err)
//		}
//	})
//}
//
//func TestClient_AddFollower(t *testing.T) {
//	t.Parallel()
//	client, done := setup(t)
//	defer done()
//
//	t.Run("test add follower", func(t *testing.T) {
//		if _, err := client.GetHostID(context.Background()); err != nil {
//			t.Fatalf("failed to get host ID: %v", err)
//		}
//	})
//}
//
//func TestClient_AddRecord(t *testing.T) {
//	t.Parallel()
//	client, done := setup(t)
//	defer done()
//
//	t.Run("test add record", func(t *testing.T) {
//		if _, err := client.GetHostID(context.Background()); err != nil {
//			t.Fatalf("failed to get host ID: %v", err)
//		}
//	})
//}
//
//func TestClient_GetRecord(t *testing.T) {
//	t.Parallel()
//	client, done := setup(t)
//	defer done()
//
//	t.Run("test get record", func(t *testing.T) {
//		if _, err := client.GetHostID(context.Background()); err != nil {
//			t.Fatalf("failed to get host ID: %v", err)
//		}
//	})
//}
//
//func TestClient_Subscribe(t *testing.T) {
//	t.Parallel()
//	client, done := setup(t)
//	defer done()
//
//	t.Run("test subscribe", func(t *testing.T) {
//		if _, err := client.GetHostID(context.Background()); err != nil {
//			t.Fatalf("failed to get host ID: %v", err)
//		}
//	})
//}

func TestClient_Close(t *testing.T) {
	t.Parallel()
	_, addr, shutdown := makeServer(t)
	defer shutdown()
	target, err := util.TCPAddrFromMultiAddr(addr)
	if err != nil {
		t.Fatal(err)
	}
	client, err := NewClient(target, grpc.WithInsecure())
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
	host, addr, shutdown := makeServer(t)
	target, err := util.TCPAddrFromMultiAddr(addr)
	if err != nil {
		t.Fatal(err)
	}
	client, err := NewClient(target, grpc.WithInsecure())
	if err != nil {
		t.Fatal(err)
	}

	return host, client, func() {
		shutdown()
		_ = client.Close()
	}
}

func makeServer(t *testing.T) (ma.Multiaddr, ma.Multiaddr, func()) {
	time.Sleep(time.Second * time.Duration(rand.Intn(5)))
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}
	hostPort, err := freeport.GetFreePort()
	if err != nil {
		t.Fatal(err)
	}
	hostAddr := util.MustParseAddr(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", hostPort))
	ts, err := store.DefaultService(
		dir,
		store.WithServiceHostAddr(hostAddr),
		store.WithServiceDebug(true))
	if err != nil {
		t.Fatal(err)
	}
	ts.Bootstrap(util.DefaultBoostrapPeers())
	apiPort, err := freeport.GetFreePort()
	if err != nil {
		t.Fatal(err)
	}
	apiAddr := util.MustParseAddr(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", apiPort))
	apiProxyAddr := util.MustParseAddr("/ip4/127.0.0.1/tcp/0")
	server, err := api.NewServer(context.Background(), ts, api.Config{
		Addr:      apiAddr,
		ProxyAddr: apiProxyAddr,
		Debug:     true,
	})
	if err != nil {
		t.Fatal(err)
	}
	return hostAddr, apiAddr, func() {
		server.Close()
		ts.Close()
		_ = os.RemoveAll(dir)
	}
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
