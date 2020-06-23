// Package db provides a DB which manage collections
package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sync"
	"time"

	"github.com/alecthomas/jsonschema"
	ds "github.com/ipfs/go-datastore"
	kt "github.com/ipfs/go-datastore/keytransform"
	"github.com/ipfs/go-datastore/query"
	format "github.com/ipfs/go-ipld-format"
	logging "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"
	threadcbor "github.com/textileio/go-threads/cbor"
	"github.com/textileio/go-threads/core/app"
	core "github.com/textileio/go-threads/core/db"
	lstore "github.com/textileio/go-threads/core/logstore"
	"github.com/textileio/go-threads/core/net"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/go-threads/util"
)

const (
	idFieldName            = "_id"
	getBlockRetries        = 3
	getBlockInitialTimeout = time.Millisecond * 500
)

var (
	log = logging.Logger("db")

	// ErrInvalidName indicates the provided name isn't valid for a Collection.
	ErrInvalidName = errors.New("name may only contain alphanumeric characters or non-consecutive hyphens, and cannot begin or end with a hyphen")
	// ErrInvalidCollectionSchema indicates the provided schema isn't valid for a Collection.
	ErrInvalidCollectionSchema = errors.New("the collection schema should specify an _id string property")
	// ErrCannotIndexIDField indicates a custom index was specified on the ID field.
	ErrCannotIndexIDField = errors.New("cannot create custom index on " + idFieldName)

	nameRx *regexp.Regexp

	dsPrefix  = ds.NewKey("/db")
	dsName    = dsPrefix.ChildString("name")
	dsSchemas = dsPrefix.ChildString("schema")
	dsIndexes = dsPrefix.ChildString("index")
)

func init() {
	nameRx = regexp.MustCompile(`^[A-Za-z0-9]+(?:[-][A-Za-z0-9]+)*$`)
}

// DB is the aggregate-root of events and state. External/remote events
// are dispatched to the DB, and are internally processed to impact collection
// states. Likewise, local changes in collections registered produce events dispatched
// externally.
type DB struct {
	io.Closer

	name      string
	connector *app.Connector

	datastore  ds.TxnDatastore
	dispatcher *dispatcher
	eventcodec core.EventCodec

	lock        sync.RWMutex
	txnlock     sync.RWMutex
	collections map[string]*Collection
	closed      bool

	localEventsBus      *app.LocalEventsBus
	stateChangedNotifee *stateChangedNotifee
}

// NewDB creates a new DB, which will *own* ds and dispatcher for internal use.
// Saying it differently, ds and dispatcher shouldn't be used externally.
func NewDB(ctx context.Context, network app.Net, id thread.ID, opts ...NewOption) (*DB, error) {
	args := &NewOptions{}
	for _, opt := range opts {
		opt(args)
	}

	if _, err := network.CreateThread(ctx, id, net.WithNewThreadToken(args.Token), net.WithThreadKey(args.ThreadKey)); err != nil {
		if !errors.Is(err, lstore.ErrThreadExists) {
			return nil, err
		}
	}
	return newDB(network, id, args)
}

// NewDBFromAddr creates a new DB from a thread hosted by another peer at address,
// which will *own* ds and dispatcher for internal use.
// Saying it differently, ds and dispatcher shouldn't be used externally.
func NewDBFromAddr(ctx context.Context, network app.Net, addr ma.Multiaddr, key thread.Key, opts ...NewOption) (*DB, error) {
	args := &NewOptions{}
	for _, opt := range opts {
		opt(args)
	}

	ti, err := network.AddThread(ctx, addr, net.WithThreadKey(key), net.WithNewThreadToken(args.Token))
	if err != nil {
		return nil, err
	}
	d, err := newDB(network, ti.ID, args)
	if err != nil {
		return nil, err
	}

	go func() {
		if err := network.PullThread(ctx, ti.ID, net.WithThreadToken(args.Token)); err != nil {
			log.Errorf("error pulling thread %s", ti.ID)
		}
	}()
	return d, nil
}

