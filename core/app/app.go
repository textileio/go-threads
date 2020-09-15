package app

import (
	"context"
	"errors"
	"time"

	"github.com/ipfs/go-ipld-format"
	logging "github.com/ipfs/go-log"
	"github.com/textileio/go-threads/broadcast"
	"github.com/textileio/go-threads/core/net"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/go-threads/util"
)

var (
	log = logging.Logger("app")

	// ErrThreadInUse indicates an operation could not be completed because the
	// thread is bound to an app.
	ErrThreadInUse = errors.New("thread is in use")

	// ErrInvalidNetRecordBody indicates the app determined the record body should not be accepted.
	ErrInvalidNetRecordBody = errors.New("app denied net record body")
)

const busTimeout = time.Second * 10

// App provides a bidirectional hook for thread-based apps.
type App interface {
	// ValidateNetRecordBody provides the app an opportunity to validate the contents
	// of a record before it's committed to a thread log.
	// identity is the author's public key.
	ValidateNetRecordBody(ctx context.Context, body format.Node, identity thread.PubKey) error

	// HandleNetRecord handles an inbound thread record from net.
	HandleNetRecord(ctx context.Context, rec net.ThreadRecord, key thread.Key) error
}

// LocalEventsBus wraps a broadcaster for local events.
type LocalEventsBus struct {
	bus *broadcast.Broadcaster
}

// NewLocalEventsBus returns a new bus for local event.
func NewLocalEventsBus() *LocalEventsBus {
	return &LocalEventsBus{bus: broadcast.NewBroadcaster(0)}
}

// Send an IPLD node and thread auth into the bus.
// These are received by the app connector and written to the underlying thread.
func (leb *LocalEventsBus) Send(event *LocalEvent) error {
	return leb.bus.SendWithTimeout(event, busTimeout)
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

// Discard the bus, closing all listeners.
func (leb *LocalEventsBus) Discard() {
	leb.bus.Discard()
}

// LocalEvent wraps an IPLD node and auth for delivery to a thread.
type LocalEvent struct {
	Node  format.Node
	Token thread.Token
}

// LocalEventListener notifies about new locally generated events.
type LocalEventListener struct {
	listener *broadcast.Listener
	c        chan *LocalEvent
}

// Channel returns an unbuffered channel to receive local events.
func (l *LocalEventListener) Channel() <-chan *LocalEvent {
	return l.c
}

// Discard indicates that no further events will be received
// and ready for being garbage collected.
func (l *LocalEventListener) Discard() {
	l.listener.Discard()
}

// Net adds the ability to connect an app to a thread.
type Net interface {
	net.Net

	// ConnectApp returns an app<->thread connector.
	ConnectApp(App, thread.ID) (*Connector, error)

	// Validate thread ID and token against the net host.
	// If token is present and was issued the net host (is valid), the embedded public key is returned.
	// If token is not present, both the returned public key and error will be nil.
	Validate(id thread.ID, token thread.Token, readOnly bool) (thread.PubKey, error)
}

// Connector connects an app to a thread.
type Connector struct {
	Net Net

	app       App
	token     net.Token
	threadID  thread.ID
	threadKey thread.Key
}

// Connection receives new thread records, which are pumped to the app.
type Connection func(context.Context, thread.ID) (<-chan net.ThreadRecord, error)

// NewConnector creates bidirectional connection between an app and a thread.
func NewConnector(app App, net Net, tinfo thread.Info) (*Connector, error) {
	if !tinfo.Key.CanRead() {
		log.Fatalf("read key not found for thread %s", tinfo.ID)
	}
	return &Connector{
		Net:       net,
		app:       app,
		token:     util.GenerateRandomBytes(32),
		threadID:  tinfo.ID,
		threadKey: tinfo.Key,
	}, nil
}

// ThreadID returns the underlying thread's ID.
func (c *Connector) ThreadID() thread.ID {
	return c.threadID
}

// Token returns the net token.
func (c *Connector) Token() net.Token {
	return c.token
}

// CreateNetRecord calls net.CreateRecord while supplying thread ID and API token.
func (c *Connector) CreateNetRecord(ctx context.Context, body format.Node, token thread.Token) (net.ThreadRecord, error) {
	return c.Net.CreateRecord(ctx, c.threadID, body, net.WithThreadToken(token), net.WithAPIToken(c.token))
}

// Validate thread token against the net host.
func (c *Connector) Validate(token thread.Token, readOnly bool) error {
	_, err := c.Net.Validate(c.threadID, token, readOnly)
	return err
}

// ValidateNetRecordBody calls the connection app's ValidateNetRecordBody.
func (c *Connector) ValidateNetRecordBody(ctx context.Context, body format.Node, identity thread.PubKey) error {
	return c.app.ValidateNetRecordBody(ctx, body, identity)
}

// HandleNetRecord calls the connection app's HandleNetRecord while supplying thread key.
func (c *Connector) HandleNetRecord(ctx context.Context, rec net.ThreadRecord) error {
	return c.app.HandleNetRecord(ctx, rec, c.threadKey)
}
