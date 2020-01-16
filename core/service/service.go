package service

import (
	"context"
	"io"

	"github.com/ipfs/go-cid"
	format "github.com/ipfs/go-ipld-format"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/textileio/go-threads/core/thread"
)

// Service is the network interface for thread orchestration.
type Service interface {
	API

	// DAGService provides a DAG API to the network.
	format.DAGService

	// Host provides a network identity.
	Host() host.Host
}

// API is the network interface for thread orchestration.
type API interface {
	io.Closer

	// GetHostID returns the host's peer ID.
	GetHostID(ctx context.Context) (peer.ID, error)

	// CreateThread with id.
	CreateThread(ctx context.Context, id thread.ID) (thread.Info, error)

	// AddThread from a multiaddress.
	AddThread(ctx context.Context, addr ma.Multiaddr, opts ...AddOption) (thread.Info, error)

	// GetThread with id.
	GetThread(ctx context.Context, id thread.ID) (thread.Info, error)

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
	Subscribe(ctx context.Context, opts ...SubOption) (<-chan ThreadRecord, error)
}
