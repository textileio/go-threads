package eventstore

import (
	"errors"
	"fmt"
	"reflect"
	"sync"

	ds "github.com/ipfs/go-datastore"
	logging "github.com/ipfs/go-log"
	"github.com/textileio/go-textile-threads/core"
)

const (
	idFieldName = "ID"
)

var (
	// ErrInvalidModel indicates that the registered model isn't valid,
	// most probably doesn't have an EntityID.ID field.
	ErrInvalidModel = errors.New("the model is valid")

	log = logging.Logger("store")
)

// Store is the aggregate-root of events and state. External/remote events
// are dispatched to the Store, and are internally processed to impact model
// states. Likewise, local changes in models registered produce events dispatched
// externally.
type Store struct {
	lock       sync.RWMutex
	datastore  ds.Datastore
	dispatcher *Dispatcher
	eventcodec core.EventCodec
	models     map[reflect.Type]*Model
}

// NewStore creates a new Store, which will *own* ds and dispatcher for internal use.
// Saying it differently, ds and dispatcher shouldn't be used externally.
func NewStore(ds ds.Datastore, dispatcher *Dispatcher, ec core.EventCodec) *Store {
	return &Store{
		datastore:  ds,
		dispatcher: dispatcher,
		eventcodec: ec,
		models:     make(map[reflect.Type]*Model),
	}
}

// Register a new model in the store by infering using a defaultInstance
func (s *Store) Register(name string, defaultInstance interface{}) (*Model, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	if s.alreadyRegistered(defaultInstance) {
		return nil, fmt.Errorf("already registered model")
	}

	if !isValidModel(defaultInstance) {
		return nil, ErrInvalidModel
	}

	m := newModel(name, defaultInstance, s.datastore, s.dispatcher, s.eventcodec, s)
	s.models[m.valueType] = m
	s.dispatcher.Register(m)
	return m, nil
}

// Dispatch applies external events to the store. This function guarantee
// no interference with registered model states, and viceversa.
func (s *Store) Dispatch(e core.Event) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.dispatcher.Dispatch(e)
}

func (s *Store) readTxn(m *Model, f func(txn *Txn) error) error {
	s.lock.RLock()
	defer s.lock.RUnlock()

	txn := &Txn{model: m, readonly: true}
	defer txn.Discard()
	if err := f(txn); err != nil {
		return err
	}
	return nil
}

func (s *Store) writeTxn(m *Model, f func(txn *Txn) error) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	txn := &Txn{model: m}
	defer txn.Discard()
	if err := f(txn); err != nil {
		return err
	}
	return txn.Commit()
}

func (s *Store) alreadyRegistered(t interface{}) bool {
	valueType := reflect.TypeOf(t)
	_, ok := s.models[valueType]
	return ok
}

func isValidModel(t interface{}) bool {
	v := reflect.ValueOf(t)
	if v.Type().Kind() != reflect.Ptr {
		v = reflect.New(reflect.TypeOf(v))
	}
	return v.Elem().FieldByName(idFieldName).IsValid()
}
