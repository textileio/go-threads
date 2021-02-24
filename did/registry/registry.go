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
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	maddr "github.com/multiformats/go-multiaddr"
	d "github.com/textileio/go-threads/core/did"
	"github.com/textileio/go-threads/core/logstore"
	"github.com/textileio/go-threads/core/thread"
	s "github.com/textileio/go-threads/did/store"
)

var (
	log = logging.Logger("registry")
)

type Registry struct {
	host  host.Host
	id    thread.Identity
	did   d.DID
	ps    *pubsub.PubSub
	store logstore.Logstore
	cache *s.Store
	reqs  map[d.DID]chan d.Document

	t *pubsub.Topic
	h *pubsub.TopicEventHandler
	s *pubsub.Subscription

	ctx    context.Context
	cancel context.CancelFunc

	sync.Mutex
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
	ps, err := pubsub.NewGossipSub(ctx, host)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("starting libp2p pubsub: %v", err)
	}

	r := &Registry{
		host:   host,
		id:     thread.NewLibp2pIdentity(sk),
		did:    self,
		ps:     ps,
		store:  store,
		cache:  s.NewStore(cache),
		reqs:   make(map[d.DID]chan d.Document),
		ctx:    ctx,
		cancel: cancel,
	}
	return r, r.join()
}

func (r *Registry) join() (err error) {
	r.t, err = r.ps.Join(string(thread.Protocol))
	if err != nil {
		return fmt.Errorf("joining registry topic: %v", err)
	}
	r.h, err = r.t.EventHandler()
	if err != nil {
		return fmt.Errorf("getting topic handler: %v", err)
	}
	r.s, err = r.t.Subscribe()
	if err != nil {
		return fmt.Errorf("subscribing to registry: %v", err)
	}
	go r.watch()
	go r.listen()
	return nil
}

// Close the registry.
func (r *Registry) Close() error {
	r.Lock()
	defer r.Unlock()
	if r.s != nil {
		r.s.Cancel()
	}
	if r.h != nil {
		r.h.Cancel()
	}
	if r.t != nil {
		if err := r.t.Close(); err != nil {
			return err
		}
	}
	r.cancel()
	return nil
}

//func (r *Registry) Register(did d.DID, doc d.Document) error {
//	return r.store.Put(did, doc)
//}

func (r *Registry) Resolve(ctx context.Context, did d.DID) (doc d.Document, err error) {
	r.Lock()
	if _, ok := r.reqs[did]; ok {
		r.Unlock()
		return doc, fmt.Errorf("request for %s already in flight", did)
	}
	r.reqs[did] = make(chan d.Document)
	r.Unlock()

	// First, check if we have it locally.
	doc, err = r.cache.Get(did)
	if err != nil && !errors.Is(err, ds.ErrNotFound) {
		r.Lock()
		delete(r.reqs, did)
		r.Unlock()
		return doc, err
	}

	// Subscribe to response topic.
	t, err := r.ps.Join(string(did))
	if err != nil {
		return doc, fmt.Errorf("joining response topic %s: %v", did, err)
	}
	defer t.Close()
	ctx, cancel := context.WithCancel(r.ctx)
	defer cancel()
	t.Subscribe()

	// Request it from the network.
	if err := r.t.Publish(ctx, []byte(did)); err != nil {
		return doc, err
	}
	timer := time.NewTimer(time.Second * 5)
	for {
		select {
		case <-r.ctx.Done():
			timer.Stop()
		case <-timer.C:
			return doc, logstore.ErrThreadNotFound
		case doc, ok := <-r.reqs[did]:
			if ok {
				return doc, nil
			}
		}
	}
}

//func (r *Registry) Revoke(did d.DID) error {
//	return r.store.Delete(did)
//}

