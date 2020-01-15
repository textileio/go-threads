// Package eventstore provides a Store which manage models
package store

import (
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"
	"sync"
	"time"

	ds "github.com/ipfs/go-datastore"
	kt "github.com/ipfs/go-datastore/keytransform"
	"github.com/ipfs/go-datastore/query"
	logging "github.com/ipfs/go-log"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/textileio/go-threads/broadcast"
	service "github.com/textileio/go-threads/core/service"
	core "github.com/textileio/go-threads/core/store"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/go-threads/crypto/symmetric"
	"github.com/textileio/go-threads/util"
)

const (
	idFieldName = "ID"
	busTimeout  = time.Second * 10
)

var (
	// ErrInvalidModel indicates that the registered model isn't valid,
	// most probably doesn't have an EntityID.ID field.
	ErrInvalidModel = errors.New("the model is invalid")
	// ErrInvalidModelType indicates the provided default type isn't compatible
	// with a Model type.
	ErrInvalidModelType = errors.New("the model type should be a non-nil pointer to a struct")

	log             = logging.Logger("store")
	dsStorePrefix   = ds.NewKey("/store")
	dsStoreThreadID = dsStorePrefix.ChildString("threadid")
	dsStoreSchemas  = dsStorePrefix.ChildString("schema")

	errSavingNonExistentInstance = errors.New("can't save nonexistent instance")
	errUnknownOperation          = errors.New("unknown operation type")
)

// Store is the aggregate-root of events and state. External/remote events
// are dispatched to the Store, and are internally processed to impact model
// states. Likewise, local changes in models registered produce events dispatched
// externally.
type Store struct {
	io.Closer

	ctx    context.Context
	cancel context.CancelFunc

	datastore  ds.TxnDatastore
	dispatcher *dispatcher
	eventcodec core.EventCodec
	service    service.Service
	adapter    *singleThreadAdapter

	lock       sync.RWMutex
	modelNames map[string]*Model
	jsonMode   bool
	closed     bool

	localEventsBus      *localEventsBus
	stateChangedNotifee *stateChangedNotifee
}

// NewStore creates a new Store, which will *own* ds and dispatcher for internal use.
// Saying it differently, ds and dispatcher shouldn't be used externally.
func NewStore(ts service.Service, opts ...Option) (*Store, error) {
	config := &Config{}
	for _, opt := range opts {
		if err := opt(config); err != nil {
			return nil, err
		}
	}
	return newStore(ts, config)
}

