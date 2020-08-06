package net

import (
	"bytes"
	"context"
	"io"

	"github.com/ipfs/go-cid"
	format "github.com/ipfs/go-ipld-format"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/textileio/go-threads/core/thread"
)

// Net wraps API with a DAGService and libp2p host.
type Net interface {
	API

	// DAGService provides a DAG API to the network.
	format.DAGService

	// Host provides a network identity.
	Host() host.Host
}

// API is the network interface for thread orchestration.
type API interface {
	io.Closer

	// GetHostID returns the host's peer id.
	GetHostID(ctx context.Context) (peer.ID, error)

	// GetToken returns a signed token representing an identity.
	GetToken(ctx context.Context, identity thread.Identity) (thread.Token, error)

	// CreateThread with credentials.
	CreateThread(ctx context.Context, id thread.ID, opts ...NewThreadOption) (thread.Info, error)

	// AddThread with credentials from a multiaddress.
	AddThread(ctx context.Context, addr ma.Multiaddr, opts ...NewThreadOption) (thread.Info, error)

	// GetThread with credentials.
	GetThread(ctx context.Context, id thread.ID, opts ...ThreadOption) (thread.Info, error)

	// PullThread for new records.
	// Logs owned by this host are traversed locally.
	// Remotely addressed logs are pulled from the network.
	// Is thread-safe.
	PullThread(ctx context.Context, id thread.ID, opts ...ThreadOption) error

	// DeleteThread with credentials.
	DeleteThread(ctx context.Context, id thread.ID, opts ...ThreadOption) error

	// AddReplicator with credentials.
	// The thread service key and all records will be pushed to paddr.
	AddReplicator(ctx context.Context, id thread.ID, paddr ma.Multiaddr, opts ...ThreadOption) (peer.ID, error)

	// CreateRecord with credentials and body.
	// The resulting record will have an author signature by the thread host.
	// Use AddRecord to add a record from a different author.
	CreateRecord(ctx context.Context, id thread.ID, body format.Node, opts ...ThreadOption) (ThreadRecord, error)

	// AddRecord to the given log.
	AddRecord(ctx context.Context, id thread.ID, lid peer.ID, rec Record, opts ...ThreadOption) error

	// GetRecord returns the record at cid.
	GetRecord(ctx context.Context, id thread.ID, rid cid.Cid, opts ...ThreadOption) (Record, error)

	// Subscribe returns a read-only channel of records.
	Subscribe(ctx context.Context, opts ...SubOption) (<-chan ThreadRecord, error)
}

// Token is used to restrict network APIs to a single app.App.
// In other words, a net token protects against writes and deletes
// which are external to an app.
type Token []byte

// Equal returns whether or not the token is equal to the given value.
func (t Token) Equal(b Token) bool {
	return bytes.Equal(t, b)
}
