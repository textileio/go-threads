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
	"github.com/textileio/go-threads/core/service"
	"github.com/textileio/go-threads/core/thread"
	sym "github.com/textileio/go-threads/crypto/symmetric"
	"github.com/textileio/go-threads/util"
)

var (
	dsDBManagerBaseKey = ds.NewKey("/manager")
)

type Manager struct {
	io.Closer

	config *Config

	service service.Service
	dbs     map[thread.ID]*DB
}

// NewManager hydrates dbs from prefixes and starts them.
func NewManager(ts service.Service, opts ...Option) (*Manager, error) {
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
		service: ts,
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
		s, err := newDB(m.service, id, getDBConfig(id, m.config))
		if err != nil {
			return nil, err
		}
		m.dbs[id] = s
	}
	return m, nil
}

// NewDB creates a new db and prefix its datastore with base key.
func (m *Manager) NewDB(ctx context.Context, id thread.ID) (*DB, error) {
	if _, ok := m.dbs[id]; ok {
		return nil, fmt.Errorf("db %s already exists", id.String())
	}
	if _, err := m.service.CreateThread(ctx, id, service.FollowKey(sym.New()), service.ReadKey(sym.New())); err != nil {
		return nil, err
	}

	db, err := newDB(m.service, id, getDBConfig(id, m.config))
	if err != nil {
		return nil, err
	}
	m.dbs[id] = db
	return db, nil
}

func (m *Manager) NewDBFromAddr(ctx context.Context, addr ma.Multiaddr, followKey, readKey *sym.Key) (*DB, error) {
	id, err := thread.FromAddr(addr)
	if err != nil {
		return nil, err
	}
	if _, ok := m.dbs[id]; ok {
		return nil, fmt.Errorf("db %s already exists", id.String())
	}
	if _, err = m.service.AddThread(ctx, addr, service.FollowKey(followKey), service.ReadKey(readKey)); err != nil {
		return nil, err
	}

	db, err := newDB(m.service, id, getDBConfig(id, m.config))
	if err != nil {
		return nil, err
	}
	m.dbs[id] = db

	go func() {
		if err := m.service.PullThread(ctx, id); err != nil {
			log.Errorf("error pulling thread %s", id.String())
		}
	}()

	return db, nil
}

// GetDB returns a db by id from the in-mem map.
func (m *Manager) GetDB(id thread.ID) *DB {
	return m.dbs[id]
}

// Service returns the manager's thread service.
func (m *Manager) Service() service.Service {
	return m.service
}

// Close all the in-mem dbs.
func (m *Manager) Close() error {
	for _, s := range m.dbs {
		if err := s.Close(); err != nil {
			log.Error("error when closing manager datastore: %v", err)
		}
	}
	return m.config.Datastore.Close()
}

// getDBConfig copies the manager's base config and
// wraps the datastore with an id prefix.
func getDBConfig(id thread.ID, base *Config) *Config {
	return &Config{
		RepoPath: base.RepoPath,
		Datastore: wrapTxnDatastore(base.Datastore, kt.PrefixTransform{
			Prefix: dsDBManagerBaseKey.ChildString(id.String()),
		}),
		EventCodec: base.EventCodec,
		JsonMode:   base.JsonMode,
		Debug:      base.Debug,
	}
}
