package db

import (
	"io"

	"github.com/google/uuid"
	ds "github.com/ipfs/go-datastore"
	kt "github.com/ipfs/go-datastore/keytransform"
	"github.com/ipfs/go-datastore/query"
	logging "github.com/ipfs/go-log"
	"github.com/textileio/go-threads/core/service"
	"github.com/textileio/go-threads/util"
)

var (
	dsStoreManagerBaseKey = ds.NewKey("/manager")
)

type Manager struct {
	io.Closer

	config *Config

	service service.Service
	dbs     map[uuid.UUID]*DB
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
		if err := util.SetLogLevels(map[string]logging.LogLevel{"db": logging.LevelDebug}); err != nil {
			return nil, err
		}
	}

	m := &Manager{
		config:  config,
		service: ts,
		dbs:     make(map[uuid.UUID]*DB),
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
		key := ds.RawKey(res.Key)
		if key.BaseNamespace() != "threadid" {
			continue
		}
		id, err := uuid.Parse(key.Parent().Parent().Name())
		if err != nil {
			continue
		}
		if _, ok := m.dbs[id]; ok {
			continue
		}
		s, err := newDB(m.service, getDBConfig(id, m.config))
		if err != nil {
			return nil, err
		}
		// @todo: Auto-starting reloaded dbs could lead to issues (#115)
		if err = s.Start(); err != nil {
			return nil, err
		}
		m.dbs[id] = s
	}

	return m, nil
}

// NewDB creates a new db and prefix its datastore with base key.
func (m *Manager) NewDB() (id uuid.UUID, db *DB, err error) {
	id, err = uuid.NewRandom()
	if err != nil {
		return
	}
	db, err = newDB(m.service, getDBConfig(id, m.config))
	if err != nil {
		return
	}

	m.dbs[id] = db
	return id, db, nil
}

// GetDB returns a db by id from the in-mem map.
func (m *Manager) GetDB(id uuid.UUID) *DB {
	return m.dbs[id]
}

// Close all the in-mem dbs.
func (m *Manager) Close() error {
	var err error
	for _, s := range m.dbs {
		if err = s.Close(); err != nil {
			log.Error("error when closing manager datastore: %v", err)
		}
	}
	err2 := m.config.Datastore.Close()
	if err != nil {
		return err
	}
	return err2
}

// getDBConfig copies the manager's base config and
// wraps the datastore with an id prefix.
func getDBConfig(id uuid.UUID, base *Config) *Config {
	return &Config{
		RepoPath: base.RepoPath,
		Datastore: wrapTxnDatastore(base.Datastore, kt.PrefixTransform{
			Prefix: dsStoreManagerBaseKey.ChildString(id.String()),
		}),
		EventCodec: base.EventCodec,
		JsonMode:   base.JsonMode,
		Debug:      base.Debug,
	}
}
