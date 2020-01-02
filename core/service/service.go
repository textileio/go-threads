package service

import (
	"context"
	"io"

	"github.com/ipfs/go-cid"
	format "github.com/ipfs/go-ipld-format"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"
	lstore "github.com/textileio/go-threads/core/logstore"
	"github.com/textileio/go-threads/core/thread"
)

// Service is the network interface for thread orchestration.
type Service interface {
	io.Closer

	// Host provides a network identity.
	Host() host.Host

	// DAGService provides a DAG API to the network.
	format.DAGService

	// Store persists thread details.
	Store() lstore.Logstore

	// AddThread from a multiaddress.
	AddThread(ctx context.Context, addr ma.Multiaddr, opts ...AddOption) (thread.Info, error)

	// PullThread for new records.
	PullThread(ctx context.Context, id thread.ID) error

	// DeleteThread with id.
	DeleteThread(ctx context.Context, id thread.ID) error

	// AddFollower to a thread.
	AddFollower(ctx context.Context, id thread.ID, pid peer.ID) error

	// AddRecord with body.
	AddRecord(ctx context.Context, id thread.ID, body format.Node) (ThreadRecord, error)

	// GetRecord returns the record at cid.
	GetRecord(ctx context.Context, id thread.ID, rid cid.Cid) (Record, error)

	// Subscribe returns a read-only channel of records.
	Subscribe(opts ...SubOption) Subscription
}

// Subscription receives thread record updates.
type Subscription interface {
	// Discard closes the subscription, disabling the reception of further records.
	Discard()

	// Channel returns the channel that receives records.
	Channel() <-chan ThreadRecord
}
