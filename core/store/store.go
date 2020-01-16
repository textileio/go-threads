package store

import (
	"github.com/google/uuid"
	ds "github.com/ipfs/go-datastore"
	format "github.com/ipfs/go-ipld-format"
)

const (
	// EmptyEntityID represents an empty EntityID
	EmptyEntityID = EntityID("")
)

// EntityID is the type used in models identities
type EntityID string

// NewEntityID generates a new identity for an instance
func NewEntityID() EntityID {
	return EntityID(uuid.New().String())
}

func (e EntityID) String() string {
	return string(e)
}

func IsValidEntityID(entityID string) bool {
	_, err := uuid.Parse(entityID)
	return err == nil
}

// Event is a local or remote event generated in a model and dispatcher
// by Dispatcher.
type Event interface {
	Time() []byte
	EntityID() EntityID
	Model() string
}

// ActionType is the type used by actions done in a txn
type ActionType int

const (
	// Create indicates the creation of an instance in a txn
	Create ActionType = iota
	// Save indicates the mutation of an instance in a txn
	Save
	// Delete indicates the deletion of an instance by ID in a txn
	Delete
)

// Action is a operation done in the model
type Action struct {
	// Type of the action
	Type ActionType
	// EntityID of the instance in action
	EntityID EntityID
	// ModelName of the instance in action
	ModelName string
	// Previous is the instance before the action
	Previous interface{}
	// Current is the instance after the action was done
	Current interface{}
}

type ReduceAction struct {
	// Type of the reduced action
	Type ActionType
	// Model in which action was made
	Model string
	// EntityID of the instance in reduced action
	EntityID EntityID
}

// EventCodec transforms actions generated in models to
// events dispatched to thread logs, and viceversa.
type EventCodec interface {
	// Reduce applies generated events into state
	Reduce(
		events []Event,
		datastore ds.TxnDatastore,
		baseKey ds.Key,
		indexFunc func(model string, key ds.Key, data []byte, txn ds.Txn) error,
	) ([]ReduceAction, error)
	// Create corresponding events to be dispatched
	Create(ops []Action) ([]Event, format.Node, error)
	// EventsFromBytes deserializes a format.Node bytes payload into
	// Events.
	EventsFromBytes(data []byte) ([]Event, error)
}
