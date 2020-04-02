package net

import (
	"context"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-ipld-format"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/textileio/go-threads/core/thread"
)

// Record is the most basic component of a log.
type Record interface {
	format.Node

	// BlockID returns the cid of the inner block.
	BlockID() cid.Cid

	// GetBlock loads the inner block.
	GetBlock(context.Context, format.DAGService) (format.Node, error)

	// PrevID returns the cid of the previous record.
	PrevID() cid.Cid

	// Sig returns a signature from the log key.
	Sig() []byte

	// PubKey of the identity used to author this record.
	PubKey() []byte

	// Verify returns a nil error if the node signature is valid.
	Verify(key crypto.PubKey) error
}

// ThreadRecord wraps Record within a thread and log context.
type ThreadRecord interface {
	// Value returns the underlying record.
	Value() Record

	// ThreadID returns the record's thread ID.
	ThreadID() thread.ID

	// LogID returns the record's log ID.
	LogID() peer.ID
}
