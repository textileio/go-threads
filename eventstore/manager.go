package eventstore

import (
	"context"
	"fmt"
	"io"

	"github.com/google/uuid"
	ds "github.com/ipfs/go-datastore"
	kt "github.com/ipfs/go-datastore/keytransform"
	"github.com/ipfs/go-datastore/query"
)

var (
	dsStoreManagerBaseKey = ds.NewKey("/manager")
)

/*
NewManager: hydrate in-mem stores from prefixes and start them
NewStore: create a new store and prefix its datastore with base key, add to in-mem map
GetStore: return store from in-mem map as it might be started
Close: close all the in-mem stores
*/

type Manager struct {
	io.Closer

	ctx    context.Context
	cancel context.CancelFunc

	config *StoreConfig

	stores map[uuid.UUID]*Store

	ownThreadService bool
}

func NewManager(opts ...StoreOption) (*Manager, error) {
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

	ctx, cancel := context.WithCancel(context.Background())
	ownThreadService := false
	if config.Threadservice == nil {
		ownThreadService = true
		s, err := newDefaultThreadservice(ctx, config.ListenPort, config.RepoPath, config.Debug)
		if err != nil {
			cancel()
			return nil, err
		}
		config.Threadservice = s
	}

	m := &Manager{
		ctx:              ctx,
		cancel:           cancel,
		config:           config,
		stores:           make(map[uuid.UUID]*Store),
		ownThreadService: ownThreadService,
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

func (m *Manager) NewStore() (id uuid.UUID, store *Store, err error) {
	store, err = newStore(m.config)
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

func (m *Manager) GetStore(id uuid.UUID) (*Store, error) {
	prefix := dsStoreManagerBaseKey.ChildString(id.String())
	q, err := m.config.Datastore.Query(query.Query{
		Prefix:   prefix.String(),
		KeysOnly: true,
	})
	if err != nil {
		return nil, err
	}
	defer q.Close()

	if _, ok := q.NextSync(); !ok {
		return nil, nil // not found
	}

	store, err := newStore(m.config)
	if err != nil {
		return nil, err
	}

	store.datastore = kt.Wrap(store.datastore, kt.PrefixTransform{
		Prefix: prefix,
	})

	m.stores[id] = store
	return store, nil
}

func (m *Manager) Close() (err error) {
	m.cancel()
	for _, s := range m.stores {
		s.Close()
	}
	if err = m.config.Datastore.Close(); err != nil {
		return
	}
	if m.ownThreadService {
		m.config.Threadservice.Close()
	}
	return
}
