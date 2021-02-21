package registry

import (
	"context"
	"errors"
	"fmt"
	"sync"

	ds "github.com/ipfs/go-datastore"
	logging "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/textileio/go-threads/core/did"
	"github.com/textileio/go-threads/core/thread"
	s "github.com/textileio/go-threads/did/store"
)

var (
	log = logging.Logger("registry")
)

type Registry struct {
	ctx    context.Context
	cancel context.CancelFunc
	host   host.Host
	id     thread.Identity
	did    did.DID
	ps     *pubsub.PubSub
	store  *s.Store

	t *pubsub.Topic
	h *pubsub.TopicEventHandler
	s *pubsub.Subscription

	sync.Mutex
}

// NewRegistry returns a new pubsub DID registry.
func NewRegistry(host host.Host, ps *pubsub.PubSub, store ds.Datastore) (*Registry, error) {
	sk := host.Peerstore().PrivKey(host.ID())
	if sk == nil {
		return nil, errors.New("host key not found")
	}
	id := thread.NewLibp2pIdentity(sk)
	self, err := id.GetPublic().DID()
	if err != nil {
		return nil, fmt.Errorf("getting host did: %v", err)
	}

	topic, err := ps.Join(string(thread.Protocol))
	if err != nil {
		return nil, fmt.Errorf("joining registry topic: %v", err)
	}
	handler, err := topic.EventHandler()
	if err != nil {
		return nil, fmt.Errorf("getting topic handler: %v", err)
	}
	sub, err := topic.Subscribe()
	if err != nil {
		return nil, fmt.Errorf("subscribing to registry: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	r := &Registry{
		ctx:    ctx,
		cancel: cancel,
		host:   host,
		id:     thread.NewLibp2pIdentity(sk),
		did:    self,
		ps:     ps,
		store:  s.NewStore(store),
		t:      topic,
		h:      handler,
		s:      sub,
	}

	go r.watch(ctx)
	go r.listen(ctx)
	return r, nil
}

func (r *Registry) Close() error {
	r.Lock()
	defer r.Unlock()
	r.s.Cancel()
	r.h.Cancel()
	if err := r.t.Close(); err != nil {
		return err
	}
	r.cancel()
	return nil
}

func (r *Registry) Register(did did.DID, doc did.Document) error {
	return r.store.Put(did, doc)
}

func (r *Registry) Resolve(did did.DID) (doc did.Document, err error) {
	// First check if we have it locally.
	doc, err = r.store.Get(did)
	if err != nil && !errors.Is(err, ds.ErrNotFound) {
		return doc, err
	}

	// todo: request from other peers

	return doc, nil
}

func (r *Registry) Revoke(did did.DID) error {
	return r.store.Delete(did)
}

// watch registry peer events.
func (r *Registry) watch(ctx context.Context) {
	for {
		e, err := r.h.NextPeerEvent(ctx)
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
		log.Infof("peer event: %s %s", e.Peer, msg)
	}
}

// listen for registry messages.
func (r *Registry) listen(ctx context.Context) {
	for {
		msg, err := r.s.Next(ctx)
		if err != nil {
			break
		}
		from, doc, err := r.handleMsg(msg)
		if err != nil {
			log.Errorf("handling message: %s", err)
			continue
		} else if len(doc.ID) == 0 {
			continue
		}
		log.Debugf("received token from %s", from)
	}
}

func (r *Registry) handleMsg(m *pubsub.Message) (from peer.ID, doc did.Document, err error) {
	from, err = peer.IDFromBytes(m.From)
	if err != nil {
		return from, doc, fmt.Errorf("getting sender id: %v", err)
	}
	if from.String() == r.host.ID().String() {
		return from, doc, nil
	}

	k, doc, err := r.id.GetPublic().Validate(did.Token(m.Data))
	if err != nil {
		return from, doc, fmt.Errorf("validating token: %v", err)
	}
	pk, ok := k.(*thread.Libp2pPubKey)
	if !ok {
		return from, doc, fmt.Errorf("token issuer must be a Libp2pPubKey")
	}
	if !from.MatchesPublicKey(pk.PubKey) {
		return from, doc, fmt.Errorf("token issuer does not match sender: %v", err)
	}
	return from, doc, nil
}
