package db

import (
	"context"
	"fmt"
	"io"
	"strings"

	ds "github.com/ipfs/go-datastore"
	kt "github.com/ipfs/go-datastore/keytransform"
	"github.com/ipfs/go-datastore/query"
	logging "github.com/ipfs/go-log"
	ma "github.com/multiformats/go-multiaddr"
	core "github.com/textileio/go-threads/core/db"
	"github.com/textileio/go-threads/core/net"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/go-threads/util"
)

var (
	dsDBManagerBaseKey = ds.NewKey("/manager")
)

type Manager struct {
	io.Closer

	config *Config

	network core.Net
	dbs     map[thread.ID]*DB
}

// NewManager hydrates dbs from prefixes and starts them.
func NewManager(network core.Net, opts ...Option) (*Manager, error) {
	config := &Config{}
	for _, opt := range opts {
		if err := opt(config); err != nil {
			return nil, err
		}
	}

	if config.Datastore == nil {
		datastore, err := newDefaultDatastore(config.RepoPath, config.LowMem)
		if err != nil {
			return nil, err
		}
		config.Datastore = datastore
	}
	if config.Debug {
		if err := util.SetLogLevels(map[string]logging.LogLevel{
			"db": logging.LevelDebug,
		}); err != nil {
			return nil, err
		}
	}

	m := &Manager{
		config:  config,
		network: network,
		dbs:     make(map[thread.ID]*DB),
	}

	results, err := m.config.Datastore.Query(query.Query{
		Prefix:   dsDBManagerBaseKey.String(),
		KeysOnly: true,
	})
	if err != nil {
		return nil, err
	}
	defer results.Close()
	for res := range results.Next() {
		parts := strings.Split(ds.RawKey(res.Key).String(), "/")
		if len(parts) < 3 {
			continue
		}
		id, err := thread.Decode(parts[2])
		if err != nil {
			continue
		}
		if _, ok := m.dbs[id]; ok {
			continue
		}
		s, err := newDB(m.network, id, getDBConfig(id, m.config))
		if err != nil {
			return nil, err
		}
		m.dbs[id] = s
	}
	return m, nil
}

// NewDB creates a new db and prefixes its datastore with base key.
func (m *Manager) NewDB(ctx context.Context, id thread.ID, opts ...NewManagedDBOption) (*DB, error) {
	if _, ok := m.dbs[id]; ok {
		return nil, fmt.Errorf("db %s already exists", id)
	}
	args := &NewManagedDBOptions{}
	for _, opt := range opts {
		opt(args)
	}
	if _, err := m.network.CreateThread(ctx, id, net.WithNewThreadAuth(args.Auth)); err != nil {
		return nil, err
	}

	db, err := newDB(m.network, id, getDBConfig(id, m.config, args.Collections...))
	if err != nil {
		return nil, err
	}
	m.dbs[id] = db
	return db, nil
}

// NewDBFromAddr creates a new db from address and prefixes its datastore with base key.
// Unlike NewDB, this method takes a list of collections added to the original db that
// should also be added to this host.
func (m *Manager) NewDBFromAddr(ctx context.Context, addr ma.Multiaddr, key thread.Key, opts ...NewManagedDBOption) (*DB, error) {
	id, err := thread.FromAddr(addr)
	if err != nil {
		return nil, err
	}
	if _, ok := m.dbs[id]; ok {
		return nil, fmt.Errorf("db %s already exists", id)
	}
	args := &NewManagedDBOptions{}
	for _, opt := range opts {
		opt(args)
	}
	if _, err := m.network.AddThread(ctx, addr, net.WithThreadKey(key), net.WithNewThreadAuth(args.Auth)); err != nil {
		return nil, err
	}

	db, err := newDB(m.network, id, getDBConfig(id, m.config, args.Collections...))
	if err != nil {
		return nil, err
	}
	m.dbs[id] = db

	go func() {
		if err := m.network.PullThread(ctx, id, net.WithThreadAuth(args.Auth)); err != nil {
			log.Errorf("error pulling thread %s", id)
		}
	}()

	return db, nil
}

// GetDB returns a db by id.
func (m *Manager) GetDB(ctx context.Context, id thread.ID, opts ...ManagedDBOption) (*DB, error) {
	args := &ManagedDBOptions{}
	for _, opt := range opts {
		opt(args)
	}
	if _, err := m.network.GetThread(ctx, id, net.WithThreadAuth(args.Auth)); err != nil {
		return nil, err
	}
	return m.dbs[id], nil
}

// DeleteDB deletes a db by id.
func (m *Manager) DeleteDB(ctx context.Context, id thread.ID, opts ...ManagedDBOption) error {
	args := &ManagedDBOptions{}
	for _, opt := range opts {
		opt(args)
	}
	if _, err := m.network.GetThread(ctx, id, net.WithThreadAuth(args.Auth)); err != nil {
		return err
	}
	db := m.dbs[id]
	if db == nil {
		return nil
	}

	if err := db.Close(); err != nil {
		return err
	}
	if err := m.network.DeleteThread(ctx, id, net.WithThreadAuth(args.Auth)); err != nil {
		return err
	}

	// Cleanup keys used by the db
	pre := dsDBManagerBaseKey.ChildString(id.String())
	q := query.Query{Prefix: pre.String(), KeysOnly: true}
	results, err := m.config.Datastore.Query(q)
	if err != nil {
		return err
	}
	defer results.Close()
	for result := range results.Next() {
		if err := m.config.Datastore.Delete(ds.NewKey(result.Key)); err != nil {
			return err
		}
	}

	delete(m.dbs, id)
	return nil
}

// Net returns the manager's thread network.
func (m *Manager) Net() net.Net {
	return m.network
}

// Close all dbs.
func (m *Manager) Close() error {
	for _, s := range m.dbs {
		if err := s.Close(); err != nil {
			log.Error("error when closing manager datastore: %v", err)
		}
	}
	return m.config.Datastore.Close()
}

// getDBConfig copies the manager's base config,
// wraps the datastore with an id prefix,
// and merges specified collection configs with those from base
func getDBConfig(id thread.ID, base *Config, collections ...CollectionConfig) *Config {
	return &Config{
		RepoPath: base.RepoPath,
		Datastore: wrapTxnDatastore(base.Datastore, kt.PrefixTransform{
			Prefix: dsDBManagerBaseKey.ChildString(id.String()),
		}),
		EventCodec:  base.EventCodec,
		Debug:       base.Debug,
		Collections: append(base.Collections, collections...),
	}
}
