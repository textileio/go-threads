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
	Token       thread.Token
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
	return func(c *Config) error {
		c.LowMem = low
		return nil
	}
}

// WithRepoPath sets the repo path.
func WithRepoPath(path string) Option {
	return func(c *Config) error {
		c.RepoPath = path
		return nil
	}
}

// WithDebug indicate to output debug information.
func WithDebug(enable bool) Option {
	return func(c *Config) error {
		c.Debug = enable
		return nil
	}
}

// WithEventCodec configure to use ec as the EventCodec
// for transforming actions in events, and viceversa.
func WithEventCodec(ec core.EventCodec) Option {
	return func(c *Config) error {
		c.EventCodec = ec
		return nil
	}
}

// WithDBToken provides authorization for interacting with a db.
func WithDBToken(t thread.Token) Option {
	return func(c *Config) error {
		c.Token = t
		return nil
	}
}

// WithDBCollections is used to specify collections that
// will be created.
func WithDBCollections(cs ...CollectionConfig) Option {
	return func(c *Config) error {
		c.Collections = cs
		return nil
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

// NewManagedDBOptions defines options for creating a new managed db.
type NewManagedDBOptions struct {
	Collections []CollectionConfig
	Token       thread.Token
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

// WithNewManagedDBToken provides authorization for creating a new managed db.
func WithNewManagedDBToken(t thread.Token) NewManagedDBOption {
	return func(args *NewManagedDBOptions) {
		args.Token = t
	}
}

// ManagedDBOptions defines options for interacting with a managed db.
type ManagedDBOptions struct {
	Token thread.Token
}

// ManagedDBOption specifies a managed db option.
type ManagedDBOption func(*ManagedDBOptions)

// WithManagedDBToken provides authorization for interacting with a managed db.
func WithManagedDBToken(t thread.Token) ManagedDBOption {
	return func(args *ManagedDBOptions) {
		args.Token = t
	}
}