// watch registry peer events.
func (r *Registry) watch() {
	for {
		e, err := r.h.NextPeerEvent(r.ctx)
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
func (r *Registry) listen() {
	for {
		msg, err := r.s.Next(r.ctx)
		if err != nil {
			break
		}
		if msg.ReceivedFrom.String() == r.host.ID().String() {
			continue
		}
		log.Debugf("received message from %s", msg.ReceivedFrom)

		id, err := thread.FromDID(d.DID(msg.Data))
		if err != nil {
			if err := r.handleThreadToken(d.Token(msg.Data), msg.ReceivedFrom); err != nil {
				log.Debug(err)
				continue
			}
		} else {
			if err := r.handleThreadRequest(id, msg.ReceivedFrom); err != nil {
				log.Debug(err)
				continue
			}
		}
	}
}

func (r *Registry) handleThreadRequest(id thread.ID, from peer.ID) error {
	tk, err := r.getThreadToken(id, d.NewKeyDID(from.String()))
	if err != nil {
		return fmt.Errorf("getting document for %s: %v", id, err)
	}

	topic := string(id.DID())
	t, err := r.ps.Join(topic)
	if err != nil {
		return fmt.Errorf("joining response topic %s: %v", topic, err)
	}
	defer t.Close()
	ctx, cancel := context.WithCancel(r.ctx)
	defer cancel()
	if err := t.Publish(ctx, []byte(tk)); err != nil {
		return fmt.Errorf("publishing response to topic %s: %v", topic, err)
	}
	return nil
}

func (r *Registry) handleThreadToken(token d.Token, from peer.ID) error {
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

	r.Lock()
	defer r.Unlock()
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

func (r *Registry) getThreadToken(id thread.ID, aud d.DID) (d.Token, error) {
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

//// queryResultSet holds a unique set of search results
//type queryResultSet struct {
//	options *pb.QueryOptions
//	items   map[string]*pb.QueryResult
//	lock    sync.Mutex
//}
//
//// newQueryResultSet returns a new queryResultSet
//func newQueryResultSet(options *pb.QueryOptions) *queryResultSet {
//	return &queryResultSet{
//		options: options,
//		items:   make(map[string]*pb.QueryResult, 0),
//	}
//}
//
//// Add only adds a result to the set if it's newer than last
//func (s *queryResultSet) Add(items ...*pb.QueryResult) []*pb.QueryResult {
//	s.lock.Lock()
//	defer s.lock.Unlock()
//
//	var added []*pb.QueryResult
//	for _, i := range items {
//		last := s.items[i.Id]
//		switch s.options.Filter {
//		case pb.QueryOptions_NO_FILTER:
//			break
//		case pb.QueryOptions_HIDE_OLDER:
//			if last != nil && util.ProtoNanos(i.Date) <= util.ProtoNanos(last.Date) {
//				continue
//			}
//		}
//		s.items[i.Id] = i
//		added = append(added, i)
//	}
//
//	return added
//}
//
//// List returns the items as a slice
//func (s *queryResultSet) List() []*pb.QueryResult {
//	s.lock.Lock()
//	defer s.lock.Unlock()
//
//	var list []*pb.QueryResult
//	for _, i := range s.items {
//		list = append(list, i)
//	}
//
//	return list
//}
//
//// Full returns whether or not the number of results meets or exceeds limit
//func (s *queryResultSet) Full() bool {
//	s.lock.Lock()
//	defer s.lock.Unlock()
//
//	return len(s.items) >= int(s.options.Limit)
//}
//
//// searchPubSub performs a network-wide search for the given query
//func (h *CafeService) searchPubSub(query *pb.Query, reply func(*pb.QueryResults) bool, cancelCh <-chan interface{}, fromCafe bool) error {
//	h.inFlightQueries[query.Id] = struct{}{}
//	defer func() {
//		delete(h.inFlightQueries, query.Id)
//	}()
//
//	// respond pubsub if this is a cafe and the request is not from a cafe
//	var rtype pb.PubSubQuery_ResponseType
//	if h.open && !fromCafe {
//		rtype = pb.PubSubQuery_PUBSUB
//	} else {
//		rtype = pb.PubSubQuery_P2P
//	}
//
//	err := h.publishQuery(&pb.PubSubQuery{
//		Id:           query.Id,
//		Type:         query.Type,
//		Payload:      query.Payload,
//		ResponseType: rtype,
//		Exclude:      query.Options.Exclude,
//		Topic:        string(cafeServiceProtocol) + "/" + h.service.Node().Identity.Pretty(),
//		Timeout:      query.Options.Wait,
//	})
//	if err != nil {
//		return err
//	}
//
//	timer := time.NewTimer(time.Second * time.Duration(query.Options.Wait))
//	listener := h.queryResults.Listen()
//	doneCh := make(chan struct{})
//
//	done := func() {
//		listener.Close()
//		close(doneCh)
//	}
//
//	go func() {
//		<-timer.C
//		done()
//	}()
//
//	for {
//		select {
//		case <-cancelCh:
//			if timer.Stop() {
//				done()
//			}
//		case <-doneCh:
//			return nil
//		case value, ok := <-listener.Ch:
//			if !ok {
//				return nil
//			}
//			if r, ok := value.(*pb.PubSubQueryResults); ok && r.Id == query.Id && r.Results.Type == query.Type {
//				if reply(r.Results) {
//					if timer.Stop() {
//						done()
//					}
//				}
//			}
//		}
//	}
//}
//
//// Search searches the network based on the given query
//func (t *Textile) search(query *pb.Query) (<-chan *pb.QueryResult, <-chan error, *broadcast.Broadcaster) {
//	query = queryDefaults(query)
//	query.Id = ksuid.New().String()
//
//	var searchChs []chan *pb.QueryResult
//
//	// local results channel
//	localCh := make(chan *pb.QueryResult)
//	searchChs = append(searchChs, localCh)
//
//	// remote results channel(s)
//	var cafeChs []chan *pb.QueryResult
//	clientCh := make(chan *pb.QueryResult)
//	sessions := t.datastore.CafeSessions().List().Items
//	if len(sessions) > 0 {
//		for range sessions {
//			cafeCh := make(chan *pb.QueryResult)
//			cafeChs = append(cafeChs, cafeCh)
//			searchChs = append(searchChs, cafeCh)
//		}
//	} else {
//		searchChs = append(searchChs, clientCh)
//	}
//
//	resultCh := mergeQueryResults(searchChs)
//	errCh := make(chan error)
//	cancel := broadcast.NewBroadcaster(0)
//
//	go func() {
//		defer func() {
//			for _, ch := range searchChs {
//				close(ch)
//			}
//		}()
//		results := newQueryResultSet(query.Options)
//
//		// search local
//		if !query.Options.RemoteOnly {
//			var err error
//			results, err = t.cafe.searchLocal(query.Type, query.Options, query.Payload, true)
//			if err != nil {
//				errCh <- err
//				return
//			}
//			for _, res := range results.items {
//				localCh <- res
//			}
//		}
//
//		if query.Options.LocalOnly || results.Full() {
//			return
//		}
//
//		// search the network
//		if len(sessions) == 0 {
//
//			// search via pubsub directly
//			canceler := cancel.Listen()
//			if err := t.cafe.searchPubSub(query, func(res *pb.QueryResults) bool {
//				for _, n := range results.Add(res.Items...) {
//					clientCh <- n
//				}
//				return results.Full()
//			}, canceler.Ch, false); err != nil {
//				errCh <- err
//				return
//			}
//
//		} else {
//
//			// search via cafes
//			wg := sync.WaitGroup{}
//			for i, session := range sessions {
//				canceler := cancel.Listen()
//
//				wg.Add(1)
//				go func(i int, cafeId string, canceler *broadcast.Listener) {
//					defer wg.Done()
//
//					// token must be attached per cafe session, use a new query
//					q := &pb.Query{}
//					*q = *query
//					if err := t.cafe.Search(q, cafeId, func(res *pb.QueryResult) {
//						for _, n := range results.Add(res) {
//							cafeChs[i] <- n
//						}
//						if results.Full() {
//							cancel.Close()
//						}
//					}, canceler.Ch); err != nil {
//						errCh <- err
//						return
//					}
//				}(i, session.Id, canceler)
//			}
//
//			wg.Wait()
//		}
//	}()
//
//	return resultCh, errCh, cancel
//}
//
//// mergeQueryResults merges results from mulitple queries
//func mergeQueryResults(cs []chan *pb.QueryResult) chan *pb.QueryResult {
//	out := make(chan *pb.QueryResult)
//	var wg sync.WaitGroup
//	wg.Add(len(cs))
//	for _, c := range cs {
//		go func(c chan *pb.QueryResult) {
//			for v := range c {
//				out <- v
//			}
//			wg.Done()
//		}(c)
//	}
//	go func() {
//		wg.Wait()
//		close(out)
//	}()
//	return out
//}
