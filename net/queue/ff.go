package queue

import (
	"context"
	"hash/fnv"
	"math"
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
}

type threadQueue struct {
	index       map[thread.ID]*linkedOperation
	first, last *linkedOperation
	sync.Mutex
}

// Simple FIFO-queue with O(1)-operations.
func newThreadQueue() *threadQueue {
	return &threadQueue{index: make(map[thread.ID]*linkedOperation)}
}

// Add new call to the queue or replace existing one with lower priority.
func (q *threadQueue) Add(tid thread.ID, call PeerCall, priority int) bool {
	op, exist := q.index[tid]
	if !exist {
		// append new entry at the end
		op = &linkedOperation{
			tid:      tid,
			call:     call,
			priority: priority,
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
func (q *threadQueue) Pop() (PeerCall, thread.ID, bool) {
	if q.first == nil {
		return nil, thread.Undef, false
	}
	op := q.first
	q.first = op.next
	delete(q.index, op.tid)
	return op.call, op.tid, true
}

// Remove corresponding call if it was scheduled.
func (q *threadQueue) Remove(tid thread.ID) bool {
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

func (q *threadQueue) Size() int {
	return len(q.index)
}

/* Call queue implementation */

var _ CallQueue = (*ffQueue)(nil)

type ffQueue struct {
	peers    map[peer.ID]*threadQueue
	inflight map[uint64]struct{}
	poll     time.Duration
	timeout  time.Duration
	ctx      context.Context
	mx       sync.Mutex
}

// Fair FIFO-queue with isolated per-peer processing and adaptive invocation rate.
// Large number of simultaneously added calls may result in smoothed call bursts.
// At every moment only one call for the peer/thread pair could be in the queue.
// Scheduled operations could be replaced with a new ones based on the priority
// value (high-priority call replaces waiting one with lower value).
func NewFFQueue(
	ctx context.Context,
	pollingInterval time.Duration,
	processingTimeout time.Duration,
) *ffQueue {
	return &ffQueue{
		ctx:      ctx,
		poll:     pollingInterval,
		timeout:  processingTimeout,
		inflight: make(map[uint64]struct{}),
		peers:    make(map[peer.ID]*threadQueue),
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
	peerQueue, exist := q.peers[pid]
	if !exist {
		peerQueue = newThreadQueue()
		q.peers[pid] = peerQueue
		go q.pollQueue(pid, peerQueue)
	}
	q.mx.Unlock()

	peerQueue.Lock()
	defer peerQueue.Unlock()
	return peerQueue.Add(tid, call, priority)
}

func (q *ffQueue) Call(
	pid peer.ID,
	tid thread.ID,
	call PeerCall,
) error {
	q.mx.Lock()
	peerQueue, exist := q.peers[pid]
	q.mx.Unlock()

	if exist {
		// remove call previously scheduled for the thread
		peerQueue.Lock()
		peerQueue.Remove(tid)
		peerQueue.Unlock()
		log.Debugf("deschedule call to [%s/%s]: directly invoked", pid, tid)
	}

	return call(q.ctx, pid, tid)
}

func (q *ffQueue) pollQueue(pid peer.ID, tq *threadQueue) {
	var (
		slots = q.timeout / q.poll
		tick  = time.NewTicker(q.poll)
	)

	for {
		select {
		case <-q.ctx.Done():
			tick.Stop()
			return
		case <-tick.C:
			tq.Lock()
			if waiting := tq.Size(); waiting > 0 {
				// number of calls to be invoked on current tick (at least 1)
				var burstSize = int(math.Ceil(float64(waiting) / float64(slots)))
				for i := 0; i < burstSize; i++ {
					call, tid, ok := tq.Pop()
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
				}
			}
			tq.Unlock()
		}
	}
}

func hash(pid peer.ID, tid thread.ID) uint64 {
	var hasher = fnv.New64a()
	_, _ = hasher.Write([]byte(pid))
	_, _ = hasher.Write(tid.Bytes())
	return hasher.Sum64()
}
