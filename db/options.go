package db

import (
	"os"
	"path/filepath"

	"github.com/dgraph-io/badger/options"
	ds "github.com/ipfs/go-datastore"
	badger "github.com/ipfs/go-ds-badger"
	core "github.com/textileio/go-threads/core/db"
	"github.com/textileio/go-threads/jsonpatcher"
)

const (
	defaultDatastorePath = "eventstore"
)

// Option takes a Config and modifies it
type Option func(*Config) error

// Config has configuration parameters for a db
type Config struct {
	RepoPath    string
	Datastore   ds.TxnDatastore
	EventCodec  core.EventCodec
	JsonMode    bool
	Debug       bool
	LowMem      bool
	Collections []CollectionConfig
}

func newDefaultEventCodec(jsonMode bool) core.EventCodec {
	return jsonpatcher.New(jsonMode)
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

func WithLowMem(low bool) Option {
	return func(sc *Config) error {
		sc.LowMem = low
		return nil
	}
}

func WithJsonMode(enabled bool) Option {
	return func(sc *Config) error {
		sc.JsonMode = enabled
		return nil
	}
}

func WithRepoPath(path string) Option {
	return func(sc *Config) error {
		sc.RepoPath = path
		return nil
	}
}

// WithDebug indicate to output debug information
func WithDebug(enable bool) Option {
	return func(sc *Config) error {
		sc.Debug = enable
		return nil
	}
}

// WithEventCodec configure to use ec as the EventCodec
// manager for transforming actions in events, and viceversa
func WithEventCodec(ec core.EventCodec) Option {
	return func(sc *Config) error {
		sc.EventCodec = ec
		return nil
	}
}

// WithCollections sets the collections to create a DB with
func WithCollections(collections ...CollectionConfig) Option {
	return func(sc *Config) error {
		sc.Collections = collections
		return nil
	}
}
