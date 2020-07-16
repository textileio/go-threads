package app

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	format "github.com/ipfs/go-ipld-format"
	logging "github.com/ipfs/go-log"
	peer "github.com/libp2p/go-libp2p-core/peer"
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
)

const (
	busTimeout        = time.Second * 10
	addRecordTimeout  = time.Second * 10
	fetchEventTimeout = time.Second * 15
)

// App provides a bidirectional hook for thread-based apps.
type App interface {
	// LocalEventListen returns a listener that is notified of *locally generated*
	// events. Caller should call .Discard() when done.
	LocalEventListen() *LocalEventListener

	// HandleNetRecord handles an inbound thread record from net.
	HandleNetRecord(rec net.ThreadRecord, key thread.Key, lid peer.ID, timeout time.Duration) error
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
}

// Connector connects an app to a thread.
type Connector struct {
	Net Net

	app        App
	token      net.Token
	threadID   thread.ID
	threadKey  thread.Key
	logID      peer.ID
	closeChan  chan struct{}
	goRoutines sync.WaitGroup

	lock   sync.Mutex
	closed bool
}

// Connection receives new thread records, which are pumped to the app.
type Connection func(context.Context, thread.ID) (<-chan net.ThreadRecord, error)

// NewConnector creates bidirectional connection between an app and a thread.
func NewConnector(app App, net Net, tinfo thread.Info, conn Connection) (*Connector, error) {
	if !tinfo.Key.CanRead() {
		log.Fatalf("read key not found for thread %s", tinfo.ID)
	}
	lg := tinfo.GetOwnLog()
	if lg == nil {
		return nil, fmt.Errorf("own log for thread %s does not exist", tinfo.ID)
	}
	a := &Connector{
		Net:       net,
		app:       app,
		token:     util.GenerateRandomBytes(32),
		threadID:  tinfo.ID,
		threadKey: tinfo.Key,
		logID:     lg.ID,
		closeChan: make(chan struct{}),
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go a.threadToApp(conn, &wg)
	go a.appToThread(&wg)
	wg.Wait()
	a.goRoutines.Add(2)
	return a, nil
}

// Close stops the bidirectional connection between the app and thread.
func (c *Connector) Close() error {
	c.lock.Lock()
	defer c.lock.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	close(c.closeChan)
	c.goRoutines.Wait()
	return nil
}

// ThreadID returns the underlying thread's ID.
func (c *Connector) ThreadID() thread.ID {
	return c.threadID
}

// Token returns the net token.
func (c *Connector) Token() net.Token {
	return c.token
}

func (c *Connector) threadToApp(con Connection, wg *sync.WaitGroup) {
	defer c.goRoutines.Done()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sub, err := con(ctx, c.threadID)
	if err != nil {
		log.Fatalf("error getting thread subscription: %v", err)
	}
	wg.Done()
	for {
		select {
		case <-c.closeChan:
			log.Debugf("closing thread-to-app flow on thread %s", c.threadID)
			return
		case rec, ok := <-sub:
			if !ok {
				log.Debug("notification channel closed, not listening to external changes anymore")
				return
			}
			if err = c.app.HandleNetRecord(rec, c.threadKey, c.logID, fetchEventTimeout); err != nil {
				log.Fatal(err)
			}
		}
	}
}

func (c *Connector) appToThread(wg *sync.WaitGroup) {
	defer c.goRoutines.Done()
	l := c.app.LocalEventListen()
	defer l.Discard()
	wg.Done()

	for {
		select {
		case <-c.closeChan:
			log.Debugf("closing app-to-thread flow on thread %s", c.threadID)
			return
		case event, ok := <-l.Channel():
			if !ok {
				log.Errorf("ending sending app local event to own thread since channel was closed for thread %s", c.threadID)
				return
			}
			ctx, cancel := context.WithTimeout(context.Background(), addRecordTimeout)
			if _, err := c.Net.CreateRecord(ctx, c.threadID, event.Node, net.WithThreadToken(event.Token), net.WithAPIToken(c.token)); err != nil {
				log.Fatalf("error writing record: %v", err)
			}
			cancel()
		}
	}
}
