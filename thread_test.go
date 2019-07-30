package thread_test

import (
	"context"
	"testing"

	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"

	. "github.com/textileio/go-textile-thread"
)

func TestNew(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	net := mocknet.New(ctx)

	host, err := net.GenPeer()
	if err != nil {
		t.Fatal(err)
	}

	thread, err := New(host)
	if err != nil {
		t.Fatal(err)
	}

	thread.Leave()
}
