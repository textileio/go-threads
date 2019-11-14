// Package eventstore provides a Store which manage models
package eventstore

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"reflect"
	"sync"
	"time"

	ds "github.com/ipfs/go-datastore"
	logging "github.com/ipfs/go-log"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/textileio/go-textile-core/broadcast"
	"github.com/textileio/go-textile-core/crypto/symmetric"
	"github.com/textileio/go-textile-core/options"
	"github.com/textileio/go-textile-core/thread"
	"github.com/textileio/go-textile-core/threadservice"
	ts "github.com/textileio/go-textile-threads"
	"github.com/textileio/go-textile-threads/core"
	"github.com/textileio/go-textile-threads/util"
	logger "github.com/whyrusleeping/go-logging"
	"gopkg.in/natefinch/lumberjack.v2"
)

const (
	idFieldName = "ID"
	busTimeout  = time.Second * 10
)

var (
	// ErrInvalidModel indicates that the registered model isn't valid,
	// most probably doesn't have an EntityID.ID field.
	ErrInvalidModel = errors.New("the model is valid")

	log             = logging.Logger("store")
	dsStorePrefix   = ds.NewKey("/store")
	dsStoreThreadID = dsStorePrefix.ChildString("threadid")
)

// Store is the aggregate-root of events and state. External/remote events
// are dispatched to the Store, and are internally processed to impact model
// states. Likewise, local changes in models registered produce events dispatched
// externally.
type Store struct {
	io.Closer
	ctx            context.Context
	cancel         context.CancelFunc
	lock           sync.RWMutex
	datastore      ds.Datastore
	dispatcher     *dispatcher
	eventcodec     core.EventCodec
	models         map[reflect.Type]*Model
	localEventsBus *localEventsBus
	stateChanged   *stateChangedNotifee

	ownThreadService bool
	threadservice    threadservice.Threadservice
}

// NewStore creates a new Store, which will *own* ds and dispatcher for internal use.
// Saying it differently, ds and dispatcher shouldn't be used externally.
func NewStore(opts ...StoreOption) (*Store, error) {
	config := &StoreConfig{}
	for _, opt := range opts {
		opt(config)
	}
	if config.Datastore == nil {
		datastore, err := newDefaultDatastore(config.RepoPath)
		if err != nil {
			return nil, err
		}
		config.Datastore = datastore
	}
	if config.EventCodec == nil {
		config.EventCodec = newDefaultEventCodec()
	}
	logWriter := &lumberjack.Logger{
		Filename:   filepath.Join(config.RepoPath, defaultRepoPath, "log"),
		MaxSize:    10, // megabytes
		MaxBackups: 3,
		MaxAge:     30, // days
	}
	if config.Debug {
		err := setLogLevels(map[string]logger.Level{
			"store":       logger.DEBUG,
			"threads":     logger.DEBUG,
			"threadstore": logger.DEBUG,
		}, logWriter, true)
		if err != nil {
			return nil, err
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	ownThreadService := false
	if config.Threadservice == nil {
		ownThreadService = true
		ts, err := newDefaultThreadservice(ctx, logWriter, config.ListenPort, config.RepoPath, config.Debug)
		if err != nil {
			cancel()
			return nil, err
		}
		config.Threadservice = ts
	}

	s := &Store{
		ctx:              ctx,
		cancel:           cancel,
		datastore:        config.Datastore,
		dispatcher:       newDispatcher(config.Datastore),
		eventcodec:       config.EventCodec,
		models:           make(map[reflect.Type]*Model),
		localEventsBus:   &localEventsBus{bus: broadcast.NewBroadcaster(0)}, // ToDo: discuss about buffered chan, and implication on eventstore & thread sync tradeoffs
		stateChanged:     &stateChangedNotifee{bus: broadcast.NewBroadcaster(1)},
		threadservice:    config.Threadservice,
		ownThreadService: ownThreadService,
	}
	return s, nil
}

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
		if _, err := util.CreateThread(s.threadservice, id); err != nil {
			return err
		}
		if err := s.datastore.Put(dsStoreThreadID, id.Bytes()); err != nil {
			return err
		}
	}
	adapter := newSingleThreadAdapter(s.ctx, s, id)
	adapter.Start()
	return nil
}

