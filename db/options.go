package db

import (
	"os"
	"path/filepath"

	"github.com/dgraph-io/badger/options"
	ds "github.com/ipfs/go-datastore"
	badger "github.com/ipfs/go-ds-badger"
	core "github.com/textileio/go-threads/core/db"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/go-threads/jsonpatcher"
)

const (
	defaultDatastorePath = "eventstore"
)

func newDefaultEventCodec() core.EventCodec {
	return jsonpatcher.New()
}

func newDefaultDatastore(repoPath string, lowMem bool) (ds.TxnDatastore, error) {
	path := filepath.Join(repoPath, defaultDatastorePath)
	if err := os.MkdirAll(path, os.ModePerm); err != nil {
		return nil, err
	}
	opts := badger.DefaultOptions
	if lowMem {
		opts.TableLoadingMode = options.FileIO
	}
	return badger.NewDatastore(path, &opts)
}

// NewOptions defines options for creating a new db.
type NewOptions struct {
	Name        string
	RepoPath    string
	Token       thread.Token
	Datastore   ds.TxnDatastore
	Collections []CollectionConfig
	Block       bool
	EventCodec  core.EventCodec
	LowMem      bool
	Debug       bool
	ThreadKey   thread.Key
}

// NewOption specifies a new db option.
type NewOption func(*NewOptions)

// WithNewName sets the db name.
func WithNewName(name string) NewOption {
	return func(o *NewOptions) {
		o.Name = name
	}
}

// WithNewRepoPath sets the repo path.
func WithNewRepoPath(path string) NewOption {
	return func(o *NewOptions) {
		o.RepoPath = path
	}
}

// WithNewToken provides authorization for interacting with a db.
func WithNewToken(t thread.Token) NewOption {
	return func(o *NewOptions) {
		o.Token = t
	}
}

// WithNewThreadKey provides control over thread keys to use with a db.
func WithNewThreadKey(key thread.Key) NewOption {
	return func(o *NewOptions) {
		o.ThreadKey = key
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

// WithNewLowMem specifies whether or not to use low memory settings.
func WithNewLowMem(low bool) NewOption {
	return func(o *NewOptions) {
		o.LowMem = low
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
