package eventstore

import (
	"os"
	"path/filepath"

	ds "github.com/ipfs/go-datastore"
	badger "github.com/ipfs/go-ds-badger"
	core "github.com/textileio/go-textile-core/store"
	"github.com/textileio/go-threads/jsonpatcher"
)

const (
	defaultDatastorePath = "eventstore"
)

// StoreOption takes a StoreConfig and modifies it
type StoreOption func(*StoreConfig) error

// StoreConfig has configuration parameters for a store
type StoreConfig struct {
	RepoPath   string
	Datastore  ds.TxnDatastore
	EventCodec core.EventCodec
	JsonMode   bool
	Debug      bool
}

func newDefaultEventCodec(jsonMode bool) core.EventCodec {
	return jsonpatcher.New(jsonMode)
}

func newDefaultDatastore(repoPath string) (ds.TxnDatastore, error) {
	path := filepath.Join(repoPath, defaultDatastorePath)
	if err := os.MkdirAll(path, os.ModePerm); err != nil {
		return nil, err
	}
	return badger.NewDatastore(path, &badger.DefaultOptions)
}

func WithJsonMode(enabled bool) StoreOption {
	return func(sc *StoreConfig) error {
		sc.JsonMode = enabled
		return nil
	}
}

func WithRepoPath(path string) StoreOption {
	return func(sc *StoreConfig) error {
		sc.RepoPath = path
		return nil
	}
}

// WithDebug indicate to output debug information
func WithDebug(enable bool) StoreOption {
	return func(sc *StoreConfig) error {
		sc.Debug = enable
		return nil
	}
}

// WithEventCodec configure to use ec as the EventCodec
// manager for transforming actions in events, and viceversa
func WithEventCodec(ec core.EventCodec) StoreOption {
	return func(sc *StoreConfig) error {
		sc.EventCodec = ec
		return nil
	}
}
