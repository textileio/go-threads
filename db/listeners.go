package db

import (
	"fmt"
	"sync"

	"github.com/ipfs/go-ipld-format"
	"github.com/textileio/go-threads/core/app"
	core "github.com/textileio/go-threads/core/db"
	"github.com/textileio/go-threads/core/thread"
)

// Listen returns a Listener which notifies about actions applying the
// defined filters. The DB *won't* wait for slow receivers, so if the
// channel is full, the action will be dropped.
func (d *DB) Listen(los ...ListenOption) (Listener, error) {
	d.txnlock.Lock()
	defer d.txnlock.Unlock()
	if d.closed {
		return nil, fmt.Errorf("can't listen on closed DB")
	}

	sl := &listener{
		scn:     d.stateChangedNotifee,
		filters: los,
		c:       make(chan Action, 1),
	}
	d.stateChangedNotifee.addListener(sl)
	return sl, nil
}

func (d *DB) notifyStateChanged(actions []Action) {
	d.stateChangedNotifee.notify(actions)
}

func (d *DB) notifyTxnEvents(node format.Node, token thread.Token) error {
	return d.localEventsBus.Send(&app.LocalEvent{
		Node:  node,
		Token: token,
	})
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
	Collection string
	Type       ActionType
	ID         core.InstanceID
}

type ListenOption struct {
	Type       ListenActionType
	Collection string
	ID         core.InstanceID
}

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
		scn.listeners[i].Close()
	}
}

// Channel returns an unbuffered channel to receive
// db change notifications
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

		if f.Collection != "" && f.Collection != a.Collection {
			continue
		}

		if f.ID != core.EmptyInstanceID && f.ID != a.ID {
			continue
		}
		return true
	}
	return false
}