// newDB is used directly by a db manager to create new dbs with the same config.
func newDB(n app.Net, id thread.ID, opts *NewOptions) (*DB, error) {
	if opts.Datastore == nil {
		datastore, err := newDefaultDatastore(opts.RepoPath, opts.LowMem)
		if err != nil {
			return nil, err
		}
		opts.Datastore = datastore
	}
	if opts.EventCodec == nil {
		opts.EventCodec = newDefaultEventCodec()
	}
	if !managedDatastore(opts.Datastore) {
		if opts.Debug {
			if err := util.SetLogLevels(map[string]logging.LogLevel{
				"db": logging.LevelDebug,
			}); err != nil {
				return nil, err
			}
		}
	}

	d := &DB{
		datastore:           opts.Datastore,
		dispatcher:          newDispatcher(opts.Datastore),
		eventcodec:          opts.EventCodec,
		collections:         make(map[string]*Collection),
		localEventsBus:      app.NewLocalEventsBus(),
		stateChangedNotifee: &stateChangedNotifee{},
	}
	if err := d.putName(opts.Name); err != nil {
		return nil, err
	}
	if err := d.loadName(); err != nil {
		return nil, err
	}
	if err := d.reCreateCollections(); err != nil {
		return nil, err
	}
	d.dispatcher.Register(d)

	for _, cc := range opts.Collections {
		if _, err := d.NewCollection(cc); err != nil {
			return nil, err
		}
	}

	connector, err := n.ConnectApp(d, id)
	if err != nil {
		log.Fatalf("unable to connect app: %s", err)
	}
	d.connector = connector
	return d, nil
}

// managedDatastore returns whether or not the datastore is
// being wrapped by an external datastore.
func managedDatastore(ds ds.Datastore) bool {
	_, ok := ds.(kt.KeyTransform)
	return ok
}

// putName saves a name for db.
func (d *DB) putName(name string) error {
	if name == "" {
		return nil
	}
	if !nameRx.MatchString(name) {
		return ErrInvalidName
	}
	return d.datastore.Put(dsName, []byte(name))
}

// loadName loads db name if present.
func (d *DB) loadName() error {
	bytes, err := d.datastore.Get(dsName)
	if err != nil && !errors.Is(err, ds.ErrNotFound) {
		return err
	}
	if bytes != nil {
		d.name = string(bytes)
	}
	return nil
}

// reCreateCollections loads and registers schemas from the datastore.
func (d *DB) reCreateCollections() error {
	d.lock.Lock()
	defer d.lock.Unlock()

	results, err := d.datastore.Query(query.Query{
		Prefix: dsSchemas.String(),
	})
	if err != nil {
		return err
	}
	defer results.Close()
	for res := range results.Next() {
		name := ds.RawKey(res.Key).Name()
		schema := &jsonschema.Schema{}
		if err := json.Unmarshal(res.Value, schema); err != nil {
			return err
		}
		c, err := newCollection(name, schema, d)
		if err != nil {
			return err
		}
		var indexes map[string]Index
		index, err := d.datastore.Get(dsIndexes.ChildString(name))
		if err == nil && index != nil {
			if err := json.Unmarshal(index, &indexes); err != nil {
				return err
			}
		}
		for _, index := range indexes {
			if index.Path != "" { // Catch bad indexes from an old bug
				c.indexes[index.Path] = index
			}
		}
		d.collections[c.name] = c
	}
	return nil
}

// GetName returns the db name.
func (d *DB) GetName() string {
	return d.name
}

// GetDBInfo returns the addresses and key that can be used to join the DB thread.
func (d *DB) GetDBInfo(opts ...Option) ([]ma.Multiaddr, thread.Key, error) {
	options := &Options{}
	for _, opt := range opts {
		opt(options)
	}

	tinfo, err := d.connector.Net.GetThread(context.Background(), d.connector.ThreadID(), net.WithThreadToken(options.Token))
	if err != nil {
		return nil, thread.Key{}, err
	}
	return tinfo.Addrs, tinfo.Key, nil
}

// CollectionConfig describes a new Collection.
type CollectionConfig struct {
	Name    string
	Schema  *jsonschema.Schema
	Indexes []Index
}

