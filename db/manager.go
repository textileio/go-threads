package db

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	ds "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/keytransform"
	"github.com/ipfs/go-datastore/query"
	logging "github.com/ipfs/go-log/v2"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/textileio/go-threads/core/app"
	"github.com/textileio/go-threads/core/net"
	"github.com/textileio/go-threads/core/thread"
	kt "github.com/textileio/go-threads/db/keytransform"
	"github.com/textileio/go-threads/util"
	"golang.org/x/sync/errgroup"
)

var (
	// ErrDBNotFound indicates that the specified db doesn't exist in the manager.
	ErrDBNotFound = errors.New("db not found")
	// ErrDBExists indicates that the specified db alrady exists in the manager.
	ErrDBExists = errors.New("db already exists")

	// MaxLoadConcurrency is the max number of dbs that will be concurrently loaded when the manager starts.
	MaxLoadConcurrency = 100

	dsManagerBaseKey = ds.NewKey("/manager")
)

type Manager struct {
	io.Closer

	opts *NewOptions

	store   kt.TxnDatastoreExtended
	network app.Net

	lk  sync.RWMutex
	dbs map[thread.ID]*DB
}

// NewManager hydrates and starts dbs from prefixes.
func NewManager(store kt.TxnDatastoreExtended, network app.Net, opts ...NewOption) (*Manager, error) {
	args := &NewOptions{}
	for _, opt := range opts {
		opt(args)
	}
	if err := util.SetLogLevels(map[string]logging.LogLevel{
		"db": util.LevelFromDebugFlag(args.Debug),
	}); err != nil {
		return nil, err
	}

	m := &Manager{
		store:   store,
		network: network,
		opts:    args,
		dbs:     make(map[thread.ID]*DB),
	}

	results, err := store.Query(query.Query{
		Prefix:   dsManagerBaseKey.String(),
		KeysOnly: true,
	})
	if err != nil {
		return nil, err
	}
	defer results.Close()

	log.Info("manager: loading dbs")
	eg, gctx := errgroup.WithContext(context.Background())
	loaded := make(map[thread.ID]struct{})
	lim := make(chan struct{}, MaxLoadConcurrency)
	var lk sync.Mutex
	var i int
	for res := range results.Next() {
		if res.Error != nil {
			return nil, err
		}

		lim <- struct{}{}
		res := res
		eg.Go(func() error {
			defer func() { <-lim }()
			if gctx.Err() != nil {
				return nil
			}

			parts := strings.Split(ds.RawKey(res.Key).String(), "/")
			if len(parts) < 3 {
				return nil
			}
			id, err := thread.Decode(parts[2])
			if err != nil {
				return nil
			}
			lk.Lock()
			if _, ok := loaded[id]; ok {
				lk.Unlock()
				return nil
			}
			loaded[id] = struct{}{}
			lk.Unlock()

			s, opts, err := wrapDB(store, id, m.opts, "")
			if err != nil {
				return fmt.Errorf("wrapping db: %v", err)
			}
			d, err := newDB(s, m.network, id, opts)
			if err != nil {
				return fmt.Errorf("unable to reload db %s: %s", id, err)
			}

			lk.Lock()
			m.dbs[id] = d
			i++
			if i%MaxLoadConcurrency == 0 {
				log.Infof("manager: loaded %d dbs", i)
			}
			lk.Unlock()
			return nil
		})
	}
	for i := 0; i < cap(lim); i++ {
		lim <- struct{}{}
	}
	if err := eg.Wait(); err != nil {
		return nil, err
	}

	log.Infof("manager: finished loading %d dbs", len(m.dbs))
	return m, nil
}

// GetToken provides access to thread network tokens.
func (m *Manager) GetToken(ctx context.Context, identity thread.Identity) (thread.Token, error) {
	log.Debug("manager: getting token")
	return m.network.GetToken(ctx, identity)
}

