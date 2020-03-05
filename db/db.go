// Package db provides a DB which manage models
package db

import (
	"context"
	"encoding/json"
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

	log             = logging.Logger("db")
	dsStorePrefix   = ds.NewKey("/db")
	dsStoreThreadID = dsStorePrefix.ChildString("threadid")
	dsStoreSchemas  = dsStorePrefix.ChildString("schema")
	dsStoreIndexes  = dsStorePrefix.ChildString("index")
)

// DB is the aggregate-root of events and state. External/remote events
// are dispatched to the DB, and are internally processed to impact model
// states. Likewise, local changes in models registered produce events dispatched
// externally.
type DB struct {
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

// NewDB creates a new DB, which will *own* ds and dispatcher for internal use.
// Saying it differently, ds and dispatcher shouldn't be used externally.
func NewDB(ts service.Service, opts ...Option) (*DB, error) {
	config := &Config{}
	for _, opt := range opts {
		if err := opt(config); err != nil {
			return nil, err
		}
	}
	return newDB(ts, config)
}

// newDB is used directly by a db manager to create new dbs
// with the same config.
func newDB(ts service.Service, config *Config) (*DB, error) {
	if config.Datastore == nil {
		datastore, err := newDefaultDatastore(config.RepoPath, config.LowMem)
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
			if err := util.SetLogLevels(map[string]logging.LogLevel{"db": logging.LevelDebug}); err != nil {
				return nil, err
			}
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	s := &DB{
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
func (s *DB) reregisterSchemas() error {
	results, err := s.datastore.Query(query.Query{
		Prefix: dsStoreSchemas.String(),
	})
	if err != nil {
		return err
	}
	defer results.Close()

	for res := range results.Next() {
		name := ds.RawKey(res.Key).Name()
		var indexes []*IndexConfig
		index, err := s.datastore.Get(dsStoreIndexes.ChildString(name))
		if err == nil && index != nil {
			_ = json.Unmarshal(index, &indexes)
		}
		if _, err := s.RegisterSchema(name, string(res.Value), indexes...); err != nil {
			return err
		}
	}
	return nil
}

// ThreadID returns the db's theadID if it exists.
func (s *DB) ThreadID() (thread.ID, bool, error) {
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
// any operation on them. If the db already boostraped on a thread, it will
// continue using that thread. In the opposite case, it will create a new thread.
func (s *DB) Start() error {
	id, found, err := s.ThreadID()
	if err != nil {
		return err
	}
	if !found {
		id = thread.NewIDV1(thread.Raw, 32)
		fk, err := symmetric.CreateKey()
		if err != nil {
			return err
		}
		rk, err := symmetric.CreateKey()
		if err != nil {
			return err
		}
		if _, err := s.service.CreateThread(
			context.Background(),
			id,
			service.FollowKey(fk),
			service.ReadKey(rk)); err != nil {
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
// and before any operation on them. It pulls the current DB thread from
// thread addr
func (s *DB) StartFromAddr(addr ma.Multiaddr, followKey, readKey *symmetric.Key) error {
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

// Service returns the Service used by the db
func (s *DB) Service() service.Service {
	return s.service
}

// Register a new model in the db by infering using a defaultInstance
func (s *DB) Register(name string, defaultInstance interface{}) (*Model, error) {
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

// RegisterSchema a new model in the db with a JSON schema.
func (s *DB) RegisterSchema(name string, schema string, indexes ...*IndexConfig) (*Model, error) {
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

	for _, config := range indexes {
		// @todo: Should check to make sure this is a valid field path for this schema
		_ = m.AddIndex(config.Path, config.Unique)
	}

	indexBytes, err := json.Marshal(indexes)
	if err != nil {
		return nil, err
	}
	indexKey := dsStoreIndexes.ChildString(name)
	exists, err = s.datastore.Has(indexKey)
	if err != nil {
		return nil, err
	}
	if !exists {
		if err := s.datastore.Put(indexKey, indexBytes); err != nil {
			return nil, err
		}
	}

	s.modelNames[name] = m
	return m, nil
}

// GetModel returns a model by name.
func (s *DB) GetModel(name string) *Model {
	return s.modelNames[name]
}

// Reduce processes txn events into the models.
func (s *DB) Reduce(events []core.Event) error {
	codecActions, err := s.eventcodec.Reduce(
		events,
		s.datastore,
		baseKey,
		defaultIndexFunc(s),
	)
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
		actions[i] = Action{Model: ca.Model, Type: actionType, ID: ca.EntityID}
	}
	s.notifyStateChanged(actions)

	return nil
}

// dispatch applies external events to the db. This function guarantee
// no interference with registered model states, and viceversa.
func (s *DB) dispatch(events []core.Event) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.dispatcher.Dispatch(events)
}

// eventFromBytes generates an Event from its binary representation using
// the underlying EventCodec configured in the DB.
func (s *DB) eventsFromBytes(data []byte) ([]core.Event, error) {
	return s.eventcodec.EventsFromBytes(data)
}

func (s *DB) readTxn(m *Model, f func(txn *Txn) error) error {
	s.lock.RLock()
	defer s.lock.RUnlock()

	txn := &Txn{model: m, readonly: true}
	defer txn.Discard()
	if err := f(txn); err != nil {
		return err
	}
	return nil
}

// Close closes the db
func (s *DB) Close() error {
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

func (s *DB) writeTxn(m *Model, f func(txn *Txn) error) error {
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

func defaultIndexFunc(s *DB) func(model string, key ds.Key, oldData, newData []byte, txn ds.Txn) error {
	return func(model string, key ds.Key, oldData, newData []byte, txn ds.Txn) error {
		indexer := s.GetModel(model)
		if err := indexDelete(indexer, txn, key, oldData); err != nil {
			return err
		}
		if newData != nil {
			if err := indexAdd(indexer, txn, key, newData); err != nil {
				return err
			}
		}
		return nil
	}
}
