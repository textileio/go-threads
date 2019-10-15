package tstoreds

import (
	"context"
	"fmt"
	"time"

	query "github.com/ipfs/go-datastore/query"
	pb "github.com/textileio/go-textile-threads/pb"
)

var (
	purgeStoreQuery = query.Query{
		Prefix:   threadBookBase.String(),
		Orders:   []query.Order{query.OrderByKey{}},
		KeysOnly: false,
	}
)

// dsAddrBookGc is responsible for garbage collection in a datastore-backed address book.
type dsAddrBookGc struct {
	ctx       context.Context
	ab        *DsAddrBook
	running   chan struct{}
	purgeFunc func()
}

func newAddressBookGc(ctx context.Context, ab *DsAddrBook) (*dsAddrBookGc, error) {
	if ab.opts.GCPurgeInterval < 0 {
		return nil, fmt.Errorf("negative GC purge interval provided: %s", ab.opts.GCPurgeInterval)
	}

	gc := &dsAddrBookGc{
		ctx:     ctx,
		ab:      ab,
		running: make(chan struct{}, 1),
	}

	gc.purgeFunc = gc.purgeStore

	// do not start GC timers if purge is disabled; this GC can only be triggered manually.
	if ab.opts.GCPurgeInterval > 0 {
		gc.ab.childrenDone.Add(1)
		go gc.background()
	}

	return gc, nil
}

// gc prunes expired addresses from the datastore at regular intervals. It should be spawned as a goroutine.
func (gc *dsAddrBookGc) background() {
	defer gc.ab.childrenDone.Done()

	select {
	case <-time.After(gc.ab.opts.GCInitialDelay):
	case <-gc.ab.ctx.Done():
		// yield if we have been cancelled/closed before the delay elapses.
		return
	}

	purgeTimer := time.NewTicker(gc.ab.opts.GCPurgeInterval)
	defer purgeTimer.Stop()

	for {
		select {
		case <-purgeTimer.C:
			gc.purgeFunc()
		case <-gc.ctx.Done():
			return
		}
	}
}

func (gc *dsAddrBookGc) purgeStore() {
	select {
	case gc.running <- struct{}{}:
		defer func() { <-gc.running }()
	default:
		// yield if lookahead is running.
		return
	}

	batch, err := newCyclicBatch(gc.ab.ds, defaultOpsPerCyclicBatch)
	if err != nil {
		log.Warningf("failed while creating batch to purge GC entries: %v", err)
	}

	results, err := gc.ab.ds.Query(purgeStoreQuery)
	if err != nil {
		log.Warningf("failed while opening iterator: %v", err)
		return
	}
	defer results.Close()

	record := &addrsRecord{AddrBookRecord: &pb.AddrBookRecord{}} // empty record to reuse and avoid allocs.
	// keys: 	/thread/addrs/<thread ID b32>
	for result := range results.Next() {
		record.Reset()
		if err = record.Unmarshal(result.Value); err != nil {
			log.Warningf("key %v has an unmarshable record", result.Key)
			continue
		}

		if !record.clean() {
			continue
		}

		id := genCacheKey(record.ThreadID.ID, record.PeerID.ID)
		if err := record.flush(batch); err != nil {
			log.Warningf("failed to flush entry modified by GC for peer: &v, err: %v", id, err)
		}
		gc.ab.cache.Remove(id)
	}

	if err = batch.Commit(); err != nil {
		log.Warningf("failed to commit GC purge batch: %v", err)
	}
}
