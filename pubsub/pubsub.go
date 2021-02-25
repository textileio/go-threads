package pubsub

import (
	"context"
	"fmt"
	"sync"

	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

// Handler is used to receive topic peer events and messages.
type Handler func(from peer.ID, topic, msg string)

// Topic provides a nice interface to a libp2p pubsub topic.
type Topic struct {
	host           peer.ID
	eventHandler   Handler
	messageHandler Handler

	t *pubsub.Topic
	h *pubsub.TopicEventHandler
	s *pubsub.Subscription

	ctx    context.Context
	cancel context.CancelFunc

	lk sync.Mutex
}

// NewTopic returns a new topic for the host.
func NewTopic(ctx context.Context, ps *pubsub.PubSub, host peer.ID, topic string, subscribe bool) (*Topic, error) {
	top, err := ps.Join(topic)
	if err != nil {
		return nil, fmt.Errorf("joining topic: %v", err)
	}

	handler, err := top.EventHandler()
	if err != nil {
		return nil, fmt.Errorf("getting topic handler: %v", err)
	}

	var sub *pubsub.Subscription
	if subscribe {
		sub, err = top.Subscribe()
		if err != nil {
			return nil, fmt.Errorf("subscribing to topic: %v", err)
		}
	}

	t := &Topic{
		host: host,
		t:    top,
		h:    handler,
		s:    sub,
	}
	t.ctx, t.cancel = context.WithCancel(ctx)

	go t.watch()
	if t.s != nil {
		go t.listen()
	}
	return t, nil
}

// Close the topic.
func (t *Topic) Close() error {
	t.lk.Lock()
	defer t.lk.Unlock()
	t.h.Cancel()
	if t.s != nil {
		t.s.Cancel()
	}
	if err := t.t.Close(); err != nil {
		return err
	}
	t.cancel()
	return nil
}

// SetEventHandler sets a handler func that will be called with peer events.
func (t *Topic) SetEventHandler(handler Handler) {
	t.lk.Lock()
	defer t.lk.Unlock()
	t.eventHandler = handler
}

// SetMessageHandler sets a handler func that will be called with topic messages.
// A subscription is required for the handler to be called.
func (t *Topic) SetMessageHandler(handler Handler) {
	t.lk.Lock()
	defer t.lk.Unlock()
	t.messageHandler = handler
}

// Publish data.
func (t *Topic) Publish(ctx context.Context, data []byte, opts ...pubsub.PubOpt) error {
	return t.t.Publish(ctx, data, opts...)
}

func (t *Topic) watch() {
	for {
		e, err := t.h.NextPeerEvent(t.ctx)
		if err != nil {
			break
		}
		var msg string
		switch e.Type {
		case pubsub.PeerJoin:
			msg = "JOINED"
		case pubsub.PeerLeave:
			msg = "LEFT"
		}
		t.lk.Lock()
		if t.eventHandler != nil {
			t.eventHandler(e.Peer, t.t.String(), msg)
		}
		t.lk.Unlock()
	}
}

func (t *Topic) listen() {
	for {
		msg, err := t.s.Next(t.ctx)
		if err != nil {
			break
		}
		if msg.ReceivedFrom.String() == t.host.String() {
			continue
		}
		t.lk.Lock()
		if t.messageHandler != nil {
			t.messageHandler(msg.ReceivedFrom, t.t.String(), string(msg.Data))
		}
		t.lk.Unlock()
	}
}
