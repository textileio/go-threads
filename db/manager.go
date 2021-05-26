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
	logging "github.com/ipfs/go-log"
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
	dbs     map[thread.ID]*DB
}

// NewManager hydrates and starts dbs from prefixes.
func NewManager(store kt.TxnDatastoreExtended, network app.Net, opts ...NewOption) (*Manager, error) {
	args := &NewOptions{}
	for _, opt := range opts {
		opt(args)
	}

	if args.Debug {
		if err := util.SetLogLevels(map[string]logging.LogLevel{
			"db": logging.LevelDebug,
		}); err != nil {
			return nil, err
		}
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

	log.Info("loading dbs")
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
				log.Infof("loaded %d dbs", i)
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

	log.Infof("finished loading %d dbs", len(m.dbs))
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
	if args.Name != "" && !nameRx.MatchString(args.Name) { // Pre-check name
		return nil, ErrInvalidName
	}

	if args.Key.Defined() && !args.Key.CanRead() {
		return nil, ErrThreadReadKeyRequired
	}
	if _, err := m.network.CreateThread(
		ctx,
		id,
		net.WithThreadKey(args.Key),
		net.WithLogKey(args.LogKey),
		net.WithNewThreadToken(args.Token),
	); err != nil {
		return nil, err
	}

	store, dbOpts, err := wrapDB(m.store, id, m.opts, args.Name, args.Collections...)
	if err != nil {
		return nil, err
	}
	db, err := newDB(store, m.network, id, dbOpts)
	if err != nil {
		return nil, err
	}
	m.dbs[id] = db
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
	if args.Name != "" && !nameRx.MatchString(args.Name) { // Pre-check name
		return nil, ErrInvalidName
	}

	if key.Defined() && !key.CanRead() {
		return nil, ErrThreadReadKeyRequired
	}
	if _, err := m.network.AddThread(
		ctx,
		addr,
		net.WithThreadKey(key),
		net.WithLogKey(args.LogKey),
		net.WithNewThreadToken(args.Token),
	); err != nil {
		return nil, err
	}

	store, dbOpts, err := wrapDB(m.store, id, m.opts, args.Name, args.Collections...)
	if err != nil {
		return nil, err
	}
	db, err := newDB(store, m.network, id, dbOpts)
	if err != nil {
		return nil, err
	}
	m.dbs[id] = db

	if args.Block {
		if err = m.network.PullThread(ctx, id, net.WithThreadToken(args.Token)); err != nil {
			return nil, err
		}
	} else {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), pullThreadBackgroundTimeout)
			defer cancel()
			if err := m.network.PullThread(ctx, id, net.WithThreadToken(args.Token)); err != nil {
				log.Errorf("error pulling thread %s", id)
			}
		}()
	}
	return db, nil
}

// ListDBs returns a list of all dbs.
func (m *Manager) ListDBs(ctx context.Context, opts ...ManagedOption) (map[thread.ID]*DB, error) {
	args := &ManagedOptions{}
	for _, opt := range opts {
		opt(args)
	}

	dbs := make(map[thread.ID]*DB)
	for id, db := range m.dbs {
		if _, err := m.network.GetThread(ctx, id, net.WithThreadToken(args.Token)); err != nil {
			return nil, err
		}
		dbs[id] = db
	}
	return dbs, nil
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
	db, ok := m.dbs[id]
	if !ok {
		return nil, ErrDBNotFound
	}
	return db, nil
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
	db, ok := m.dbs[id]
	if !ok {
		return ErrDBNotFound
	}

	if err := db.Close(); err != nil {
		return err
	}
	if err := m.network.DeleteThread(
		ctx,
		id,
		net.WithThreadToken(args.Token),
		net.WithAPIToken(db.connector.Token()),
	); err != nil {
		return err
	}

	// Cleanup keys used by the db
	if err := id.Validate(); err != nil {
		return err
	}
	if err := m.deleteThreadNamespace(id); err != nil {
		return err
	}

	delete(m.dbs, id)
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