// NewCollection creates a new db collection with config.
// @todo: Handle token auth
func (d *DB) NewCollection(config CollectionConfig, opts ...Option) (*Collection, error) {
	d.lock.Lock()
	defer d.lock.Unlock()

	if _, ok := d.collections[config.Name]; ok {
		return nil, ErrCollectionAlreadyRegistered
	}
	c, err := newCollection(config.Name, config.Schema, d)
	if err != nil {
		return nil, err
	}
	if err := d.addIndexes(c, config.Schema, config.Indexes, opts...); err != nil {
		return nil, err
	}
	if err := d.saveCollection(c); err != nil {
		return nil, err
	}
	return c, nil
}

// UpdateCollection updates an existing db collection with a new config.
// Indexes to new paths will be created.
// Indexes to removed paths will be dropped.
// @todo: Handle token auth
func (d *DB) UpdateCollection(config CollectionConfig, opts ...Option) (*Collection, error) {
	d.lock.Lock()
	defer d.lock.Unlock()

	xc, ok := d.collections[config.Name]
	if !ok {
		return nil, ErrCollectionNotFound
	}
	c, err := newCollection(config.Name, config.Schema, d)
	if err != nil {
		return nil, err
	}
	if err := d.addIndexes(c, config.Schema, config.Indexes, opts...); err != nil {
		return nil, err
	}

	// Drop indexes that are no longer requested
	for _, index := range xc.indexes {
		if _, ok := c.indexes[index.Path]; !ok {
			if err := c.dropIndex(index.Path, opts...); err != nil {
				return nil, err
			}
		}
	}

	if err := d.saveCollection(c); err != nil {
		return nil, err
	}
	return c, nil
}

func (d *DB) addIndexes(c *Collection, schema *jsonschema.Schema, indexes []Index, opts ...Option) error {
	for _, index := range indexes {
		if index.Path == idFieldName {
			return ErrCannotIndexIDField
		}
		if err := c.addIndex(schema, index, opts...); err != nil {
			return err
		}
	}
	return c.addIndex(schema, Index{Path: idFieldName, Unique: true}, opts...)
}

func (d *DB) saveCollection(c *Collection) error {
	schema := c.schemaLoader.JsonSource().([]byte)
	if err := d.datastore.Put(dsSchemas.ChildString(c.name), schema); err != nil {
		return err
	}
	d.collections[c.name] = c
	return nil
}

// GetCollection returns a collection by name.
// @todo: Handle token auth
func (d *DB) GetCollection(name string, opts ...Option) *Collection {
	d.lock.Lock()
	defer d.lock.Unlock()
	options := &Options{}
	for _, opt := range opts {
		opt(options)
	}
	return d.collections[name]
}

// DeleteCollection deletes collection by name and drops all indexes.
// @todo: Handle token auth
func (d *DB) DeleteCollection(name string, opts ...Option) error {
	d.lock.Lock()
	defer d.lock.Unlock()
	options := &Options{}
	for _, opt := range opts {
		opt(options)
	}

	c, ok := d.collections[name]
	if !ok {
		return ErrCollectionNotFound
	}
	txn, err := d.datastore.NewTransaction(false)
	if err != nil {
		return err
	}
	if err := txn.Delete(dsIndexes.ChildString(c.name)); err != nil {
		return err
	}
	if err := txn.Delete(dsSchemas.ChildString(c.name)); err != nil {
		return err
	}
	if err := txn.Commit(); err != nil {
		return err
	}
	delete(d.collections, c.name)
	return nil
}

func (d *DB) Close() error {
	d.lock.Lock()
	defer d.lock.Unlock()
	d.txnlock.Lock()
	defer d.txnlock.Unlock()

	if d.closed {
		return nil
	}
	d.closed = true

	if d.connector != nil {
		if err := d.connector.Close(); err != nil {
			return err
		}
	}
	d.localEventsBus.Discard()
	if !managedDatastore(d.datastore) {
		if err := d.datastore.Close(); err != nil {
			return err
		}
	}
	d.stateChangedNotifee.close()
	return nil
}

