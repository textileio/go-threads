package net

import (
	"context"
	"errors"
	"sync"

	"github.com/gogo/protobuf/proto"
	"github.com/libp2p/go-libp2p-core/peer"
	gostream "github.com/libp2p/go-libp2p-gostream"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/textileio/go-threads/core/thread"
	pb "github.com/textileio/go-threads/net/pb"
	grpcpeer "google.golang.org/grpc/peer"
)

// Handler receives all pushed thread records.
type Handler func(context.Context, *pb.PushRecordRequest)

// PubSub manages thread pubsub topics.
type PubSub struct {
	sync.RWMutex

	ctx     context.Context
	host    peer.ID
	ps      *pubsub.PubSub
	handler Handler
	m       map[thread.ID]*topic
}

type topic struct {
	t *pubsub.Topic
	h *pubsub.TopicEventHandler
	s *pubsub.Subscription

	cancel context.CancelFunc
}

// NewPubSub returns a new thread topic manager.
func NewPubSub(ctx context.Context, host peer.ID, ps *pubsub.PubSub, handler Handler) *PubSub {
	return &PubSub{
		ctx:     ctx,
		host:    host,
		ps:      ps,
		handler: handler,
		m:       make(map[thread.ID]*topic),
	}
}

// Add a new thread topic. This may be called repeatedly for the same thread.
func (s *PubSub) Add(id thread.ID) error {
	s.Lock()
	defer s.Unlock()
	if _, ok := s.m[id]; ok {
		return nil
	}

	if err := id.Validate(); err != nil {
		return err
	}
	pt, err := s.ps.Join(id.String())
	if err != nil {
		return err
	}
	h, err := pt.EventHandler()
	if err != nil {
		return err
	}
	if err = s.ps.RegisterTopicValidator(id.String(), s.topicValidator); err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(s.ctx)
	topic := &topic{
		t:      pt,
		h:      h,
		cancel: cancel,
	}
	s.m[id] = topic
	go s.watch(ctx, id, topic)
	go s.subscribe(ctx, id, topic)
	return nil
}

// Remove a thread topic. This may be called repeatedly for the same thread.
func (s *PubSub) Remove(id thread.ID) error {
	s.Lock()
	defer s.Unlock()
	topic, ok := s.m[id]
	if !ok {
		return nil
	}
	topic.s.Cancel()
	topic.h.Cancel()
	if err := id.Validate(); err != nil {
		return err
	}
	if err := s.ps.UnregisterTopicValidator(id.String()); err != nil {
		return err
	}
	if err := topic.t.Close(); err != nil {
		return err
	}
	topic.cancel()
	delete(s.m, id)
	return nil
}

func (s *PubSub) topicValidator(context.Context, peer.ID, *pubsub.Message) bool {
	// @todo: determine if this is needed (related to host signatures)
	return true
}

// Publish a record request to a thread.
func (s *PubSub) Publish(ctx context.Context, id thread.ID, req *pb.PushRecordRequest) error {
	s.RLock()
	defer s.RUnlock()
	topic, ok := s.m[id]
	if !ok {
		return errors.New("thread topic not found")
	}

	data, err := req.Marshal()
	if err != nil {
		return err
	}
	return topic.t.Publish(ctx, data)
}

// watch peer events from a pubsub topic.
func (s *PubSub) watch(ctx context.Context, id thread.ID, topic *topic) {
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
		log.Infof("pubsub peer event: %s %s %s", pe.Peer, msg, id)
	}
}

// subscribe to a topic for thread updates.
func (s *PubSub) subscribe(ctx context.Context, id thread.ID, topic *topic) {
	var err error
	s.Lock()
	topic.s, err = topic.t.Subscribe()
	s.Unlock()
	if err != nil {
		log.Errorf("error subscribing to topic %s: %s", id, err)
		return
	}

	for {
		msg, err := topic.s.Next(ctx)
		if err != nil {
			break
		}
		from, req, err := s.handleMsg(msg)
		if err != nil {
			log.Errorf("error handling multicast request: %s", err)
			continue
		} else if req == nil {
			continue
		}
		log.Debugf("received multicast record from %s", from)

		ctx = grpcpeer.NewContext(ctx, &grpcpeer.Peer{
			Addr: &addr{id: from},
		})
		s.handler(ctx, req)
	}
}

func (s *PubSub) handleMsg(m *pubsub.Message) (from peer.ID, rec *pb.PushRecordRequest, err error) {
	from, err = peer.IDFromBytes(m.From)
	if err != nil {
		return "", nil, err
	}
	if from.String() == s.host.String() {
		return "", nil, nil
	}

	req := new(pb.PushRecordRequest)
	if err = proto.Unmarshal(m.Data, req); err != nil {
		return "", nil, err
	}
	return from, req, nil
}

// addr implements net.Addr and holds a libp2p peer ID.
type addr struct{ id peer.ID }

// Network returns the name of the network that this address belongs to (libp2p).
func (a *addr) Network() string { return gostream.Network }

// String returns the peer ID of this address in string form
// (B58-encoded).
func (a *addr) String() string { return a.id.Pretty() }
