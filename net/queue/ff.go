package queue

import (
	"context"
	"hash/fnv"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/textileio/go-threads/core/thread"
)

type linkedOperation struct {
	prev, next *linkedOperation
	tid        thread.ID
	call       PeerCall
	priority   int
	created    int64
}

type peerQueue struct {
	index       map[thread.ID]*linkedOperation
	first, last *linkedOperation
	sync.Mutex
}

// Simple FIFO-queue with O(1)-operations.
func newPeerQueue() *peerQueue {
	return &peerQueue{index: make(map[thread.ID]*linkedOperation)}
}

// Add new call to the queue or replace existing one with lower priority.
func (q *peerQueue) Add(tid thread.ID, call PeerCall, priority int) bool {
	op, exist := q.index[tid]
	if !exist {
		// append new entry at the end
		op = &linkedOperation{
			tid:      tid,
			call:     call,
			priority: priority,
			created:  time.Now().Unix(),
		}
		if q.last == nil {
			// empty queue
			q.first = op
			q.last = op
		} else {
			q.last.next = op
			op.prev = q.last
			q.last = op
		}
		q.index[tid] = op
		return true
	}

	if op.priority < priority {
		// just replace the call
		op.call = call
	}
	return false
}

// Return previously added calls in FIFO order.
func (q *peerQueue) Pop() (PeerCall, thread.ID, int64, bool) {
	if q.first == nil {
		return nil, thread.Undef, 0, false
	}
	op := q.first

	q.first = op.next
	if q.first != nil {
		q.first.prev = nil
	} else {
		q.last = nil
	}

	delete(q.index, op.tid)
	return op.call, op.tid, op.created, true
}

// Remove corresponding call if it was scheduled.
func (q *peerQueue) Remove(tid thread.ID) bool {
	op, exist := q.index[tid]
	if !exist {
		return false
	}

	switch {
	case q.last == op && q.first == op:
		// single operation - empty the queue
		q.first = nil
		q.last = nil
	case q.first == op:
		// first operation
		next := op.next
		next.prev = nil
		q.first = next
	case q.last == op:
		// last operation
		prev := op.prev
		prev.next = nil
		q.last = prev
	default:
		prev, next := op.prev, op.next
		prev.next = next
		next.prev = prev
	}
	delete(q.index, tid)
	return true
}

func (q *peerQueue) Size() int {
	return len(q.index)
}

/* Call queue implementation */

var _ CallQueue = (*ffQueue)(nil)

type ffQueue struct {
	peers    map[peer.ID]*peerQueue
	inflight map[uint64]struct{}
	poll     time.Duration
	deadline time.Duration
	ctx      context.Context
	mx       sync.Mutex
}

// Fair FIFO-queue with isolated per-peer processing and adaptive invocation rate.
// Queue is polled with specified frequency and every scheduled call expected to be
// spawned until its deadline. At every moment only one call for the peer/thread
// pair exists in the queue. Scheduled operations could be replaced with a new ones
// based on the priority value (new higher-priority call replaces waiting one).
func NewFFQueue(
	ctx context.Context,
	pollInterval time.Duration,
	spawnDeadline time.Duration,
) *ffQueue {
	return &ffQueue{
		ctx:      ctx,
		poll:     pollInterval,
		deadline: spawnDeadline,
		inflight: make(map[uint64]struct{}),
		peers:    make(map[peer.ID]*peerQueue),
	}
}

func (q *ffQueue) Schedule(
	pid peer.ID,
	tid thread.ID,
	priority int,
	call PeerCall,
) bool {
	h := hash(pid, tid)
	q.mx.Lock()
	if _, inflight := q.inflight[h]; inflight {
		q.mx.Unlock()
		log.Debugf("skip call to [%s/%s]: in-flight", pid, tid)
		return false
	}
	pq, exist := q.peers[pid]
	if !exist {
		pq = newPeerQueue()
		q.peers[pid] = pq
		go q.pollQueue(pid, pq)
	}
	q.mx.Unlock()

	pq.Lock()
	defer pq.Unlock()
	return pq.Add(tid, call, priority)
}

func (q *ffQueue) Call(
	pid peer.ID,
	tid thread.ID,
	call PeerCall,
) error {
	h := hash(pid, tid)
	q.mx.Lock()
	pq, exist := q.peers[pid]
	q.inflight[h] = struct{}{}
	q.mx.Unlock()

	if exist {
		pq.Lock()
		removed := pq.Remove(tid)
		pq.Unlock()
		if removed {
			log.Debugf("deschedule call to [%s/%s]: directly invoked", pid, tid)
		}
	}

	err := call(q.ctx, pid, tid)
	q.mx.Lock()
	delete(q.inflight, h)
	q.mx.Unlock()
	return err
}

func (q *ffQueue) pollQueue(pid peer.ID, pq *peerQueue) {
	var tick = time.NewTicker(q.poll)

	for {
		select {
		case <-q.ctx.Done():
			tick.Stop()
			return

		case <-tick.C:
			pq.Lock()
			// every call scheduled before this moment is overdue now and should be spawned immediately
			var deadlineBound = time.Now().Add(-q.deadline).Unix()
			for waiting := pq.Size(); waiting > 0; waiting-- {
				call, tid, created, ok := pq.Pop()
				if !ok {
					break
				}

				go func() {
					var h = hash(pid, tid)

					// set in-flight status
					q.mx.Lock()
					q.inflight[h] = struct{}{}
					q.mx.Unlock()

					// make a call
					if err := call(q.ctx, pid, tid); err != nil {
						log.Errorf("call to [%s/%s] failed: %v", pid, tid, err)
					}

					// clear in-flight status
					q.mx.Lock()
					delete(q.inflight, h)
					q.mx.Unlock()
				}()

				// Ok, so whats going on. Here we have an underlying FIFO-queue containing every scheduled
				// call (associated with the moment it was added) in the linked list. There is also a
				// predefined parameter 'deadline' - every scheduled call should be spawned during this
				// period, though it's not a hard deadline really, but a desired property. And moreover,
				// we want it to be as uniform over time as possible in the presence of non-stationary
				// process of new call arrivals to the queue.
				// On every polling iteration we repeatedly make a decision: pop and spawn next call from
				// the queue or wait until the next iteration. It'd be too expensive to obtain precise
				// deadline distribution from the large queue, but since it's FIFO at least we know that
				// deadlines of remaining calls are in the interval '[created - deadlineBound; deadline)'.
				// And, of course, number of calls waiting in the queue is known, as well.
				// Not much information, but we can use some probabilistic model for polling decisions
				// and it may help to avoid undesirable synchronization with other queues. Implemented
				// model doesn't have solid theoretical underpinning, rather it's more an empirical one.
				// It's based on two assumptions about call popping probability:
				// 1) it should be inversely proportional to the worst case deadline (when all calls
				//    waiting in the queue were added at the same time), and
				// 2) it should grow sublinear depending on a current queue size.
				// Despite being the best model known to me so far in terms of smooth operation and
				// meeting deadlines in general, nevertheless it's far from perfect. So if you are
				// aware of any better approach - please, contribute it!

				if remainIters := int(float64(created-deadlineBound) / q.poll.Seconds()); remainIters > 0 &&
					rand.Float64() > math.Sqrt(3*float64(waiting))/float64(remainIters) {
					break
				}
			}
			pq.Unlock()
		}
	}
}

func hash(pid peer.ID, tid thread.ID) uint64 {
	var hasher = fnv.New64a()
	_, _ = hasher.Write([]byte(pid))
	_, _ = hasher.Write(tid.Bytes())
	return hasher.Sum64()
}
