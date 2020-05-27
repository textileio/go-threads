package db

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/gob"
	"strconv"
	"sync"

	datastore "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
	core "github.com/textileio/go-threads/core/db"
	"golang.org/x/sync/errgroup"
)

var (
	dsDispatcherPrefix = dsDBPrefix.ChildString("dispatcher")
)

// Reducer applies an event to an existing state.
type Reducer interface {
	Reduce(events []core.Event) error
}

// dispatcher is used to dispatch events to registered reducers.
//
// This is different from generic pub-sub systems because reducers are not subscribed to particular events.
// Every event is dispatched to every registered reducer. When a given reducer is registered, it returns a `token`,
// which can be used to deregister the reducer later.
type dispatcher struct {
	store    datastore.TxnDatastore
	reducers []Reducer
	lock     sync.RWMutex
	lastID   int
}

// NewDispatcher creates a new EventDispatcher.
func newDispatcher(store datastore.TxnDatastore) *dispatcher {
	return &dispatcher{
		store: store,
	}
}

// Store returns the internal event store.
func (d *dispatcher) Store() datastore.Datastore {
	return d.store
}

// Register takes a reducer to be invoked with each dispatched event.
func (d *dispatcher) Register(reducer Reducer) {
	d.lock.Lock()
	defer d.lock.Unlock()
	d.lastID++
	d.reducers = append(d.reducers, reducer)
}

// Dispatch dispatches a payload to all registered reducers.
// The logic is separated in two parts:
// 1. Save all txn events with transaction guarantees.
// 2. Notify all reducers about the known events.
func (d *dispatcher) Dispatch(events []core.Event) error {
	d.lock.Lock()
	defer d.lock.Unlock()

	txn, err := d.store.NewTransaction(false)
	if err != nil {
		return err
	}
	defer txn.Discard()
	for _, event := range events {
		key, err := getKey(event)
		if err != nil {
			return err
		}
		// Encode and add an Event to event store
		b := bytes.Buffer{}
		e := gob.NewEncoder(&b)
		if err := e.Encode(event); err != nil {
			return err
		}
		if err := txn.Put(key, b.Bytes()); err != nil {
			return err
		}
	}
	if err := txn.Commit(); err != nil {
		return err
	}
	// Safe to fire off reducers now that event is persisted
	g, _ := errgroup.WithContext(context.Background())
	for _, reducer := range d.reducers {
		reducer := reducer
		// Launch each reducer in a separate goroutine
		g.Go(func() error {
			return reducer.Reduce(events)
		})
	}
	// Wait for all reducers to complete or error out
	if err := g.Wait(); err != nil {
		return err
	}
	return nil
}

// Query searches the internal event store and returns a query result.
// This is a syncronouse version of github.com/ipfs/go-datastore's Query method.
func (d *dispatcher) Query(query query.Query) ([]query.Entry, error) {
	result, err := d.store.Query(query)
	if err != nil {
		return nil, err
	}
	return result.Rest()
}

// Key format: <timestamp>/<instance-id>/<type>
// @todo: This is up for debate, its a 'fake' Event struct right now anyway
func getKey(event core.Event) (key datastore.Key, err error) {
	buf := bytes.NewBuffer(event.Time())
	var unix int64
	if err = binary.Read(buf, binary.BigEndian, &unix); err != nil {
		return
	}
	time := strconv.FormatInt(unix, 10)
	key = dsDispatcherPrefix.ChildString(time).
		ChildString(event.InstanceID().String()).
		ChildString(event.Collection())
	return key, nil
}
