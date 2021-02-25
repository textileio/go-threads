package registry

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	ds "github.com/ipfs/go-datastore"
	logging "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	libpubsub "github.com/libp2p/go-libp2p-pubsub"
	maddr "github.com/multiformats/go-multiaddr"
	d "github.com/textileio/go-threads/core/did"
	"github.com/textileio/go-threads/core/logstore"
	"github.com/textileio/go-threads/core/thread"
	s "github.com/textileio/go-threads/did/store"
	"github.com/textileio/go-threads/net/util"
	"github.com/textileio/go-threads/pubsub"
)

var log = logging.Logger("registry")

//var _ util.SemaphoreKey = (*didLock)(nil)
//
//type didLock map[d.DID]
//
//func (l customerLock) Key() string {
//	return string(l)
//}

type Registry struct {
	host host.Host
	id   thread.Identity
	did  d.DID
	ps   *libpubsub.PubSub

	topic *pubsub.Topic
	store logstore.Logstore
	cache *s.Store
	reqs  map[d.DID]chan d.Document

	ctx    context.Context
	cancel context.CancelFunc

	semaphores *util.SemaphorePool
	lk         sync.Mutex
}

// NewRegistry returns a new pubsub DID registry.
func NewRegistry(host host.Host, store logstore.Logstore, cache ds.Datastore) (*Registry, error) {
	sk := host.Peerstore().PrivKey(host.ID())
	if sk == nil {
		return nil, errors.New("host key not found")
	}
	id := thread.NewLibp2pIdentity(sk)
	self, err := id.GetPublic().DID()
	if err != nil {
		return nil, fmt.Errorf("getting host did: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	ps, err := libpubsub.NewGossipSub(ctx, host)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("starting libp2p pubsub: %v", err)
	}

	topic, err := pubsub.NewTopic(ctx, ps, host.ID(), string(thread.Protocol), true)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("creating registry topic: %v", err)
	}

	r := &Registry{
		host:       host,
		id:         thread.NewLibp2pIdentity(sk),
		did:        self,
		ps:         ps,
		topic:      topic,
		store:      store,
		cache:      s.NewStore(cache),
		reqs:       make(map[d.DID]chan d.Document),
		ctx:        ctx,
		cancel:     cancel,
		semaphores: util.NewSemaphorePool(1),
	}

	topic.SetEventHandler(r.eventHandler)
	topic.SetMessageHandler(r.messageHandler)
	return r, nil
}

// Close the registry.
func (r *Registry) Close() error {
	r.lk.Lock()
	defer r.lk.Unlock()
	r.topic.Close()
	r.cancel()
	return nil
}

//func (r *Registry) Register(did d.DID, doc d.Document) error {
//	return r.store.Put(did, doc)
//}

func (r *Registry) Resolve(ctx context.Context, did d.DID) (doc d.Document, err error) {
	if _, err := thread.FromDID(did); err != nil {
		return doc, fmt.Errorf("decoding did: %v", err)
	}

	r.lk.Lock()
	if _, ok := r.reqs[did]; ok {
		r.lk.Unlock()
		return doc, fmt.Errorf("request for %s already in flight", did)
	}
	resCh := make(chan d.Document)
	r.reqs[did] = resCh
	r.lk.Unlock()
	defer func() {
		r.lk.Lock()
		delete(r.reqs, did)
		r.lk.Unlock()
	}()

	// First, check if we have it locally.
	//doc, err = r.cache.Get(did)
	//if err != nil && !errors.Is(err, ds.ErrNotFound) {
	//	r.lk.Lock()
	//	delete(r.reqs, did)
	//	r.lk.Unlock()
	//	return doc, err
	//}

	// Subscribe to response topic.
	res, err := pubsub.NewTopic(r.ctx, r.ps, r.host.ID(), string(did), true)
	if err != nil {
		return doc, fmt.Errorf("creating results topic %s: %v", did, err)
	}
	defer res.Close()
	res.SetEventHandler(r.eventHandler)
	res.SetMessageHandler(r.messageHandler)

	// Request it from the network.
	if err := r.topic.Publish(ctx, []byte(did)); err != nil {
		return doc, err
	}
	timer := time.NewTimer(time.Second * 5)
	for {
		select {
		case <-r.ctx.Done():
			timer.Stop()
		case <-timer.C:
			return doc, logstore.ErrThreadNotFound
		case doc, ok := <-resCh:
			if ok {
				return doc, nil
			}
		}
	}
}

//func (r *Registry) Revoke(did d.DID) error {
//	return r.store.Delete(did)
//}

func (r *Registry) eventHandler(from peer.ID, topic, msg string) {
	log.Debugf("%s peer event: %s %s", topic, from, msg)
}

func (r *Registry) messageHandler(from peer.ID, topic, msg string) {
	log.Debugf("%s received message from %s", topic, from)

	// See if this message is a DID request.
	id, err := thread.FromDID(d.DID(msg))
	if err == nil {
		if err := r.handleRequest(id, from); err != nil {
			log.Debug(err)
			return
		}
	} else {
		// Not a DID. Treat the message as a token response.
		if err := r.handleResponse(d.Token(msg), from); err != nil {
			log.Debug(err)
			return
		}
	}
}

func (r *Registry) handleRequest(id thread.ID, from peer.ID) error {
	tk, err := r.getToken(id, d.NewKeyDID(from.String()))
	if err != nil {
		return fmt.Errorf("getting token for %s: %v", id, err)
	}

	// Subscribe to response topic.
	res, err := pubsub.NewTopic(r.ctx, r.ps, r.host.ID(), string(id.DID()), false)
	if err != nil {
		return fmt.Errorf("creating results topic %s: %v", id.DID(), err)
	}
	defer res.Close()
	res.SetEventHandler(r.eventHandler)

	// Publish token.
	if err := res.Publish(r.ctx, []byte(tk)); err != nil {
		return fmt.Errorf("publishing response to topic %s: %v", id.DID(), err)
	}
	return nil
}

func (r *Registry) getToken(id thread.ID, aud d.DID) (d.Token, error) {
	// Build a token from local thread info.
	info, err := r.store.GetThread(id)
	if err != nil {
		return "", err
	}
	pc, err := maddr.NewComponent(maddr.ProtocolWithCode(maddr.P_P2P).Name, r.host.ID().String())
	if err != nil {
		return "", nil
	}
	tc, err := maddr.NewComponent(thread.ProtocolName, info.ID.String())
	if err != nil {
		return "", nil
	}
	addrs := r.host.Addrs()
	res := make([]maddr.Multiaddr, len(addrs))
	for i := range addrs {
		res[i] = addrs[i].Encapsulate(pc).Encapsulate(tc)
	}
	info.Addrs = res
	return info.Token(r.id, aud, time.Minute)
}

func (r *Registry) handleResponse(token d.Token, from peer.ID) error {
	// Validate token.
	k, doc, err := r.id.GetPublic().Validate(token)
	if err != nil {
		return fmt.Errorf("validating token: %v", err)
	}
	pk, ok := k.(*thread.Libp2pPubKey)
	if !ok {
		return fmt.Errorf("token issuer must be a Libp2pPubKey")
	}
	if !from.MatchesPublicKey(pk.PubKey) {
		return fmt.Errorf("token issuer does not match sender: %v", err)
	}

	// Send to results channel.
	r.lk.Lock()
	defer r.lk.Unlock()
	ch, ok := r.reqs[doc.ID]
	if ok {
		select {
		case ch <- doc:
		default:
			log.Warnf("slow document receiver for %s", doc.ID)
		}
	}
	return nil
}
