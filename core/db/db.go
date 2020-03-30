package db

import (
	"io"
	"time"

	"github.com/google/uuid"
	ds "github.com/ipfs/go-datastore"
	format "github.com/ipfs/go-ipld-format"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/textileio/go-threads/broadcast"
	"github.com/textileio/go-threads/core/net"
	"github.com/textileio/go-threads/core/thread"
)

// NetDB is a thread-connectable DB.
type NetDB interface {
	// LocalEventListen returns a listener which notifies *locally generated*
	// events in collections of the db. Caller should call .Discard() when done.
	LocalEventListen() *LocalEventListener
	// HandleNetRecord handles and inbound thread record from net.
	// Records are unpacked and dispatched to all registered reducers.
	HandleNetRecord(rec net.ThreadRecord, info thread.Info, lid peer.ID, timeout time.Duration) error
}

const (
	busTimeout = time.Second * 10
)

// LocalEventsBus wraps a broadcaster for local events.
type LocalEventsBus struct {
	bus *broadcast.Broadcaster
}

// NewLocalEventsBus returns a new bus for local event.
func NewLocalEventsBus() *LocalEventsBus {
	return &LocalEventsBus{bus: broadcast.NewBroadcaster(0)}
}

// Send an IPLD node and thread auth into the bus.
// These are received by the thead connector and written to the underlying thread.
func (leb *LocalEventsBus) Send(node format.Node, auth *thread.Auth) error {
	return leb.bus.SendWithTimeout(&LocalEvent{Node: node, Auth: auth}, busTimeout)
}

// Listen returns a local event listener.
func (leb *LocalEventsBus) Listen() *LocalEventListener {
	l := &LocalEventListener{
		listener: leb.bus.Listen(),
		c:        make(chan *LocalEvent),
	}
	go func() {
		for v := range l.listener.Channel() {
			events := v.(*LocalEvent)
			l.c <- events
		}
		close(l.c)
	}()
	return l
}

func (leb *LocalEventsBus) Discard() {
	leb.bus.Discard()
}

// LocalEvent wraps an IPLD node and auth for delivery to a thread.
type LocalEvent struct {
	Node format.Node
	Auth *thread.Auth
}

// LocalEventListener notifies about new locally generated ipld.Nodes results
// of transactions
type LocalEventListener struct {
	listener *broadcast.Listener
	c        chan *LocalEvent
}

// Channel returns an unbuffered channel to receive local events
func (l *LocalEventListener) Channel() <-chan *LocalEvent {
	return l.c
}

// Discard indicates that no further events will be received
// and ready for being garbage collected
func (l *LocalEventListener) Discard() {
	l.listener.Discard()
}

// Net adds the ability to connect a DB to an underlying thread.
type Net interface {
	net.Net

	// ConnectDB returns a connector that connects a db to a thread.
	ConnectDB(NetDB, thread.ID) Connector
}

// Connector handles the connection between a db and a thread.
type Connector interface {
	io.Closer

	// Start the db connection.
	// Inbound thread records are pushed to the DB.
	// Inbound DB events are persisted to the thread as new records.
	Start()
}

const (
	// EmptyInstanceID represents an empty InstanceID.
	EmptyInstanceID = InstanceID("")
)

// InstanceID is the type used in instance identities.
type InstanceID string

// NewInstanceID generates a new identity for an instance.
func NewInstanceID() InstanceID {
	return InstanceID(uuid.New().String())
}

func (e InstanceID) String() string {
	return string(e)
}

func IsValidInstanceID(instanceID string) bool {
	// ToDo: Decide how we want to apply instance id requirements
	return len(instanceID) > 0
	// _, err := uuid.Parse(instanceID)
	// return err == nil
}

// Event is a local or remote event generated in collection and dispatcher
// by Dispatcher.
type Event interface {
	Time() []byte
	InstanceID() InstanceID
	Collection() string
}

// ActionType is the type used by actions done in a txn.
type ActionType int

const (
	// Create indicates the creation of an instance in a txn.
	Create ActionType = iota
	// Save indicates the mutation of an instance in a txn.
	Save
	// Delete indicates the deletion of an instance by ID in a txn.
	Delete
)

// Action is a operation done in the collection.
type Action struct {
	// Type of the action.
	Type ActionType
	// InstanceID of the instance in action.
	InstanceID InstanceID
	// CollectionName of the instance in action.
	CollectionName string
	// Previous is the instance before the action.
	Previous []byte
	// Current is the instance after the action was done.
	Current []byte
}

type ReduceAction struct {
	// Type of the reduced action.
	Type ActionType
	// Collection in which action was made.
	Collection string
	// InstanceID of the instance in reduced action.
	InstanceID InstanceID
}

// EventCodec transforms actions generated in collections to
// events dispatched to thread logs, and viceversa.
type EventCodec interface {
	// Reduce applies generated events into state.
	Reduce(
		events []Event,
		datastore ds.TxnDatastore,
		baseKey ds.Key,
		indexFunc func(collection string, key ds.Key, oldData, newData []byte, txn ds.Txn) error,
	) ([]ReduceAction, error)
	// Create corresponding events to be dispatched.
	Create(ops []Action) ([]Event, format.Node, error)
	// EventsFromBytes deserializes a format.Node bytes payload into
	// Events.
	EventsFromBytes(data []byte) ([]Event, error)
}
