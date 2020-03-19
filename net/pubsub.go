package net

import (
	"context"
	"sync"

	"github.com/gogo/protobuf/proto"
	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/textileio/go-threads/core/thread"
	pb "github.com/textileio/go-threads/net/pb"
)

func (s *server) addTopic(id thread.ID) error {
	if _, ok := s.topics.Load(id); ok {
		return nil
	}

	t, err := s.pubsub.Join(id.String())
	if err != nil {
		return err
	}
	h, err := t.EventHandler()
	if err != nil {
		return err
	}
	if err = s.pubsub.RegisterTopicValidator(id.String(), s.topicValidator); err != nil {
		return err
	}

	topic := &topic{t: t, h: h}
	s.topics.Store(id, topic)
	go s.watch(id, topic)
	go s.subscribe(id, topic)
	return nil
}

func (s *server) removeTopic(id thread.ID) error {
	topic, ok := s.topics.Load(id)
	if !ok {
		return nil
	}
	topic.s.Cancel()
	topic.h.Cancel()
	if err := s.pubsub.UnregisterTopicValidator(id.String()); err != nil {
		return err
	}
	if err := topic.t.Close(); err != nil {
		return err
	}
	s.topics.Delete(id)
	return nil
}

func (s *server) topicValidator(ctx context.Context, from peer.ID, msg *pubsub.Message) bool {
	// @todo: determine if this is needed (related to host signatures)
	return true
}

// subscribe to a topic for thread updates.
func (s *server) subscribe(id thread.ID, topic *topic) {
	var err error
	topic.s, err = topic.t.Subscribe()
	if err != nil {
		log.Errorf("error subscribing to topic %s: %s", id.String(), err)
		return
	}

	for {
		msg, err := topic.s.Next(s.net.ctx)
		if err != nil {
			break
		}
		from, err := peer.IDFromBytes(msg.From)
		if err != nil {
			log.Errorf("error decoding publisher ID: %s", err)
			continue
		}
		if from.String() == s.net.host.ID().String() {
			continue
		}

		req := new(pb.PushRecordRequest)
		if err = proto.Unmarshal(msg.Data, req); err != nil {
			log.Errorf("error unmarshaling published record", err)
			continue
		}

		log.Debugf("received multicast record from %s", from.String())

		if _, err = s.PushRecord(s.net.ctx, req); err != nil {
			log.Errorf("error handling published record: %s", err)
		}
	}
}

// watch peer events from a pubsub topic.
func (s *server) watch(id thread.ID, topic *topic) {
	for {
		pe, err := topic.h.NextPeerEvent(s.net.ctx)
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

// topic wraps the multitude of objects needed to manage a pubsub topic.
type topic struct {
	t *pubsub.Topic
	h *pubsub.TopicEventHandler
	s *pubsub.Subscription
}

type topics struct {
	sync.RWMutex
	m map[thread.ID]*topic
}

func newTopics() *topics {
	return &topics{
		m: make(map[thread.ID]*topic),
	}
}

func (t *topics) Load(key thread.ID) (value *topic, ok bool) {
	t.RLock()
	result, ok := t.m[key]
	t.RUnlock()
	return result, ok
}

func (t *topics) Delete(key thread.ID) {
	t.Lock()
	delete(t.m, key)
	t.Unlock()
}

func (t *topics) Store(key thread.ID, value *topic) {
	t.Lock()
	t.m[key] = value
	t.Unlock()
}
