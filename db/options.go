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
	Identity    thread.Identity
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

// WithDBIdentity provides a signing identity for interacting with a db.
func WithDBIdentity(i thread.Identity) Option {
	return func(c *Config) error {
		c.Identity = i
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
	Identity thread.Identity
	Auth     thread.Auth
}

// TxnOption specifies a transaction option.
type TxnOption func(*TxnOptions)

// WithTxnIdentity provides a signing identity for the transaction.
// This option will overwrite WithTxnAuth.
func WithTxnIdentity(i thread.Identity) TxnOption {
	return func(args *TxnOptions) {
		args.Identity = i
	}
}

// WithTxnAuth provides authorization for the transaction.
func WithTxnAuth(a thread.Auth) TxnOption {
	return func(args *TxnOptions) {
		args.Auth = a
	}
}

// NewManagedDBOptions defines options for creating a new managed db.
type NewManagedDBOptions struct {
	Collections []CollectionConfig
	Identity    thread.Identity
	Auth        thread.Auth
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

// WithNewManagedDBIdentity provides a signing identity for creating a new managed db.
// This option will overwrite WithNewManagedDBAuth.
func WithNewManagedDBIdentity(i thread.Identity) NewManagedDBOption {
	return func(args *NewManagedDBOptions) {
		args.Identity = i
	}
}

// WithNewManagedDBAuth provides authorization for creating a new managed db.
func WithNewManagedDBAuth(a thread.Auth) NewManagedDBOption {
	return func(args *NewManagedDBOptions) {
		args.Auth = a
	}
}

// ManagedDBOptions defines options for interacting with a managed db.
type ManagedDBOptions struct {
	Identity thread.Identity
	Auth     thread.Auth
}

// ManagedDBOption specifies a managed db option.
type ManagedDBOption func(*ManagedDBOptions)

// WithManagedDBIdentity provides a signing identity for interacting with a managed db.
// This option will overwrite WithManagedDBAuth.
func WithManagedDBIdentity(i thread.Identity) ManagedDBOption {
	return func(args *ManagedDBOptions) {
		args.Identity = i
	}
}

// WithManagedDBAuth provides authorization for interacting with a managed db.
func WithManagedDBAuth(a thread.Auth) ManagedDBOption {
	return func(args *ManagedDBOptions) {
		args.Auth = a
	}
}
