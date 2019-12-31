// Package broadcast implements multi-listener broadcast channels.
// See https://godoc.org/github.com/tjgq/broadcast for original implementation.
//
// To create an un-buffered broadcast channel, just declare a Broadcaster:
//
//     var b broadcast.Broadcaster
//
// To create a buffered broadcast channel with capacity n, call New:
//
//     b := broadcast.New(n)
//
// To add a listener to a channel, call Listen and read from Channel():
//
//     l := b.Listen()
//     for v := range l.Channel() {
//         // ...
//     }
//
//
// To send to the channel, call Send:
//
//     b.Send("Hello world!")
//     v <- l.Channel() // returns interface{}("Hello world!")
//
// To remove a listener, call Discard.
//
//     l.Discard()
//
// To close the broadcast channel, call Discard. Any existing or future listeners
// will read from a closed channel:
//
//     b.Discard()
//     v, ok <- l.Channel() // returns ok == false
package broadcast

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/go-multierror"
)

// ErrClosedChannel means the caller attempted to send to one or more closed broadcast channels.
const ErrClosedChannel = broadcastError("send after close")

type broadcastError string

func (e broadcastError) Error() string { return string(e) }

// Broadcaster implements a Publisher. The zero value is a usable un-buffered channel.
type Broadcaster struct {
	m         sync.Mutex
	listeners map[uint]chan<- interface{} // lazy init
	nextID    uint
	capacity  int
	closed    bool
}

// NewBroadcaster returns a new Broadcaster with the given capacity (0 means un-buffered).
func NewBroadcaster(n int) *Broadcaster {
	return &Broadcaster{capacity: n}
}

// SendWithTimeout broadcasts a message to each listener's channel.
// Sending on a closed channel causes a runtime panic.
// This method blocks for a duration of up to `timeout` on each channel.
// Returns error(s) if it is unable to send on a given listener's channel within `timeout` duration.
func (b *Broadcaster) SendWithTimeout(v interface{}, timeout time.Duration) error {
	b.m.Lock()
	defer b.m.Unlock()
	if b.closed {
		return ErrClosedChannel
	}
	var result *multierror.Error
	for id, l := range b.listeners {
		select {
		case l <- v:
			// Success!
		case <-time.After(timeout):
			err := fmt.Sprintf("unable to send to listener '%d'", id)
			result = multierror.Append(result, errors.New(err))
		}
	}
	if result != nil {
		return result.ErrorOrNil()
	} else {
		return nil
	}
}

// Send broadcasts a message to each listener's channel.
// Sending on a closed channel causes a runtime panic.
// This method is non-blocking, and will return errors if unable to send on a given listener's channel.
func (b *Broadcaster) Send(v interface{}) error {
	return b.SendWithTimeout(v, 0)
}

// Discard closes the channel, disabling the sending of further messages.
func (b *Broadcaster) Discard() {
	b.m.Lock()
	defer b.m.Unlock()
	if b.closed {
		return
	}
	b.closed = true
	for _, l := range b.listeners {
		close(l)
	}
}

// Listen returns a Listener for the broadcast channel.
func (b *Broadcaster) Listen() *Listener {
	b.m.Lock()
	defer b.m.Unlock()
	if b.listeners == nil {
		b.listeners = make(map[uint]chan<- interface{})
	}
	if b.listeners[b.nextID] != nil {
		b.nextID++
	}
	ch := make(chan interface{}, b.capacity)
	if b.closed {
		close(ch)
	}
	b.listeners[b.nextID] = ch
	return &Listener{ch, b, b.nextID}
}

// Listener implements a Subscriber to broadcast channel.
type Listener struct {
	ch <-chan interface{}
	b  *Broadcaster
	id uint
}

// Discard closes the Listener, disabling the reception of further messages.
func (l *Listener) Discard() {
	l.b.m.Lock()
	defer l.b.m.Unlock()
	delete(l.b.listeners, l.id)
}

// Channel returns the channel that receives broadcast messages.
func (l *Listener) Channel() <-chan interface{} {
	return l.ch
}
