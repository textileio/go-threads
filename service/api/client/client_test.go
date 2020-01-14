package client

import (
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"testing"
	"time"

	ma "github.com/multiformats/go-multiaddr"
	"github.com/phayes/freeport"
	"github.com/textileio/go-threads/service/api"
	"github.com/textileio/go-threads/store"
	"github.com/textileio/go-threads/util"
	"google.golang.org/grpc"
)

func TestGetHostID(t *testing.T) {
	t.Parallel()
	client, done := setup(t)
	defer done()

	t.Run("test get host ID", func(t *testing.T) {
		if _, err := client.GetHostID(context.Background()); err != nil {
			t.Fatalf("failed to get host ID: %v", err)
		}
	})
}

func TestClose(t *testing.T) {
	t.Parallel()
	addr, shutdown := makeServer(t)
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

func setup(t *testing.T) (*Client, func()) {
	addr, shutdown := makeServer(t)
	target, err := util.TCPAddrFromMultiAddr(addr)
	if err != nil {
		t.Fatal(err)
	}
	client, err := NewClient(target, grpc.WithInsecure())
	if err != nil {
		t.Fatal(err)
	}

	return client, func() {
		shutdown()
		_ = client.Close()
	}
}

func makeServer(t *testing.T) (addr ma.Multiaddr, shutdown func()) {
	time.Sleep(time.Second * time.Duration(rand.Intn(5)))
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}
	ts, err := store.DefaultService(
		dir,
		store.WithServiceDebug(true))
	if err != nil {
		t.Fatal(err)
	}
	ts.Bootstrap(util.DefaultBoostrapPeers())
	port, err := freeport.GetFreePort()
	if err != nil {
		t.Fatal(err)
	}
	apiAddr := util.MustParseAddr(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", port))
	apiProxyAddr := util.MustParseAddr("/ip4/127.0.0.1/tcp/0")
	server, err := api.NewServer(context.Background(), ts, api.Config{
		Addr:      apiAddr,
		ProxyAddr: apiProxyAddr,
		Debug:     true,
	})
	if err != nil {
		t.Fatal(err)
	}
	return apiAddr, func() {
		server.Close()
		ts.Close()
		_ = os.RemoveAll(dir)
	}
}
