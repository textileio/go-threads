package dispatcher

import (
	"errors"
	"sync"

	"github.com/hashicorp/go-multierror"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
	dispatch "github.com/textileio/go-textile-core/dispatcher"
)

// Token is a simple unique ID used to reference a registered callback.
type Token uint

// lastID is the last ID used by the singleton Dispatcher.
var lastID uint

// ErrPersistence means that a write to the underlying event store failed.
var ErrPersistence = errors.New("persistence failure")

// ErrTokenNotFound means the given token was not found in the dispatcher's reducer map.
var ErrTokenNotFound = errors.New("token not found")

// Events are stored under the following db key pattern:
// /events/<event.time><event.entityid><event.type>
// @todo: This is up for debate! It might make more sense to have time at the end of the key.
// @todo: We might also want to include thread, and log information in the key?
var baseKey = datastore.NewKey("/events")

// Dispatcher is used to dispatch events to registered reducers.
//
// This is different from generic pub-sub systems because reducers are not subscribed to particular events.
// Every event is dispatched to every registered reducer. When a given reducer is registered, it returns a `token`,
// which can be used to deregister the reducer later. The dispatcher wraps an underlying event store, which it uses to
// persist events before dispatching to registered reducers. The whole system uses a mutex to prevent additional
// dispatch calls from kicking off reducers until all in-flight reducers complete.
type Dispatcher struct {
	store    datastore.Datastore
	reducers map[Token]dispatch.Reducer
	lock     sync.Mutex
}

// NewDispatcher returns a new Dispatcher. This should only be called once in an application to ensure a singleton
// dispatcher. While it is not enforced here, it is a good idea to a singleton pattern in your own code, for example:
// 	var (
// 		once sync.Once
// 		instance *Dispatcher
// 	)
// 	var store = ...
//
//	func GetDispatcher() *Dispatcher {
// 		once.Do(func() {
// 			singleton = NewDispatcher(store)
// 		})
// 		return singleton
// 	}
func NewDispatcher(store datastore.Datastore) *Dispatcher {
	return &Dispatcher{
		store:    store,
		reducers: make(map[Token]dispatch.Reducer),
	}
}

// Register takes a reducer to be invoked with each dispatched event and returns a token for de-registration.
func (d *Dispatcher) Register(reducer dispatch.Reducer) Token {
	d.lock.Lock()
	defer d.lock.Unlock()
	lastID++
	id := Token(lastID)
	d.reducers[id] = reducer
	return id
}

// Deregister removes a reducer based on its token. If the token is invalid it will return an error.
func (d *Dispatcher) Deregister(token Token) error {
	d.lock.Lock()
	defer d.lock.Unlock()
	if _, ok := d.reducers[token]; !ok {
		return ErrTokenNotFound
	}
	delete(d.reducers, token)
	return nil
}

// Dispatch dispatches a payload to all registered reducers. It returns a multierror object, which may contain
// zero (nil) or more errors. Errors from reducer callbacks can be safely ignored and/or retried, whereas errors due
// to event persistence (`ErrPersistence`) are likely critical, and should be handled by the caller.  If a given
// reducer does fail, it does not affect the other reducers or event store state. It is up to the caller to catch
// this, and rerun if/as needed.
func (d *Dispatcher) Dispatch(event dispatch.Event) error {
	d.lock.Lock()
	defer d.lock.Unlock()
	var result *multierror.Error
	key := baseKey.ChildString(string(event.Time())).ChildString(event.EntityID()).ChildString(event.Type())
	// Add an Event's body to the event store as the value
	if err := d.store.Put(key, event.Body()); err != nil {
		return ErrPersistence
	}
	// Fire off reducers now that event is safely persisted
	wg := sync.WaitGroup{}
	wg.Add(len(d.reducers))
	errChan := make(chan error, len(d.reducers))
	for _, reducer := range d.reducers {
		// Launch each reducer in a separate goroutine and send back errors
		go func(r dispatch.Reducer) {
			defer wg.Done()
			if err := r.Reduce(event); err != nil {
				errChan <- err
			}
		}(reducer)
	}
	wg.Wait()
	// Close and then read from error channel to put into multierror
	close(errChan)
	for err := range errChan {
		result = multierror.Append(result, err)
	}
	if result != nil {
		return result.ErrorOrNil()
	}
	return nil
}

// Query searches the internal event store and returns a query result. This is a proxy to the underlying event store's
// Query method.
func (d *Dispatcher) Query(query query.Query) (query.Results, error) {
	return d.store.Query(query)
}
