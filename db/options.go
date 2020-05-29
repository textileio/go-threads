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
	EventCodec  core.EventCodec
	LowMem      bool
	Debug       bool
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

// WithNewCollections is used to specify collections that
// will be created.
func WithNewCollections(cs ...CollectionConfig) NewOption {
	return func(o *NewOptions) {
		o.Collections = cs
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
	return func(args *Options) {
		args.Token = t
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
	return func(args *TxnOptions) {
		args.Token = t
	}
}

// NewManagedOptions defines options for creating a new managed db.
type NewManagedOptions struct {
	Collections []CollectionConfig
	Token       thread.Token
	Name        string
}

// NewManagedOption specifies a new managed db option.
type NewManagedOption func(*NewManagedOptions)

// WithNewManagedCollections is used to specify collections that
// will be created in a managed db.
func WithNewManagedCollections(cs ...CollectionConfig) NewManagedOption {
	return func(args *NewManagedOptions) {
		args.Collections = cs
	}
}

// WithNewManagedToken provides authorization for creating a new managed db.
func WithNewManagedToken(t thread.Token) NewManagedOption {
	return func(args *NewManagedOptions) {
		args.Token = t
	}
}

// WithNewManagedName assigns a name to a new managed db.
func WithNewManagedName(name string) NewManagedOption {
	return func(args *NewManagedOptions) {
		args.Name = name
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
	return func(args *ManagedOptions) {
		args.Token = t
	}
}
