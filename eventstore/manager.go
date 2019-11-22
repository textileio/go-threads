package eventstore

import (
	"io"

	"github.com/google/uuid"
	ds "github.com/ipfs/go-datastore"
	kt "github.com/ipfs/go-datastore/keytransform"
	"github.com/ipfs/go-datastore/query"
	"github.com/textileio/go-textile-core/threadservice"
	"github.com/textileio/go-textile-threads/util"
	logger "github.com/whyrusleeping/go-logging"
)

var (
	dsStoreManagerBaseKey = ds.NewKey("/manager")
)

type Manager struct {
	io.Closer

	config *StoreConfig

	threadservice threadservice.Threadservice
	stores        map[uuid.UUID]*Store
}

// NewManager hydrates stores from prefixes and starts them.
func NewManager(ts threadservice.Threadservice, opts ...StoreOption) (*Manager, error) {
	config := &StoreConfig{}
	for _, opt := range opts {
		if err := opt(config); err != nil {
			return nil, err
		}
	}

	if config.Datastore == nil {
		datastore, err := newDefaultDatastore(config.RepoPath)
		if err != nil {
			return nil, err
		}
		config.Datastore = datastore
	}
	if config.Debug {
		if err := util.SetLogLevels(map[string]logger.Level{"store": logger.DEBUG}); err != nil {
			return nil, err
		}
	}

	m := &Manager{
		config:        config,
		threadservice: ts,
		stores:        make(map[uuid.UUID]*Store),
	}

	results, err := m.config.Datastore.Query(query.Query{
		Prefix:   dsStoreManagerBaseKey.String(),
		KeysOnly: true,
	})
	if err != nil {
		return nil, err
	}
	defer results.Close()
	for res := range results.Next() {
		idStr := ds.RawKey(res.Key).Parent().Parent().Parent().Parent().Name() // reaching for the stars here
		id, err := uuid.Parse(idStr)
		if err != nil {
			continue
		}
		if _, ok := m.stores[id]; ok {
			continue
		}
		s, err := newStore(m.threadservice, getStoreConfig(id, m.config))
		if err != nil {
			return nil, err
		}
		// @todo: Auto-starting reloaded stores could lead to issues (#115)
		if err = s.Start(); err != nil {
			return nil, err
		}
		m.stores[id] = s
	}

	return m, nil
}

// NewStore creates a new store and prefix its datastore with base key.
func (m *Manager) NewStore() (id uuid.UUID, store *Store, err error) {
	id, err = uuid.NewRandom()
	if err != nil {
		return
	}
	store, err = newStore(m.threadservice, getStoreConfig(id, m.config))
	if err != nil {
		return
	}

	m.stores[id] = store
	return id, store, nil
}

// GetStore returns a store by id from the in-mem map.
func (m *Manager) GetStore(id uuid.UUID) *Store {
	return m.stores[id]
}

// Close all the in-mem stores.
func (m *Manager) Close() (err error) {
	for _, s := range m.stores {
		s.Close()
	}
	return m.config.Datastore.Close()
}

// getStoreConfig copies the manager's base config and
// wraps the datastore with an id prefix.
func getStoreConfig(id uuid.UUID, base *StoreConfig) *StoreConfig {
	return &StoreConfig{
		RepoPath: base.RepoPath,
		Datastore: kt.Wrap(base.Datastore, kt.PrefixTransform{
			Prefix: dsStoreManagerBaseKey.ChildString(id.String()),
		}),
		EventCodec: base.EventCodec,
		JsonMode:   base.JsonMode,
		Debug:      base.Debug,
	}
}