// StartFromAddr should be called immediatelly after registering all schemas
// and before any operation on them. It pulls the current Store thread from
// thread addr
func (s *Store) StartFromAddr(addr ma.Multiaddr, followKey, readKey *symmetric.Key) error {
	idstr, err := addr.ValueForProtocol(ts.ThreadCode)
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
	if _, err = s.threadservice.AddThread(s.ctx, addr, options.FollowKey(followKey), options.ReadKey(readKey)); err != nil {
		return err
	}
	return nil
}

// Threadservice returns the Threadservice used by the store
func (s *Store) Threadservice() threadservice.Threadservice {
	return s.threadservice
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

	m := newModel(name, defaultInstance, s)
	s.models[m.valueType] = m
	s.dispatcher.Register(m)
	return m, nil
}

// StateChangeListen returns a listener which notifies when store state
// changed; some model reduced a new event.
func (s *Store) StateChangeListen() *StateChangeListener {
	return s.stateChanged.Listen()
}

// dispatch applies external events to the store. This function guarantee
// no interference with registered model states, and viceversa.
func (s *Store) dispatch(e core.Event) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.dispatcher.Dispatch(e)
}

// localEventListen returns a listener which notifies *locally generated*
// events in models of the store. Caller should call .Discard() when
// done.
func (s *Store) localEventListen() *LocalEventListener {
	return s.localEventsBus.Listen()
}

// eventFromBytes generates an Event from its binary representation using
// the underlying EventCodec configured in the Store.
func (s *Store) eventFromBytes(data []byte) (core.Event, error) {
	return s.eventcodec.EventFromBytes(data)
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
func (s *Store) Close() {
	s.cancel()
	s.localEventsBus.bus.Discard()
	s.stateChanged.bus.Discard()
	s.datastore.Close()
	if s.ownThreadService {
		s.threadservice.Close()
	}
}

func (s *Store) notifyStateChanged() error {
	return s.stateChanged.bus.SendWithTimeout(struct{}{}, 0)
}

func (s *Store) broadcastLocalEvent(e core.Event) error {
	return s.localEventsBus.bus.SendWithTimeout(e, busTimeout)
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

type localEventsBus struct {
	bus *broadcast.Broadcaster
}

func (br *localEventsBus) Listen() *LocalEventListener {
	l := &LocalEventListener{
		listener: br.bus.Listen(),
		c:        make(chan core.Event),
	}

	go func() {
		for v := range l.listener.Channel() {
			event := v.(core.Event)
			l.c <- event
		}
		close(l.c)
	}()

	return l
}

// LocalEventListener notifies about store-local generated Events
type LocalEventListener struct {
	listener *broadcast.Listener
	c        chan core.Event
}

// Channel returns an unbuffered channel to receive local events
func (l *LocalEventListener) Channel() <-chan core.Event {
	return l.c
}

// Discard indicates that no further events will be received
// and ready for being garbage collected
func (l *LocalEventListener) Discard() {
	l.listener.Discard()
}

type stateChangedNotifee struct {
	bus *broadcast.Broadcaster
}

func (scn *stateChangedNotifee) Listen() *StateChangeListener {
	l := &StateChangeListener{
		listener: scn.bus.Listen(),
		c:        make(chan struct{}),
	}

	go func() {
		for range l.listener.Channel() {

			l.c <- struct{}{}
		}
		close(l.c)
	}()

	return l
}

// StateChangeListener notifies about store changed state
type StateChangeListener struct {
	listener *broadcast.Listener
	c        chan struct{}
}

// Channel returns an unbuffered channel to receive
// store change notifications
func (scl *StateChangeListener) Channel() <-chan struct{} {
	return scl.c
}

// Discard indicates that no further notifications will be received
// and ready for being garbage collected
func (scl *StateChangeListener) Discard() {
	scl.listener.Discard()
}

func setLogLevels(systems map[string]logger.Level, writer io.Writer, color bool) error {
	if writer != nil {
		backendFile := logger.NewLogBackend(writer, "", 0)
		logger.SetBackend(backendFile)
	}

	var form string
	if color {
		form = logging.LogFormats["color"]
	} else {
		form = logging.LogFormats["nocolor"]
	}
	logger.SetFormatter(logger.MustStringFormatter(form))
	logging.SetAllLoggers(logger.ERROR)

	var err error
	for sys, level := range systems {
		if sys == "*" {
			for _, s := range logging.GetSubsystems() {
				err = logging.SetLogLevel(s, level.String())
				if err != nil {
					return err
				}
			}
		}
		err = logging.SetLogLevel(sys, level.String())
		if err != nil {
			return err
		}
	}
	return nil
}