func (d *DB) Reduce(events []core.Event) error {
	codecActions, err := d.eventcodec.Reduce(events, d.datastore, baseKey, defaultIndexFunc(d))
	if err != nil {
		return err
	}
	actions := make([]Action, len(codecActions))
	for i, ca := range codecActions {
		var actionType ActionType
		switch codecActions[i].Type {
		case core.Create:
			actionType = ActionCreate
		case core.Save:
			actionType = ActionSave
		case core.Delete:
			actionType = ActionDelete
		default:
			panic("eventcodec action not recognized")
		}
		actions[i] = Action{Collection: ca.Collection, Type: actionType, ID: ca.InstanceID}
	}
	d.notifyStateChanged(actions)
	return nil
}

func defaultIndexFunc(d *DB) func(collection string, key ds.Key, oldData, newData []byte, txn ds.Txn) error {
	return func(collection string, key ds.Key, oldData, newData []byte, txn ds.Txn) error {
		c := d.GetCollection(collection)
		if c == nil {
			return fmt.Errorf("collection (%s) not found", collection)
		}
		if err := c.indexDelete(txn, key, oldData); err != nil {
			return err
		}
		if newData == nil {
			return nil
		}
		return c.indexAdd(txn, key, newData)
	}
}

func (d *DB) HandleNetRecord(rec net.ThreadRecord, key thread.Key, lid peer.ID, timeout time.Duration) error {
	if rec.LogID() == lid {
		return nil // Ignore our own events since DB already dispatches to DB reducers
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	event, err := threadcbor.EventFromRecord(ctx, d.connector.Net, rec.Value())
	if err != nil {
		block, err := d.getBlockWithRetry(ctx, rec.Value())
		if err != nil {
			return fmt.Errorf("error when getting block from record: %v", err)
		}
		event, err = threadcbor.EventFromNode(block)
		if err != nil {
			return fmt.Errorf("error when decoding block to event: %v", err)
		}
	}
	node, err := event.GetBody(ctx, d.connector.Net, key.Read())
	if err != nil {
		return fmt.Errorf("error when getting body of event on thread %s/%s: %v", d.connector.ThreadID(), rec.LogID(), err)
	}
	dbEvents, err := d.eventsFromBytes(node.RawData())
	if err != nil {
		return fmt.Errorf("error when unmarshaling event from bytes: %v", err)
	}
	log.Debugf("dispatching new record: %s/%s", rec.ThreadID(), rec.LogID())
	return d.dispatch(dbEvents)
}

// getBlockWithRetry gets a record block with exponential backoff.
func (d *DB) getBlockWithRetry(ctx context.Context, rec net.Record) (format.Node, error) {
	backoff := getBlockInitialTimeout
	var err error
	for i := 1; i <= getBlockRetries; i++ {
		n, err := rec.GetBlock(ctx, d.connector.Net)
		if err == nil {
			return n, nil
		}
		log.Warnf("error when fetching block %s in retry %d", rec.Cid(), i)
		time.Sleep(backoff)
		backoff *= 2
	}
	return nil, err
}

// dispatch applies external events to the db. This function guarantee
// no interference with registered collection states, and viceversa.
func (d *DB) dispatch(events []core.Event) error {
	d.txnlock.Lock()
	defer d.txnlock.Unlock()
	return d.dispatcher.Dispatch(events)
}

// eventFromBytes generates an Event from its binary representation using
// the underlying EventCodec configured in the DB.
func (d *DB) eventsFromBytes(data []byte) ([]core.Event, error) {
	return d.eventcodec.EventsFromBytes(data)
}

func (d *DB) readTxn(c *Collection, f func(txn *Txn) error, opts ...TxnOption) error {
	d.txnlock.RLock()
	defer d.txnlock.RUnlock()

	args := &TxnOptions{}
	for _, opt := range opts {
		opt(args)
	}
	txn := &Txn{collection: c, token: args.Token, readonly: true}
	defer txn.Discard()
	if err := f(txn); err != nil {
		return err
	}
	return nil
}

func (d *DB) writeTxn(c *Collection, f func(txn *Txn) error, opts ...TxnOption) error {
	d.txnlock.Lock()
	defer d.txnlock.Unlock()

	args := &TxnOptions{}
	for _, opt := range opts {
		opt(args)
	}
	txn := &Txn{collection: c, token: args.Token}
	defer txn.Discard()
	if err := f(txn); err != nil {
		return err
	}
	return txn.Commit()
}
