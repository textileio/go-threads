package net

import (
	"bytes"
	"context"
	"io"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-ipld-format"
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

	// GetToken returns a signed token representing an identity that can be used with other API methods, e.g.,
	// CreateThread, AddThread, etc.
	GetToken(ctx context.Context, identity thread.Identity) (thread.Token, error)

	// CreateThread creates and adds a new thread with id and opts.
	CreateThread(ctx context.Context, id thread.ID, opts ...NewThreadOption) (thread.Info, error)

	// AddThread adds an existing thread from a multiaddress and opts.
	AddThread(ctx context.Context, addr ma.Multiaddr, opts ...NewThreadOption) (thread.Info, error)

	// GetThread returns thread info by id.
	GetThread(ctx context.Context, id thread.ID, opts ...ThreadOption) (thread.Info, error)

	// PullThread requests new records from each known thread host.
	// This method is called internally on an interval as part of the orchestration protocol.
	// Calling it manually can be useful when new records are known to be available.
	PullThread(ctx context.Context, id thread.ID, opts ...ThreadOption) error

	// DeleteThread removes a thread by id and opts.
	DeleteThread(ctx context.Context, id thread.ID, opts ...ThreadOption) error

	// AddReplicator replicates a thread by id on a different host.
	// All logs and records are pushed to the new host.
	AddReplicator(ctx context.Context, id thread.ID, paddr ma.Multiaddr, opts ...ThreadOption) (peer.ID, error)

	// CreateRecord creates and adds a new record with body to a thread by id.
	CreateRecord(ctx context.Context, id thread.ID, body format.Node, opts ...ThreadOption) (ThreadRecord, error)

	// AddRecord add an existing record to a thread by id and lid.
	AddRecord(ctx context.Context, id thread.ID, lid peer.ID, rec Record, opts ...ThreadOption) error

	// GetRecord returns a record by thread id and cid.
	GetRecord(ctx context.Context, id thread.ID, rid cid.Cid, opts ...ThreadOption) (Record, error)

	// Subscribe returns a read-only channel that receives newly created / added thread records.
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
