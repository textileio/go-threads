package eventstore

import (
	"bytes"
	"encoding/gob"
	"sync"

	"context"

	datastore "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
	"github.com/textileio/go-textile-threads/core"
	"golang.org/x/sync/errgroup"
)

// Reducer applies an event to an existing state.
type Reducer interface {
	Reduce(event core.Event) error
}

// Dispatcher is used to dispatch events to registered reducers.
//
// This is different from generic pub-sub systems because reducers are not subscribed to particular events.
// Every event is dispatched to every registered reducer. When a given reducer is registered, it returns a `token`,
// which can be used to deregister the reducer later.
type Dispatcher struct {
	store    datastore.TxnDatastore
	reducers []Reducer
	lock     sync.RWMutex
	lastID   int
}

// NewDispatcher creates a new EventDispatcher
func NewDispatcher(store datastore.TxnDatastore) *Dispatcher {
	return &Dispatcher{
		store: store,
	}
}

// Store returns the internal event store.
func (d *Dispatcher) Store() datastore.TxnDatastore {
	return d.store
}

// Register takes a reducer to be invoked with each dispatched event
func (d *Dispatcher) Register(reducer Reducer) {
	d.lock.Lock()
	defer d.lock.Unlock()
	d.lastID++
	d.reducers = append(d.reducers, reducer)
}

// Dispatch dispatches a payload to all registered reducers.
func (d *Dispatcher) Dispatch(event core.Event) error {
	d.lock.Lock()
	defer d.lock.Unlock()
	// Key format: <timestamp>/<entity-id>/<type>
	// @todo: This is up for debate, its a 'fake' Event struct right now anyway
	key := datastore.NewKey(string(event.Time())).ChildString(event.EntityID().String()).ChildString(event.Type())
	// Encode and add an Event to event store
	b := bytes.Buffer{}
	e := gob.NewEncoder(&b)
	if err := e.Encode(event); err != nil {
		return err
	}
	if err := d.Store().Put(key, b.Bytes()); err != nil {
		return err
	}
	// Safe to fire off reducers now that event is persisted
	g, _ := errgroup.WithContext(context.Background())
	for _, reducer := range d.reducers {
		// Launch each reducer in a separate goroutine
		g.Go(func() error {
			return reducer.Reduce(event)
		})
	}
	// Wait for all reducers to complete or error out
	if err := g.Wait(); err != nil {
		return err
	}
	return nil
}

// Query searches the internal event store and returns a query result.
// This is a syncronouse version of github.com/ipfs/go-datastore's Query method
func (d *Dispatcher) Query(query query.Query) ([]query.Entry, error) {
	result, err := d.store.Query(query)
	if err != nil {
		return nil, err
	}
	return result.Rest()
}
