package queue

import (
	"context"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/textileio/go-threads/core/thread"
)

var (
	// Size of incoming requests buffer
	InBufSize = 1

	// Size of packed threads buffer
	OutBufSize = 1
)

var _ ThreadPacker = (*threadPacker)(nil)

type (
	tEntry struct {
		tid   thread.ID
		added int64
	}

	request struct {
		pid   peer.ID
		tid   thread.ID
		added int64
	}

	threadPacker struct {
		ctx         context.Context
		peers       map[peer.ID][]tEntry
		input       chan request
		timeout     time.Duration
		maxPackSize int
	}
)

// Packer accumulates peer-related thread requests and packs it into the
// limited-size containers during time-window constrained by provided timeout.
func NewThreadPacker(ctx context.Context, maxPackSize int, timeout time.Duration) *threadPacker {
	return &threadPacker{
		peers:       make(map[peer.ID][]tEntry),
		input:       make(chan request, InBufSize),
		timeout:     timeout,
		maxPackSize: maxPackSize,
		ctx:         ctx,
	}
}

func (q *threadPacker) Add(pid peer.ID, tid thread.ID) {
	q.input <- request{
		pid:   pid,
		tid:   tid,
		added: time.Now().Unix(),
	}
}

func (q *threadPacker) Run() <-chan ThreadPack {
	var sink = make(chan ThreadPack, OutBufSize)

	go func() {
		tm := time.NewTicker(q.timeout)
		defer tm.Stop()

		for {
			select {
			case <-q.ctx.Done():
				for pid := range q.peers {
					q.drainPeerQueue(pid, sink)
				}
				close(sink)
				return

			case <-tm.C:
				// periodic check for inactive peer queues with overdue entries
				var now = time.Now().Unix()
				for pid, pq := range q.peers {
					if len(pq) > 0 && now-pq[0].added >= int64(q.timeout/time.Second) {
						q.drainPeerQueue(pid, sink)
					}
				}

			case req := <-q.input:
				var pq = q.peers[req.pid]
				pq = append(pq, tEntry{tid: req.tid, added: req.added})
				q.peers[req.pid] = pq
				if len(pq) >= q.maxPackSize || req.added-pq[0].added >= int64(q.timeout/time.Second) {
					// max size limit reached or pack is overdue
					q.drainPeerQueue(req.pid, sink)
				}
			}
		}
	}()

	return sink
}

func (q *threadPacker) drainPeerQueue(pid peer.ID, sink chan<- ThreadPack) {
	pq := q.peers[pid]
	if len(pq) == 0 {
		return
	}

	pack := ThreadPack{
		Peer:    pid,
		Threads: make([]thread.ID, len(pq)),
	}
	for i := 0; i < len(pq); i++ {
		pack.Threads[i] = pq[i].tid
	}
	sink <- pack

	// reset queue
	q.peers[pid] = pq[:0]
}
