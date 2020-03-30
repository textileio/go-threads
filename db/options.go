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

// Option takes a Config and modifies it.
type Option func(*Config) error

// Config has configuration parameters for a db.
type Config struct {
	RepoPath    string
	Datastore   ds.TxnDatastore
	EventCodec  core.EventCodec
	Debug       bool
	LowMem      bool
	Collections []CollectionConfig
	Auth        *thread.Auth
}

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

// WithLowMem specifies whether or not to use low memory settings.
func WithLowMem(low bool) Option {
	return func(sc *Config) error {
		sc.LowMem = low
		return nil
	}
}

// WithRepoPath sets the repo path.
func WithRepoPath(path string) Option {
	return func(sc *Config) error {
		sc.RepoPath = path
		return nil
	}
}

// WithDebug indicate to output debug information.
func WithDebug(enable bool) Option {
	return func(sc *Config) error {
		sc.Debug = enable
		return nil
	}
}

// WithEventCodec configure to use ec as the EventCodec
// for transforming actions in events, and viceversa.
func WithEventCodec(ec core.EventCodec) Option {
	return func(sc *Config) error {
		sc.EventCodec = ec
		return nil
	}
}

// WithDBAuth provides authentication for interacting with a db.
func WithDBAuth(a *thread.Auth) Option {
	return func(sc *Config) error {
		sc.Auth = a
		return nil
	}
}

// WithDBCollections is used to specify collections that
// will be created.
func WithDBCollections(cs ...CollectionConfig) Option {
	return func(sc *Config) error {
		sc.Collections = cs
		return nil
	}
}

// TxnOptions defines options for a transaction.
type TxnOptions struct {
	Auth *thread.Auth
}

// TxnOption specifies a transaction option.
type TxnOption func(*TxnOptions)

// WithTxnAuth provides authentication for the transaction.
func WithTxnAuth(a *thread.Auth) TxnOption {
	return func(args *TxnOptions) {
		args.Auth = a
	}
}

// NewManagedDBOptions defines options for creating a new managed db.
type NewManagedDBOptions struct {
	Collections []CollectionConfig
	Auth        *thread.Auth
}

// NewManagedDBOption specifies a new managed db option.
type NewManagedDBOption func(*NewManagedDBOptions)

// WithNewManagedDBCollections is used to specify collections that
// will be created in a managed db.
func WithNewManagedDBCollections(cs ...CollectionConfig) NewManagedDBOption {
	return func(args *NewManagedDBOptions) {
		args.Collections = cs
	}
}

// WithNewManagedDBAuth provides authentication for interacting with a managed db.
func WithNewManagedDBAuth(a *thread.Auth) NewManagedDBOption {
	return func(args *NewManagedDBOptions) {
		args.Auth = a
	}
}

// ManagedDBOptions defines options for interacting with a managed db.
type ManagedDBOptions struct {
	Auth *thread.Auth
}

// ManagedDBOption specifies a managed db option.
type ManagedDBOption func(*ManagedDBOptions)

// WithManagedDBAuth provides authentication for interacting with a managed db.
func WithManagedDBAuth(a *thread.Auth) ManagedDBOption {
	return func(args *ManagedDBOptions) {
		args.Auth = a
	}
}
