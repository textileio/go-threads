package store

import (
	"fmt"
	"sync"

	ipldformat "github.com/ipfs/go-ipld-format"
	core "github.com/textileio/go-textile-core/store"
	"github.com/textileio/go-threads/broadcast"
)

// Listen returns a StoreListener which notifies about actions applying the
// defined filters. The Store *won't* wait for slow receivers, so if the
// channel is full, the action will be dropped.
func (s *Store) Listen(los ...ListenOption) (StoreListener, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	if s.closed {
		return nil, fmt.Errorf("can't listen on closed Store")
	}

	sl := &storeListener{
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

func (s *Store) notifyTxnEvents(node ipldformat.Node) error {
	return s.localEventsBus.send(node)
}

type ActionType int
type ListenActionType int

const (
	ActionCreate ActionType = iota + 1
	ActionSave
	ActionDelete
)

const (
	ListenAll ListenActionType = iota
	ListenCreate
	ListenSave
	ListenDelete
)

type Action struct {
	Model string
	Type  ActionType
	ID    core.EntityID
}

type ListenOption struct {
	Type  ListenActionType
	Model string
	ID    core.EntityID
}

type StoreListener interface {
	Channel() <-chan Action
	Close()
}

type stateChangedNotifee struct {
	lock      sync.Mutex
	listeners []*storeListener
}

type storeListener struct {
	scn     *stateChangedNotifee
	filters []ListenOption
	c       chan Action
}

var _ StoreListener = (*storeListener)(nil)

func (scn *stateChangedNotifee) notify(actions []Action) {
	for _, a := range actions {
		for _, l := range scn.listeners {
			if l.evaluate(a) {
				select {
				case l.c <- a:
				default:
					log.Warningf("dropped action %v for reducer with filters %v", a, l.filters)
				}
			}
		}
	}
}

func (scn *stateChangedNotifee) addListener(sl *storeListener) {
	scn.lock.Lock()
	defer scn.lock.Unlock()
	scn.listeners = append(scn.listeners, sl)
}

func (scn *stateChangedNotifee) remove(sl *storeListener) bool {
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
func (sl *storeListener) Channel() <-chan Action {
	return sl.c
}

// Close indicates that no further notifications will be received
// and ready for being garbage collected
func (sl *storeListener) Close() {
	if ok := sl.scn.remove(sl); ok {
		close(sl.c)
	}
}

func (sl *storeListener) evaluate(a Action) bool {
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

func (leb *localEventsBus) send(node ipldformat.Node) error {
	return leb.bus.SendWithTimeout(node, busTimeout)
}

func (leb *localEventsBus) Listen() *LocalEventListener {
	l := &LocalEventListener{
		listener: leb.bus.Listen(),
		c:        make(chan ipldformat.Node),
	}

	go func() {
		for v := range l.listener.Channel() {
			events := v.(ipldformat.Node)
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
	c        chan ipldformat.Node
}

// Channel returns an unbuffered channel to receive local events
func (l *LocalEventListener) Channel() <-chan ipldformat.Node {
	return l.c
}

// Discard indicates that no further events will be received
// and ready for being garbage collected
func (l *LocalEventListener) Discard() {
	l.listener.Discard()
}
