package db

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	ds "github.com/ipfs/go-datastore"
	kt "github.com/ipfs/go-datastore/keytransform"
	"github.com/ipfs/go-datastore/query"
	logging "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p-core/crypto"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/textileio/go-threads/core/net"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/go-threads/util"
)

var (
	dsDBManagerBaseKey = ds.NewKey("/manager")
	dsDBAuthor         = ds.NewKey("/author")
)

type Manager struct {
	io.Closer

	config *Config

	network net.Net
	dbs     map[thread.ID]*DB
}

// NewManager hydrates dbs from prefixes and starts them.
func NewManager(network net.Net, opts ...Option) (*Manager, error) {
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
		author, err := m.getDBAuthor(id) // can be nil for open dbs
		if err != nil {
			return nil, err
		}
		creds := credentials{threadID: id, privKey: author}
		s, err := newDB(m.network, creds, getDBConfig(id, m.config))
		if err != nil {
			return nil, err
		}
		m.dbs[id] = s
	}
	return m, nil
}

// NewDB creates a new db and prefixes its datastore with base key.
func (m *Manager) NewDB(ctx context.Context, id thread.ID, author crypto.PrivKey, collections ...CollectionConfig) (*DB, error) {
	if _, ok := m.dbs[id]; ok {
		return nil, fmt.Errorf("db %s already exists", id)
	}
	creds := credentials{threadID: id, privKey: author}
	if _, err := m.network.CreateThread(ctx, creds); err != nil {
		return nil, err
	}
	if err := m.putDBAuthor(id, author); err != nil {
		return nil, err
	}

	db, err := newDB(m.network, creds, getDBConfig(id, m.config, collections...))
	if err != nil {
		return nil, err
	}
	m.dbs[id] = db
	return db, nil
}

// NewDBFromAddr creates a new db from address and prefixes its datastore with base key.
// Unlike NewDB, this method takes a list of collections added to the original db that
// should also be added to this host.
func (m *Manager) NewDBFromAddr(ctx context.Context, addr ma.Multiaddr, author crypto.PrivKey, key thread.Key, collections ...CollectionConfig) (*DB, error) {
	id, err := thread.FromAddr(addr)
	if err != nil {
		return nil, err
	}
	if _, ok := m.dbs[id]; ok {
		return nil, fmt.Errorf("db %s already exists", id)
	}
	creds := credentials{threadID: id, privKey: author}
	if _, err := m.network.AddThread(ctx, creds, addr, net.WithThreadKey(key)); err != nil {
		return nil, err
	}
	if err := m.putDBAuthor(id, author); err != nil {
		return nil, err
	}

	db, err := newDB(m.network, creds, getDBConfig(id, m.config, collections...))
	if err != nil {
		return nil, err
	}
	m.dbs[id] = db

	go func() {
		if err := m.network.PullThread(ctx, creds); err != nil {
			log.Errorf("error pulling thread %s", id)
		}
	}()

	return db, nil
}

// GetDB returns a db by id.
func (m *Manager) GetDB(id thread.ID) *DB {
	return m.dbs[id]
}

// DeleteDB deletes a db by id.
func (m *Manager) DeleteDB(ctx context.Context, id thread.ID) error {
	db := m.dbs[id]
	if db == nil {
		return nil
	}
	author, err := m.getDBAuthor(id)
	if err != nil {
		return err
	}
	creds := credentials{threadID: id, privKey: author}

	if err := db.Close(); err != nil {
		return err
	}
	if err := m.network.DeleteThread(ctx, creds); err != nil {
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

// putDBAuthor persists a db author to the datastore.
func (m *Manager) putDBAuthor(id thread.ID, sk crypto.PrivKey) error {
	skb, err := crypto.MarshalPrivateKey(sk)
	if err != nil {
		return err
	}
	key := dsDBManagerBaseKey.ChildString(id.String()).Child(dsDBAuthor)
	return m.config.Datastore.Put(key, skb)
}

// getDBAuthor loads a db author from the datastore.
func (m *Manager) getDBAuthor(id thread.ID) (crypto.PrivKey, error) {
	key := dsDBManagerBaseKey.ChildString(id.String()).Child(dsDBAuthor)
	skb, err := m.config.Datastore.Get(key)
	if err != nil {
		if errors.Is(err, ds.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return crypto.UnmarshalPrivateKey(skb)
}
