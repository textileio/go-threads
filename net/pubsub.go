package net

import (
	"context"
	"fmt"
	"sync"

	"github.com/gogo/protobuf/proto"
	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/textileio/go-threads/core/thread"
	pb "github.com/textileio/go-threads/net/pb"
)

// TopicHandler receives all pushed thread records.
type TopicHandler func(*pb.PushRecordRequest)

// TopicManager manages thread pubsub topics.
type TopicManager struct {
	sync.RWMutex

	ctx    context.Context
	host   peer.ID
	ps     *pubsub.PubSub
	handle TopicHandler
	m      map[thread.ID]*topic
}

type topic struct {
	t *pubsub.Topic
	h *pubsub.TopicEventHandler
	s *pubsub.Subscription
}

// NewTopicManager returns a new thread topic manager.
func NewTopicManager(ctx context.Context, host peer.ID, ps *pubsub.PubSub, handler TopicHandler) *TopicManager {
	return &TopicManager{
		ctx:    ctx,
		host:   host,
		ps:     ps,
		handle: handler,
		m:      make(map[thread.ID]*topic),
	}
}

// Add a new thread topic. This may be called repeatedly for the same thread.
func (m *TopicManager) Add(id thread.ID) error {
	m.Lock()
	defer m.Unlock()
	if _, ok := m.m[id]; ok {
		return nil
	}

	pt, err := m.ps.Join(id.String())
	if err != nil {
		return err
	}
	h, err := pt.EventHandler()
	if err != nil {
		return err
	}
	if err = m.ps.RegisterTopicValidator(id.String(), m.topicValidator); err != nil {
		return err
	}

	topic := &topic{t: pt, h: h}
	m.m[id] = topic
	go m.watch(m.ctx, id, topic)
	go m.subscribe(m.ctx, id, topic)
	return nil
}

// Remove a thread topic. This may be called repeatedly for the same thread.
func (m *TopicManager) Remove(id thread.ID) error {
	m.Lock()
	defer m.Unlock()
	topic, ok := m.m[id]
	if !ok {
		return nil
	}
	topic.s.Cancel()
	topic.h.Cancel()
	if err := m.ps.UnregisterTopicValidator(id.String()); err != nil {
		return err
	}
	if err := topic.t.Close(); err != nil {
		return err
	}
	delete(m.m, id)
	return nil
}

func (m *TopicManager) topicValidator(context.Context, peer.ID, *pubsub.Message) bool {
	// @todo: determine if this is needed (related to host signatures)
	return true
}

// Publish a record request to a thread.
func (m *TopicManager) Publish(ctx context.Context, id thread.ID, req *pb.PushRecordRequest) error {
	m.RLock()
	defer m.RUnlock()
	topic, ok := m.m[id]
	if !ok {
		return fmt.Errorf("thread topic not found")
	}

	data, err := req.Marshal()
	if err != nil {
		return err
	}
	return topic.t.Publish(ctx, data)
}

// watch peer events from a pubsub topic.
func (m *TopicManager) watch(ctx context.Context, id thread.ID, topic *topic) {
	for {
		pe, err := topic.h.NextPeerEvent(ctx)
		if err != nil {
			break
		}
		var msg string
		switch pe.Type {
		case pubsub.PeerJoin:
			msg = "JOINED"
		case pubsub.PeerLeave:
			msg = "LEFT"
		}
		log.Infof("pubsub peer event: %s %s %s", pe.Peer.String(), msg, id.String())
	}
}

// subscribe to a topic for thread updates.
func (m *TopicManager) subscribe(ctx context.Context, id thread.ID, topic *topic) {
	var err error
	topic.s, err = topic.t.Subscribe()
	if err != nil {
		log.Errorf("error subscribing to topic %s: %s", id.String(), err)
		return
	}

	for {
		msg, err := topic.s.Next(ctx)
		if err != nil {
			break
		}
		from, err := peer.IDFromBytes(msg.From)
		if err != nil {
			log.Errorf("error decoding publisher ID: %s", err)
			continue
		}
		if from.String() == m.host.String() {
			continue
		}

		req := new(pb.PushRecordRequest)
		if err = proto.Unmarshal(msg.Data, req); err != nil {
			log.Errorf("error unmarshaling published record", err)
			continue
		}

		log.Debugf("received multicast record from %s", from.String())

		m.handle(req)
	}
}
