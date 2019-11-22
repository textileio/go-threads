package eventstore

import (
	"context"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
	tservopts "github.com/textileio/go-textile-core/options"
	"github.com/textileio/go-textile-core/thread"
	tserv "github.com/textileio/go-textile-core/threadservice"
	threadcbor "github.com/textileio/go-textile-threads/cbor"
	"github.com/textileio/go-textile-threads/util"
)

const (
	addRecordTimeout  = time.Second * 10
	fetchEventTimeout = time.Second * 15
)

// SingleThreadAdapter connects a Store with a Threadservice
type singleThreadAdapter struct {
	ctx      context.Context
	api      tserv.Threadservice
	store    *Store
	threadID thread.ID
	ownLogID peer.ID

	lock    sync.Mutex
	started bool
}

// NewSingleThreadAdapter returns a new Adapter which maps
// a Store with a single Thread
func newSingleThreadAdapter(ctx context.Context, store *Store, threadID thread.ID) *singleThreadAdapter {
	a := &singleThreadAdapter{
		ctx:      ctx,
		api:      store.Threadservice(),
		threadID: threadID,
		store:    store,
	}

	return a
}

// Start starts connection from Store to Threadservice, and viceversa
func (a *singleThreadAdapter) Start() {
	a.lock.Lock()
	defer a.lock.Unlock()
	if a.started {
		return
	}
	a.started = true
	li, err := util.GetOrCreateOwnLog(a.api, a.threadID)
	if err != nil {
		log.Fatalf("error when getting/creating own log for thread %s: %v", a.threadID, err)
	}
	a.ownLogID = li.ID

	var wg sync.WaitGroup
	wg.Add(2)
	go a.threadToStore(&wg)
	go a.storeToThread(&wg)
	wg.Wait()
}

func (a *singleThreadAdapter) threadToStore(wg *sync.WaitGroup) {
	sub := a.api.Subscribe(tservopts.ThreadID(a.threadID))
	defer sub.Discard()
	wg.Done()
	for {
		select {
		case <-a.ctx.Done():
			log.Infof("cancelling thread event dispatch to store for thread %s since ctx was cancelled", a.threadID)
			return
		case rec, ok := <-sub.Channel():
			if !ok {
				log.Info("ending dispatch of events to store since thread channel was closed") // ToDo: reconsider action
				return
			}
			if rec.LogID() == a.ownLogID {
				continue // Ignore our own events since Store already dispatches to Store reducers
			}
			ctx, cancel := context.WithTimeout(context.Background(), fetchEventTimeout)

			event, err := threadcbor.EventFromRecord(ctx, a.api, rec.Value())
			if err != nil {
				log.Fatalf("error when getting event from record: %v", err) // ToDo: Buffer them and retry...
			}

			readKey, err := a.api.Store().ReadKey(a.threadID)
			if err != nil {
				log.Fatalf("error when getting read key for thread %s: %v", a.threadID, err)
			}
			if readKey == nil {
				log.Fatalf("read key not found for thread %s/%s", a.threadID, rec.LogID())
			}
			node, err := event.GetBody(ctx, a.api, readKey)
			if err != nil {
				log.Fatalf("error when getting body of event on thread %s/%s: %v", a.threadID, rec.LogID(), err)
			}
			storeEvent, err := a.store.eventFromBytes(node.RawData())
			if err != nil {
				log.Fatalf("error when unmarshaling event from bytes: %v", err)
			}
			log.Debugf("dispatching to store external new record: %s/%s", rec.ThreadID(), rec.LogID())
			if err := a.store.dispatch(storeEvent); err != nil {
				log.Fatal(err)
			}
			cancel()
		}
	}
}

func (a *singleThreadAdapter) storeToThread(wg *sync.WaitGroup) {
	l := a.store.localEventListen()
	defer l.Discard()
	wg.Done()

	for {
		select {
		case <-a.ctx.Done():
			log.Infof("cancelling sending store local events to own thread log for thread %s", a.threadID)
			return
		case event, ok := <-l.Channel():
			if !ok {
				log.Infof("ending sending store local event to own thread since channel was closed for thread %s", a.threadID)
				return
			}
			n, err := event.Node()
			if err != nil {
				log.Fatalf("error when generating node for own log for thread %s: %v", a.threadID, err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), addRecordTimeout)

			log.Debugf("adding new local store event to own log from entityid: %s", event.EntityID())
			_, err = a.api.AddRecord(ctx, a.threadID, n)
			if err != nil {
				log.Fatalf("error writing record: %v", err)
			}
			cancel()
		}
	}
}
