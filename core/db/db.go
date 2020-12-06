package db

import (
	"crypto/rand"
	"strings"

	ds "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-ipld-format"
	ulid "github.com/oklog/ulid/v2"
)

const (
	// EmptyInstanceID represents an empty InstanceID.
	EmptyInstanceID = InstanceID("")
)

// InstanceID is the type used in instance identities.
type InstanceID string

// NewInstanceID generates a new identity for an instance.
func NewInstanceID() InstanceID {
	id := ulid.MustNew(ulid.Now(), rand.Reader)
	return InstanceID(strings.ToLower(id.String()))
}

func (e InstanceID) String() string {
	return string(e)
}

// Event is a local or remote event generated in collection and dispatcher
// by Dispatcher.
type Event interface {
	// Time (wall-clock) the event was created.
	Time() []byte
	// InstanceID is the associated instance's unique identifier.
	InstanceID() InstanceID
	// Collection is the associated instance's collection name.
	Collection() string
	// Marshal the event to JSON.
	Marshal() ([]byte, error)
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

// IndexFunc handles index updates.
type IndexFunc func(collection string, key ds.Key, oldData, newData []byte, txn ds.Txn) error

// EventCodec transforms actions generated in collections to
// events dispatched to thread logs, and viceversa.
type EventCodec interface {
	// Reduce applies generated events into state.
	Reduce(events []Event, store ds.TxnDatastore, baseKey ds.Key, indexFunc IndexFunc) ([]ReduceAction, error)
	// Create corresponding events to be dispatched.
	Create(ops []Action) ([]Event, format.Node, error)
	// EventsFromBytes deserializes a format.Node bytes payload into Events.
	EventsFromBytes(data []byte) ([]Event, error)
}
