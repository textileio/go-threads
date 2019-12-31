package service

import (
	"context"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-ipld-format"
	ic "github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
)

// Record is the most basic component of a log.
type Record interface {
	format.Node

	// BlockID returns the cid of the node block.
	BlockID() cid.Cid

	// GetBlock loads the node block.
	GetBlock(context.Context, format.DAGService) (format.Node, error)

	// PrevID returns the cid of the previous node.
	PrevID() cid.Cid

	// Sig returns the node signature.
	Sig() []byte

	// Verify returns a non-nil error if the node signature is valid.
	Verify(pk ic.PubKey) error
}

// ThreadRecord wraps Record within a thread and log context.
type ThreadRecord interface {
	// Value returns the underlying record.
	Value() Record

	// ThreadID returns the record's thread ID.
	ThreadID() ID

	// LogID returns the record's log ID.
	LogID() peer.ID
}