// NewDB creates a new db and prefixes its datastore with base key.
func (m *Manager) NewDB(ctx context.Context, id thread.ID, opts ...NewManagedOption) (*DB, error) {
	log.Debugf("manager: creating new db with id %s", id)
	m.lk.RLock()
	_, ok := m.dbs[id]
	m.lk.RUnlock()
	if ok {
		return nil, ErrDBExists
	}

	args := &NewManagedOptions{}
	for _, opt := range opts {
		opt(args)
	}
	if args.Name != "" && !nameRx.MatchString(args.Name) { // Pre-check name
		return nil, ErrInvalidName
	}

	if args.Key.Defined() && !args.Key.CanRead() {
		return nil, ErrThreadReadKeyRequired
	}
	log.Debugf("manager: creating thread with net %s", id)
	if _, err := m.network.CreateThread(
		ctx,
		id,
		net.WithThreadKey(args.Key),
		net.WithLogKey(args.LogKey),
		net.WithNewThreadToken(args.Token),
	); err != nil {
		return nil, err
	}
	log.Debugf("manager: created thread with net %s", id)

	store, dbOpts, err := wrapDB(m.store, id, m.opts, args.Name, args.Collections...)
	if err != nil {
		return nil, err
	}
	db, err := newDB(store, m.network, id, dbOpts)
	if err != nil {
		return nil, err
	}
	m.lk.Lock()
	m.dbs[id] = db
	m.lk.Unlock()

	log.Debugf("manager: created new db with id %s", id)
	return db, nil
}

// NewDBFromAddr creates a new db from address and prefixes its datastore with base key.
// Unlike NewDB, this method takes a list of collections added to the original db that
// should also be added to this host.
func (m *Manager) NewDBFromAddr(
	ctx context.Context,
	addr ma.Multiaddr,
	key thread.Key,
	opts ...NewManagedOption,
) (*DB, error) {
	log.Debugf("manager: creating new db from address %s", addr)
	id, err := thread.FromAddr(addr)
	if err != nil {
		return nil, err
	}

	m.lk.RLock()
	_, ok := m.dbs[id]
	m.lk.RUnlock()
	if ok {
		return nil, ErrDBExists
	}

	args := &NewManagedOptions{}
	for _, opt := range opts {
		opt(args)
	}
	if args.Name != "" && !nameRx.MatchString(args.Name) { // Pre-check name
		return nil, ErrInvalidName
	}

	if key.Defined() && !key.CanRead() {
		return nil, ErrThreadReadKeyRequired
	}
	log.Debugf("manager: adding thread to net %s", id)
	if _, err := m.network.AddThread(
		ctx,
		addr,
		net.WithThreadKey(key),
		net.WithLogKey(args.LogKey),
		net.WithNewThreadToken(args.Token),
	); err != nil {
		return nil, err
	}
	log.Debugf("manager: added thread to net %s", id)

	store, dbOpts, err := wrapDB(m.store, id, m.opts, args.Name, args.Collections...)
	if err != nil {
		return nil, err
	}
	db, err := newDB(store, m.network, id, dbOpts)
	if err != nil {
		return nil, err
	}
	m.lk.Lock()
	m.dbs[id] = db
	m.lk.Unlock()

	if args.Block {
		log.Debugf("manager: pulling thread %s", id)
		if err = m.network.PullThread(ctx, id, net.WithThreadToken(args.Token)); err != nil {
			return nil, err
		}
		log.Debugf("manager: pulled thread %s", id)
	} else {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), pullThreadBackgroundTimeout)
			defer cancel()
			log.Debugf("manager: pulling thread %s", id)
			if err := m.network.PullThread(ctx, id, net.WithThreadToken(args.Token)); err != nil {
				log.Errorf("error pulling thread %s", id)
			}
			log.Debugf("manager: pulled thread %s", id)
		}()
	}

	log.Debugf("manager: created new db from address %s", addr)
	return db, nil
}

// ListDBs returns a list of all dbs.
func (m *Manager) ListDBs(ctx context.Context, opts ...ManagedOption) (map[thread.ID]*DB, error) {
	log.Debug("manager: listing dbs")
	args := &ManagedOptions{}
	for _, opt := range opts {
		opt(args)
	}

	m.lk.RLock()
	defer m.lk.RUnlock()
	dbs := make(map[thread.ID]*DB)
	for id, db := range m.dbs {
		log.Debugf("manager: getting thread %s from net", id)
		if _, err := m.network.GetThread(ctx, id, net.WithThreadToken(args.Token)); err != nil {
			return nil, err
		}
		log.Debugf("manager: got thread %s from net", id)
		dbs[id] = db
	}

	log.Debug("manager: listed dbs")
	return dbs, nil
}

