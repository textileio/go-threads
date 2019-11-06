// This file is temporary in in this repository. On the first minimal
// stable version, this file should be moved to go-textile-core

package core

import (
	"bytes"
	"encoding/binary"
	"time"

	"github.com/google/uuid"
	ds "github.com/ipfs/go-datastore"
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

// Event is a local or remote event generated in a model and dispatcher
// by Dispatcher.
type Event interface {
	Body() []byte
	Time() []byte
	EntityID() EntityID
	Type() string
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
	// EntityType of the instance in action
	EntityType string
	// Previous is the instance before the action
	Previous interface{}
	// Current is the instance after the action was done
	Current interface{}
}

// EventCodec transforms actions generated in models to
// events dispatched to thread logs, and viceversa.
type EventCodec interface {
	// Reduce applies generated events into state
	Reduce(e Event, datastore ds.Datastore, baseKey ds.Key) error
	// Create corresponding events to be dispatched
	Create(ops []Action) ([]Event, error)
}

// ToDo: kick out for testing only

func NewNullEvent(t time.Time) Event {
	return &nullEvent{Timestamp: t}
}

type nullEvent struct {
	Timestamp time.Time
}

func (n *nullEvent) Body() []byte {
	return nil
}

func (n *nullEvent) Time() []byte {
	t := n.Timestamp.UnixNano()
	buf := new(bytes.Buffer)
	// Use big endian to preserve lexicographic sorting
	binary.Write(buf, binary.BigEndian, t)
	return buf.Bytes()
}

func (n *nullEvent) EntityID() EntityID {
	return "null"
}

func (n *nullEvent) Type() string {
	return "null"
}

// Sanity check
var _ Event = (*nullEvent)(nil)
