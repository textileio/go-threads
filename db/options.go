package db

import (
	"github.com/libp2p/go-libp2p-core/crypto"
	core "github.com/textileio/go-threads/core/db"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/go-threads/jsonpatcher"
)

func newDefaultEventCodec() core.EventCodec {
	return jsonpatcher.New()
}

// NewOptions defines options for creating a new db.
type NewOptions struct {
	Name        string
	Key         thread.Key
	LogKey      crypto.Key
	Collections []CollectionConfig
	Block       bool
	EventCodec  core.EventCodec
	Token       thread.Token
	Debug       bool
}

// NewOption specifies a new db option.
type NewOption func(*NewOptions)

// WithNewKey provides control over thread keys to use with a db.
func WithNewKey(key thread.Key) NewOption {
	return func(o *NewOptions) {
		o.Key = key
	}
}

// WithNewLogKey is the public or private key used to write log records.
// If this is just a public key, the service itself won't be able to create records.
// In other words, all records must be pre-created and added with AddRecord.
// If no log key is provided, one will be created internally.
func WithNewLogKey(key crypto.Key) NewOption {
	return func(o *NewOptions) {
		o.LogKey = key
	}
}

// WithNewName sets the db name.
func WithNewName(name string) NewOption {
	return func(o *NewOptions) {
		o.Name = name
	}
}

// WithNewCollections is used to specify collections that
// will be created.
func WithNewCollections(cs ...CollectionConfig) NewOption {
	return func(o *NewOptions) {
		o.Collections = cs
	}
}

// WithNewBackfillBlock makes the caller of NewDBFromAddr block until the
// underlying thread is completely backfilled.
// Without this, NewDBFromAddr returns immediately and thread backfilling
// happens in the background.
func WithNewBackfillBlock(block bool) NewOption {
	return func(o *NewOptions) {
		o.Block = block
	}
}

// WithNewEventCodec configure to use ec as the EventCodec
// for transforming actions in events, and viceversa.
func WithNewEventCodec(ec core.EventCodec) NewOption {
	return func(o *NewOptions) {
		o.EventCodec = ec
	}
}

// WithNewToken provides authorization for interacting with a db.
func WithNewToken(t thread.Token) NewOption {
	return func(o *NewOptions) {
		o.Token = t
	}
}

// WithNewDebug indicate to output debug information.
func WithNewDebug(enable bool) NewOption {
	return func(o *NewOptions) {
		o.Debug = enable
	}
}

// Options defines options for interacting with a db.
type Options struct {
	Token thread.Token
}

// Option specifies a db option.
type Option func(*Options)

// WithToken provides authorization for interacting with a db.
func WithToken(t thread.Token) Option {
	return func(o *Options) {
		o.Token = t
	}
}

// TxnOptions defines options for a transaction.
type TxnOptions struct {
	Token thread.Token
}

// TxnOption specifies a transaction option.
type TxnOption func(*TxnOptions)

// WithTxnToken provides authorization for the transaction.
func WithTxnToken(t thread.Token) TxnOption {
	return func(o *TxnOptions) {
		o.Token = t
	}
}

// NewManagedOptions defines options for creating a new managed db.
type NewManagedOptions struct {
	Name        string
	Key         thread.Key
	LogKey      crypto.Key
	Token       thread.Token
	Collections []CollectionConfig
	Block       bool
}

// NewManagedOption specifies a new managed db option.
type NewManagedOption func(*NewManagedOptions)

// WithNewManagedName assigns a name to a new managed db.
func WithNewManagedName(name string) NewManagedOption {
	return func(o *NewManagedOptions) {
		o.Name = name
	}
}

// WithNewManagedToken provides authorization for creating a new managed db.
func WithNewManagedToken(t thread.Token) NewManagedOption {
	return func(o *NewManagedOptions) {
		o.Token = t
	}
}

// WithNewManagedKey provides control over thread keys to use with a managed db.
func WithNewManagedKey(key thread.Key) NewManagedOption {
	return func(o *NewManagedOptions) {
		o.Key = key
	}
}

// WithNewManagedLogKey is the public or private key used to write log records.
// If this is just a public key, the service itself won't be able to create records.
// In other words, all records must be pre-created and added with AddRecord.
// If no log key is provided, one will be created internally.
func WithNewManagedLogKey(key crypto.Key) NewManagedOption {
	return func(o *NewManagedOptions) {
		o.LogKey = key
	}
}

// WithNewManagedCollections is used to specify collections that
// will be created in a managed db.
func WithNewManagedCollections(cs ...CollectionConfig) NewManagedOption {
	return func(o *NewManagedOptions) {
		o.Collections = cs
	}
}

// WithNewBackfillBlock makes the caller of NewDBFromAddr block until the
// underlying thread is completely backfilled.
// Without this, NewDBFromAddr returns immediately and thread backfilling
// happens in the background.
func WithNewManagedBackfillBlock(block bool) NewManagedOption {
	return func(o *NewManagedOptions) {
		o.Block = block
	}
}

// ManagedOptions defines options for interacting with a managed db.
type ManagedOptions struct {
	Token thread.Token
}

// ManagedOption specifies a managed db option.
type ManagedOption func(*ManagedOptions)

// WithManagedToken provides authorization for interacting with a managed db.
func WithManagedToken(t thread.Token) ManagedOption {
	return func(o *ManagedOptions) {
		o.Token = t
	}
}