// newStore is used directly by a store manager to create new stores
// with the same config.
func newStore(ts service.Service, config *Config) (*Store, error) {
	if config.Datastore == nil {
		datastore, err := newDefaultDatastore(config.RepoPath)
		if err != nil {
			return nil, err
		}
		config.Datastore = datastore
	}
	if config.EventCodec == nil {
		config.EventCodec = newDefaultEventCodec(config.JsonMode)
	}
	if !managedDatastore(config.Datastore) {
		if config.Debug {
			if err := util.SetLogLevels(map[string]logging.LogLevel{"store": logging.LevelDebug}); err != nil {
				return nil, err
			}
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	s := &Store{
		ctx:                 ctx,
		cancel:              cancel,
		datastore:           config.Datastore,
		dispatcher:          newDispatcher(config.Datastore),
		eventcodec:          config.EventCodec,
		modelNames:          make(map[string]*Model),
		jsonMode:            config.JsonMode,
		localEventsBus:      &localEventsBus{bus: broadcast.NewBroadcaster(0)},
		stateChangedNotifee: &stateChangedNotifee{},
		service:             ts,
	}

	if s.jsonMode {
		if err := s.reregisterSchemas(); err != nil {
			return nil, err
		}
	}
	s.dispatcher.Register(s)
	return s, nil
}

// reregisterSchemas loads and registers schemas from the datastore.
func (s *Store) reregisterSchemas() error {
	results, err := s.datastore.Query(query.Query{
		Prefix: dsStoreSchemas.String(),
	})
	if err != nil {
		return err
	}
	defer results.Close()

	for res := range results.Next() {
		name := ds.RawKey(res.Key).Name()
		if _, err = s.RegisterSchema(name, string(res.Value)); err != nil {
			return err
		}
	}
	return nil
}

// ThreadID returns the store's theadID if it exists.
func (s *Store) ThreadID() (thread.ID, bool, error) {
	v, err := s.datastore.Get(dsStoreThreadID)
	if err == ds.ErrNotFound {
		return thread.ID{}, false, nil
	}
	if err != nil {
		return thread.ID{}, false, err
	}
	id, err := thread.Cast(v)
	return id, true, err
}

// Start should be called immediatelly after registering all schemas and before
// any operation on them. If the store already boostraped on a thread, it will
// continue using that thread. In the opposite case, it will create a new thread.
func (s *Store) Start() error {
	id, found, err := s.ThreadID()
	if err != nil {
		return err
	}
	if !found {
		id = thread.NewIDV1(thread.Raw, 32)
		if _, err := util.CreateThread(s.service, id); err != nil {
			return err
		}
		if err := s.datastore.Put(dsStoreThreadID, id.Bytes()); err != nil {
			return err
		}
	}
	adapter := newSingleThreadAdapter(s, id)
	s.adapter = adapter
	adapter.Start()
	return nil
}

// StartFromAddr should be called immediatelly after registering all schemas
// and before any operation on them. It pulls the current Store thread from
// thread addr
func (s *Store) StartFromAddr(addr ma.Multiaddr, followKey, readKey *symmetric.Key) error {
	idstr, err := addr.ValueForProtocol(thread.Code)
	if err != nil {
		return err
	}
	maThreadID, err := thread.Decode(idstr)
	if err != nil {
		return err
	}
	if err := s.datastore.Put(dsStoreThreadID, maThreadID.Bytes()); err != nil {
		return err
	}
	if err = s.Start(); err != nil {
		return err
	}
	if _, err = s.service.AddThread(s.ctx, addr, service.FollowKey(followKey), service.ReadKey(readKey)); err != nil {
		return err
	}
	return nil
}

// Service returns the Service used by the store
func (s *Store) Service() service.Service {
	return s.service
}

// Register a new model in the store by infering using a defaultInstance
func (s *Store) Register(name string, defaultInstance interface{}) (*Model, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	diType := reflect.TypeOf(defaultInstance)
	if reflect.ValueOf(defaultInstance).IsNil() || diType.Kind() != reflect.Ptr || diType.Elem().Kind() != reflect.Struct {
		return nil, ErrInvalidModelType
	}

	if _, ok := s.modelNames[name]; ok {
		return nil, fmt.Errorf("already registered model")
	}

	if !isValidModel(defaultInstance) {
		return nil, ErrInvalidModel
	}

	m := newModel(name, defaultInstance, s)
	s.modelNames[name] = m
	return m, nil
}

// RegisterSchema registers a new model in the store with a JSON schema.
func (s *Store) RegisterSchema(name string, schema string) (*Model, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	if _, ok := s.modelNames[name]; ok {
		return nil, fmt.Errorf("already registered model")
	}

	m := newModelFromSchema(name, schema, s)
	key := dsStoreSchemas.ChildString(name)
	exists, err := s.datastore.Has(key)
	if err != nil {
		return nil, err
	}
	if !exists {
		if err := s.datastore.Put(key, []byte(schema)); err != nil {
			return nil, err
		}
	}
	s.modelNames[name] = m
	return m, nil
}

// GetModel returns a model by name.
func (s *Store) GetModel(name string) *Model {
	return s.modelNames[name]
}

// Reduce processes txn events into the models.
func (s *Store) Reduce(events []core.Event) error {
	txn, err := s.datastore.NewTransaction(false)
	if err != nil {
		return err
	}
	defer txn.Discard()

	actions := make([]Action, len(events))
	for i, e := range events {
		key := baseKey.ChildString(e.Model()).ChildString(e.EntityID().String())
		var actionType ActionType
		var ca *core.CodecResult
		var oldState []byte
		oldState, err := txn.Get(key)
		notFound := errors.Is(err, ds.ErrNotFound)
		if err != nil && !notFound {
			return err
		}
		ca, err = s.eventcodec.Reduce(e, oldState)
		switch ca.Action.Type {
		case core.Create:
			if !notFound {
				return errCantCreateExistingInstance
			}
			actionType = ActionCreate
			if err := txn.Put(key, ca.State); err != nil {
				return fmt.Errorf("error when reducing create event: %v", err)
			}
			log.Debug("\tcreate operation applied")
		case core.Save:
			if notFound {
				return errSavingNonExistentInstance
			}
			actionType = ActionSave
			if err = txn.Put(key, ca.State); err != nil {
				return err
			}
			log.Debug("\tsave operation applied")
		case core.Delete:
			actionType = ActionDelete
			if err := txn.Delete(key); err != nil {
				return err
			}
			log.Debug("\tdelete operation applied")
		default:
			panic("action not recognized")
		}
		actions[i] = Action{Model: ca.Action.Model, Type: actionType, ID: ca.Action.EntityID}
	}
	if err := txn.Commit(); err != nil {
		return err
	}
	s.notifyStateChanged(actions)

	return nil
}

// dispatch applies external events to the store. This function guarantee
// no interference with registered model states, and viceversa.
func (s *Store) dispatch(events []core.Event) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.dispatcher.Dispatch(events)
}

// eventFromBytes generates an Event from its binary representation using
// the underlying EventCodec configured in the Store.
func (s *Store) eventsFromBytes(data []byte) ([]core.Event, error) {
	return s.eventcodec.EventsFromBytes(data)
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

// Close closes the store
func (s *Store) Close() error {
	s.lock.Lock()
	defer s.lock.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true

	if s.adapter != nil {
		s.adapter.Close()
	}
	s.cancel()
	s.localEventsBus.bus.Discard()
	if !managedDatastore(s.datastore) {
		if err := s.datastore.Close(); err != nil {
			return err
		}
	}
	s.stateChangedNotifee.close()
	return nil
}

// managedDatastore returns whether or not the datastore is
// being wrapped by an external datastore.
func managedDatastore(ds ds.Datastore) bool {
	_, ok := ds.(kt.KeyTransform)
	return ok
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

func isValidModel(t interface{}) (valid bool) {
	defer func() {
		if err := recover(); err != nil {
			valid = false
		}
	}()
	v := reflect.ValueOf(t)
	if v.Type().Kind() != reflect.Ptr {
		v = reflect.New(reflect.TypeOf(v))
	}
	return v.Elem().FieldByName(idFieldName).IsValid()
}