// GetDB returns a db by id.
func (m *Manager) GetDB(ctx context.Context, id thread.ID, opts ...ManagedOption) (*DB, error) {
	log.Debugf("manager: getting db %s", id)
	args := &ManagedOptions{}
	for _, opt := range opts {
		opt(args)
	}

	log.Debugf("manager: getting thread %s from net", id)
	if _, err := m.network.GetThread(ctx, id, net.WithThreadToken(args.Token)); err != nil {
		return nil, err
	}
	log.Debugf("manager: got thread %s from net", id)

	m.lk.RLock()
	defer m.lk.RUnlock()
	log.Debugf("manager: getting db %s from map", id)
	db, ok := m.dbs[id]
	if !ok {
		return nil, ErrDBNotFound
	}
	return db, nil
}

// DeleteDB deletes a db by id.
func (m *Manager) DeleteDB(ctx context.Context, id thread.ID, opts ...ManagedOption) error {
	log.Debugf("manager: deleting db %s", id)
	args := &ManagedOptions{}
	for _, opt := range opts {
		opt(args)
	}

	log.Debugf("manager: getting thread %s from net", id)
	if _, err := m.network.GetThread(ctx, id, net.WithThreadToken(args.Token)); err != nil {
		return err
	}
	log.Debugf("manager: got thread %s from net", id)

	m.lk.RLock()
	db, ok := m.dbs[id]
	m.lk.RUnlock()
	if !ok {
		return ErrDBNotFound
	}

	if err := db.Close(); err != nil {
		return err
	}
	log.Debugf("manager: deleting thread %s from net", id)
	if err := m.network.DeleteThread(
		ctx,
		id,
		net.WithThreadToken(args.Token),
		net.WithAPIToken(db.connector.Token()),
	); err != nil {
		return err
	}
	log.Debugf("manager: deleted thread %s from net", id)

	// Cleanup keys used by the db
	if err := id.Validate(); err != nil {
		return err
	}
	if err := m.deleteThreadNamespace(id); err != nil {
		return err
	}

	m.lk.Lock()
	delete(m.dbs, id)
	m.lk.Unlock()

	log.Debugf("manager: deleted db %s", id)
	return nil
}

func (m *Manager) deleteThreadNamespace(id thread.ID) error {
	pre := dsManagerBaseKey.ChildString(id.String())
	q := query.Query{Prefix: pre.String(), KeysOnly: true}
	results, err := m.store.Query(q)
	if err != nil {
		return err
	}
	defer results.Close()
	for result := range results.Next() {
		if err := m.store.Delete(ds.NewKey(result.Key)); err != nil {
			return err
		}
	}
	return nil
}

// Net returns the manager's thread network.
func (m *Manager) Net() net.Net {
	return m.network
}

// Close all dbs.
func (m *Manager) Close() error {
	m.lk.Lock()
	defer m.lk.Unlock()
	for _, s := range m.dbs {
		if err := s.Close(); err != nil {
			log.Error("error when closing manager datastore: %v", err)
		}
	}
	return nil
}

// wrapDB copies the manager's base config,
// wraps the datastore with an id prefix,
// and merges specified collection configs with those from base
func wrapDB(
	store kt.TxnDatastoreExtended,
	id thread.ID,
	base *NewOptions,
	name string,
	collections ...CollectionConfig,
) (kt.TxnDatastoreExtended, *NewOptions, error) {
	if err := id.Validate(); err != nil {
		return nil, nil, err
	}
	store = kt.WrapTxnDatastore(store, keytransform.PrefixTransform{
		Prefix: dsManagerBaseKey.ChildString(id.String()),
	})
	opts := &NewOptions{
		Name:        name,
		Collections: append(base.Collections, collections...),
		EventCodec:  base.EventCodec,
		Debug:       base.Debug,
	}
	return store, opts, nil
}
