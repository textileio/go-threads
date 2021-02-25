package registry

import (
	"context"
	"crypto/rand"
	"os"
	"testing"
	"time"

	ds "github.com/ipfs/go-datastore"
	logging "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/go-threads/logstore/lstoremem"
)

func TestMain(m *testing.M) {
	_ = logging.SetLogLevel("registry", "debug")
	os.Exit(m.Run())
}

func TestRegistry_Resolve(t *testing.T) {
	r := setupN(t, 3)

	// Resolve invalid thread DID
	//_, err := r[0].Resolve(context.Background(), "did:thread:123")
	//require.Error(t, err)

	// Add a thread to peer 1
	info := thread.Info{
		ID:  thread.NewRandomIDV1(),
		Key: thread.NewRandomKey(),
	}
	err := r[1].store.AddThread(info)
	require.NoError(t, err)

	// Resolve it from peer 0
	doc, err := r[0].Resolve(context.Background(), info.ID.DID())
	require.NoError(t, err)
	assert.Equal(t, info.ID.DID(), doc.ID)
	assert.NotEmpty(t, doc.Services)
}

func setup(t *testing.T) *Registry {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	// Build a libp2p host.
	sk, _, err := crypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	host, err := libp2p.New(ctx, libp2p.Identity(sk))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, host.Close())
	})

	// Create registry.
	r, err := NewRegistry(host, lstoremem.NewLogstore(), ds.NewMapDatastore())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, r.Close())
	})
	return r
}

func setupN(t *testing.T, n int) []*Registry {
	rs := make([]*Registry, n)
	for i := range rs {
		r := setup(t)
		j := 0
		for j < i {
			// Direct connect to the other registries.
			err := r.host.Connect(context.Background(), peer.AddrInfo{
				ID:    rs[j].host.ID(),
				Addrs: rs[j].host.Addrs(),
			})
			require.NoError(t, err)
			j++
		}
		rs[i] = r
	}
	time.Sleep(time.Second)
	return rs
}
