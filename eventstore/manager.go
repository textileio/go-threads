package eventstore

import (
	"fmt"
	"io"

	"github.com/google/uuid"
	ds "github.com/ipfs/go-datastore"
	kt "github.com/ipfs/go-datastore/keytransform"
	"github.com/ipfs/go-datastore/query"
	"github.com/textileio/go-textile-core/threadservice"
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

// NewManager hydrates stores from prefixes and start them.
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

	// @todo: handle debug here or in store?

	m := &Manager{
		config:        config,
		threadservice: ts,
		stores:        make(map[uuid.UUID]*Store),
	}

	q, err := m.config.Datastore.Query(query.Query{
		Prefix:   dsStoreManagerBaseKey.String(),
		KeysOnly: true,
	})
	if err != nil {
		return nil, err
	}
	defer q.Close()

	for res := range q.Next() {
		id := ds.RawKey(res.Key).Parent().Name()
		fmt.Println(id)

		//s, err := NewStore(m.opts...)
		//if err != nil {
		//	return nil, err
		//}

		//store.datastore = kt.Wrap(store.datastore, kt.PrefixTransform{
		//	Prefix: res.Key,
		//})
		//
		//m.stores[id] = store
	}

	return m, nil
}

// NewStore creates a new store and prefix its datastore with base key.
func (m *Manager) NewStore() (id uuid.UUID, store *Store, err error) {
	store, err = newStore(m.threadservice, m.config)
	if err != nil {
		return
	}

	id, err = uuid.NewRandom()
	if err != nil {
		return
	}
	store.datastore = kt.Wrap(store.datastore, kt.PrefixTransform{
		Prefix: dsStoreManagerBaseKey.ChildString(id.String()),
	})

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
