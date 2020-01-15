package store

import (
	"fmt"
	"sync"

	format "github.com/ipfs/go-ipld-format"
	"github.com/textileio/go-threads/broadcast"
	core "github.com/textileio/go-threads/core/store"
)

// Listen returns a Listener which notifies about actions applying the
// defined filters. The Store *won't* wait for slow receivers, so if the
// channel is full, the action will be dropped.
func (s *Store) Listen(los ...ListenOption) (Listener, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	if s.closed {
		return nil, fmt.Errorf("can't listen on closed Store")
	}

	sl := &listener{
		scn:     s.stateChangedNotifee,
		filters: los,
		c:       make(chan Action, 1),
	}
	s.stateChangedNotifee.addListener(sl)
	return sl, nil
}

// localEventListen returns a listener which notifies *locally generated*
// events in models of the store. Caller should call .Discard() when
// done.
func (s *Store) localEventListen() *LocalEventListener {
	return s.localEventsBus.Listen()
}

func (s *Store) notifyStateChanged(actions []Action) {
	s.stateChangedNotifee.notify(actions)
}

func (s *Store) notifyTxnEvents(node format.Node) error {
	return s.localEventsBus.send(node)
}

// ActionType is the type of the action.
// It is intentionally unique from core.ActionType to provide greater flexibility.
type ActionType int

// ListenActionType is the type for a listener.
type ListenActionType int

// The set of ActionTypes
const (
	ActionCreate ActionType = iota + 1
	ActionSave
	ActionDelete
)

// The set of ListenActionTypes
const (
	ListenAll ListenActionType = iota
	ListenCreate
	ListenSave
	ListenDelete
)

// Action is an operation that can be observed
type Action struct {
	Model string
	Type  ActionType
	ID    core.EntityID
}

// ListenOption is a configuration option for observing Actions
type ListenOption struct {
	Type  ListenActionType
	Model string
	ID    core.EntityID
}

// Listener is an interface describing an object with an Action channel
type Listener interface {
	Channel() <-chan Action
	Close()
}

type stateChangedNotifee struct {
	lock      sync.Mutex
	listeners []*listener
}

type listener struct {
	scn     *stateChangedNotifee
	filters []ListenOption
	c       chan Action
}

var _ Listener = (*listener)(nil)

func (scn *stateChangedNotifee) notify(actions []Action) {
	for _, a := range actions {
		for _, l := range scn.listeners {
			if l.evaluate(a) {
				select {
				case l.c <- a:
				default:
					log.Warnf("dropped action %v for reducer with filters %v", a, l.filters)
				}
			}
		}
	}
}

func (scn *stateChangedNotifee) addListener(sl *listener) {
	scn.lock.Lock()
	defer scn.lock.Unlock()
	scn.listeners = append(scn.listeners, sl)
}

func (scn *stateChangedNotifee) remove(sl *listener) bool {
	scn.lock.Lock()
	defer scn.lock.Unlock()
	for i := range scn.listeners {
		if scn.listeners[i] == sl {
			scn.listeners[i] = scn.listeners[len(scn.listeners)-1]
			scn.listeners[len(scn.listeners)-1] = nil
			scn.listeners = scn.listeners[:len(scn.listeners)-1]
			return true
		}
	}
	return false
}

func (scn *stateChangedNotifee) close() {
	scn.lock.Lock()
	defer scn.lock.Unlock()
	for i := range scn.listeners {
		close(scn.listeners[i].c)
	}
}

// Channel returns an unbuffered channel to receive
// store change notifications
func (sl *listener) Channel() <-chan Action {
	return sl.c
}

// Close indicates that no further notifications will be received
// and ready for being garbage collected
func (sl *listener) Close() {
	if ok := sl.scn.remove(sl); ok {
		close(sl.c)
	}
}

func (sl *listener) evaluate(a Action) bool {
	if len(sl.filters) == 0 {
		return true
	}
	for _, f := range sl.filters {
		switch f.Type {
		case ListenAll:
		case ListenCreate:
			if a.Type != ActionCreate {
				continue
			}
		case ListenSave:
			if a.Type != ActionSave {
				continue
			}
		case ListenDelete:
			if a.Type != ActionDelete {
				continue
			}
		default:
			panic("unknown action type")
		}

		if f.Model != "" && f.Model != a.Model {
			continue
		}

		if f.ID != core.EmptyEntityID && f.ID != a.ID {
			continue
		}
		return true
	}
	return false
}

type localEventsBus struct {
	bus *broadcast.Broadcaster
}

func (leb *localEventsBus) send(node format.Node) error {
	return leb.bus.SendWithTimeout(node, busTimeout)
}

func (leb *localEventsBus) Listen() *LocalEventListener {
	l := &LocalEventListener{
		listener: leb.bus.Listen(),
		c:        make(chan format.Node),
	}

	go func() {
		for v := range l.listener.Channel() {
			events := v.(format.Node)
			l.c <- events
		}
		close(l.c)
	}()

	return l
}

// LocalEventListener notifies about new locally generated ipld.Nodes results
// of transactions
type LocalEventListener struct {
	listener *broadcast.Listener
	c        chan format.Node
}

// Channel returns an unbuffered channel to receive local events
func (l *LocalEventListener) Channel() <-chan format.Node {
	return l.c
}

// Discard indicates that no further events will be received
// and ready for being garbage collected
func (l *LocalEventListener) Discard() {
	l.listener.Discard()
}
