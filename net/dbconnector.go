package net

import (
	"context"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
	coredb "github.com/textileio/go-threads/core/db"
	core "github.com/textileio/go-threads/core/net"
	"github.com/textileio/go-threads/core/thread"
)

const (
	addRecordTimeout  = time.Second * 10
	fetchEventTimeout = time.Second * 15
)

// dbConnector connects db to net.
type dbConnector struct {
	net        *net
	db         coredb.NetDB
	threadID   thread.ID
	ownLogID   peer.ID
	closeChan  chan struct{}
	goRoutines sync.WaitGroup

	lock    sync.Mutex
	started bool
	closed  bool
}

// ConnectDB returns a connector that connects a db to a thread.
func (n *net) ConnectDB(db coredb.NetDB, id thread.ID) coredb.Connector {
	return &dbConnector{
		net:       n,
		threadID:  id,
		db:        db,
		closeChan: make(chan struct{}),
	}
}

// Close closes the db thread and stops listening both directions of thread<->db.
func (a *dbConnector) Close() error {
	a.lock.Lock()
	defer a.lock.Unlock()
	if a.closed {
		return nil
	}
	a.closed = true
	close(a.closeChan)
	a.goRoutines.Wait()
	return nil
}

// Start starts connection from DB to Service, and viceversa.
func (a *dbConnector) Start() {
	a.lock.Lock()
	defer a.lock.Unlock()
	if a.started {
		return
	}
	a.started = true
	li, err := a.net.getThread(context.Background(), a.threadID)
	if err != nil {
		log.Fatalf("error getting thread %s: %v", a.threadID, err)
	}
	if ownLog := li.GetOwnLog(); ownLog != nil {
		a.ownLogID = ownLog.ID
	} else {
		log.Fatalf("error getting own log for thread %s: %v", a.threadID, err)
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go a.threadToDB(&wg)
	go a.dbToThread(&wg)
	wg.Wait()
	a.goRoutines.Add(2)
}

func (a *dbConnector) threadToDB(wg *sync.WaitGroup) {
	defer a.goRoutines.Done()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	filter := map[thread.ID]struct{}{a.threadID: {}}
	sub, err := a.net.subscribe(ctx, filter)
	if err != nil {
		log.Fatalf("error getting thread subscription: %v", err)
	}
	wg.Done()
	for {
		select {
		case <-a.closeChan:
			log.Debugf("closing thread-to-db flow on thread %s", a.threadID)
			return
		case rec, ok := <-sub:
			if !ok {
				log.Debug("notification channel closed, not listening to external changes anymore")
				return
			}
			info, err := a.net.getThread(ctx, a.threadID)
			if err != nil {
				log.Fatalf("error when getting info for thread %s: %v", a.threadID, err)
			}
			if !info.Key.CanRead() {
				log.Fatalf("read key not found for thread %s/%s", info.ID, rec.LogID())
			}

			if err = a.db.HandleNetRecord(rec, info, a.ownLogID, fetchEventTimeout); err != nil {
				log.Fatal(err)
			}
		}
	}
}

func (a *dbConnector) dbToThread(wg *sync.WaitGroup) {
	defer a.goRoutines.Done()
	l := a.db.LocalEventListen()
	defer l.Discard()
	wg.Done()

	for {
		select {
		case <-a.closeChan:
			log.Debugf("closing db-to-thread flow on thread %s", a.threadID)
			return
		case event, ok := <-l.Channel():
			if !ok {
				log.Errorf("ending sending db local event to own thread since channel was closed for thread %s", a.threadID)
				return
			}
			ctx, cancel := context.WithTimeout(context.Background(), addRecordTimeout)
			if _, err := a.net.CreateRecord(ctx, a.threadID, event.Node, core.WithThreadAuth(event.Credentials)); err != nil {
				log.Fatalf("error writing record: %v", err)
			}
			cancel()
		}
	}
}
