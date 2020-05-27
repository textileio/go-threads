package db

import (
	"context"
	"errors"
	"io"
	"strings"

	ds "github.com/ipfs/go-datastore"
	kt "github.com/ipfs/go-datastore/keytransform"
	"github.com/ipfs/go-datastore/query"
	logging "github.com/ipfs/go-log"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/textileio/go-threads/core/app"
	"github.com/textileio/go-threads/core/net"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/go-threads/util"
)

var (
	// ErrDBNotFound indicates that the specified db doesn't exist in the manager.
	ErrDBNotFound = errors.New("db not found")

	// ErrDBExists indicates that the specified db alrady exists in the manager.
	ErrDBExists = errors.New("db already exists")

	dsDBManagerBaseKey = ds.NewKey("/manager")
)

type Manager struct {
	io.Closer

	newDBOptions *NewOptions

	network app.Net
	dbs     map[thread.ID]*DB
}

// NewManager hydrates and starts dbs from prefixes.
func NewManager(network app.Net, opts ...NewOption) (*Manager, error) {
	options := &NewOptions{}
	for _, opt := range opts {
		if err := opt(options); err != nil {
			return nil, err
		}
	}

	if options.Datastore == nil {
		datastore, err := newDefaultDatastore(options.RepoPath, options.LowMem)
		if err != nil {
			return nil, err
		}
		options.Datastore = datastore
	}
	if options.Debug {
		if err := util.SetLogLevels(map[string]logging.LogLevel{
			"db": logging.LevelDebug,
		}); err != nil {
			return nil, err
		}
	}

	m := &Manager{
		newDBOptions: options,
		network:      network,
		dbs:          make(map[thread.ID]*DB),
	}

	results, err := m.newDBOptions.Datastore.Query(query.Query{
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
		opts, err := getDBOptions(id, m.newDBOptions)
		if err != nil {
			return nil, err
		}
		s, err := newDB(m.network, id, opts)
		if err != nil {
			return nil, err
		}
		m.dbs[id] = s
	}
	return m, nil
}

// GetToken provides access to thread network tokens.
func (m *Manager) GetToken(ctx context.Context, identity thread.Identity) (thread.Token, error) {
	return m.network.GetToken(ctx, identity)
}

// NewDB creates a new db and prefixes its datastore with base key.
func (m *Manager) NewDB(ctx context.Context, id thread.ID, opts ...NewManagedOption) (*DB, error) {
	if _, ok := m.dbs[id]; ok {
		return nil, ErrDBExists
	}
	args := &NewManagedOptions{}
	for _, opt := range opts {
		opt(args)
	}
	if _, err := m.network.CreateThread(ctx, id, net.WithNewThreadToken(args.Token)); err != nil {
		return nil, err
	}

	dbOpts, err := getDBOptions(id, m.newDBOptions, args.Collections...)
	if err != nil {
		return nil, err
	}
	db, err := newDB(m.network, id, dbOpts)
	if err != nil {
		return nil, err
	}
	m.dbs[id] = db
	return db, nil
}

// NewDBFromAddr creates a new db from address and prefixes its datastore with base key.
// Unlike NewDB, this method takes a list of collections added to the original db that
// should also be added to this host.
func (m *Manager) NewDBFromAddr(ctx context.Context, addr ma.Multiaddr, key thread.Key, opts ...NewManagedOption) (*DB, error) {
	id, err := thread.FromAddr(addr)
	if err != nil {
		return nil, err
	}
	if _, ok := m.dbs[id]; ok {
		return nil, ErrDBExists
	}
	args := &NewManagedOptions{}
	for _, opt := range opts {
		opt(args)
	}
	if _, err := m.network.AddThread(ctx, addr, net.WithThreadKey(key), net.WithNewThreadToken(args.Token)); err != nil {
		return nil, err
	}

	dbOpts, err := getDBOptions(id, m.newDBOptions, args.Collections...)
	if err != nil {
		return nil, err
	}
	db, err := newDB(m.network, id, dbOpts)
	if err != nil {
		return nil, err
	}
	m.dbs[id] = db

	go func() {
		if err := m.network.PullThread(ctx, id, net.WithThreadToken(args.Token)); err != nil {
			log.Errorf("error pulling thread %s", id)
		}
	}()

	return db, nil
}

// GetDB returns a db by id.
func (m *Manager) GetDB(ctx context.Context, id thread.ID, opts ...ManagedOption) (*DB, error) {
	args := &ManagedOptions{}
	for _, opt := range opts {
		opt(args)
	}
	if _, err := m.network.GetThread(ctx, id, net.WithThreadToken(args.Token)); err != nil {
		return nil, err
	}
	return m.dbs[id], nil
}

// DeleteDB deletes a db by id.
func (m *Manager) DeleteDB(ctx context.Context, id thread.ID, opts ...ManagedOption) error {
	args := &ManagedOptions{}
	for _, opt := range opts {
		opt(args)
	}
	if _, err := m.network.GetThread(ctx, id, net.WithThreadToken(args.Token)); err != nil {
		return err
	}
	db := m.dbs[id]
	if db == nil {
		return nil
	}

	if err := db.Close(); err != nil {
		return err
	}
	if err := m.network.DeleteThread(ctx, id, net.WithThreadToken(args.Token)); err != nil {
		return err
	}

	// Cleanup keys used by the db
	if err := id.Validate(); err != nil {
		return err
	}
	pre := dsDBManagerBaseKey.ChildString(id.String())
	q := query.Query{Prefix: pre.String(), KeysOnly: true}
	results, err := m.newDBOptions.Datastore.Query(q)
	if err != nil {
		return err
	}
	defer results.Close()
	for result := range results.Next() {
		if err := m.newDBOptions.Datastore.Delete(ds.NewKey(result.Key)); err != nil {
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
	return m.newDBOptions.Datastore.Close()
}

// getDBOptions copies the manager's base config,
// wraps the datastore with an id prefix,
// and merges specified collection configs with those from base
func getDBOptions(id thread.ID, base *NewOptions, collections ...CollectionConfig) (*NewOptions, error) {
	if err := id.Validate(); err != nil {
		return nil, err
	}
	return &NewOptions{
		RepoPath: base.RepoPath,
		Datastore: wrapTxnDatastore(base.Datastore, kt.PrefixTransform{
			Prefix: dsDBManagerBaseKey.ChildString(id.String()),
		}),
		EventCodec:  base.EventCodec,
		Debug:       base.Debug,
		Collections: append(base.Collections, collections...),
	}, nil
}
