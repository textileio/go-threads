package net

import (
	"fmt"
	"sync"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/peer"
	core "github.com/textileio/go-threads/core/net"
)

type linkedRecord interface {
	Cid() cid.Cid
	PrevID() cid.Cid
}

// Collector maintains an ordered list of records from multiple sources (thread-safe)
type recordCollector struct {
	rs   map[peer.ID]*recordSequence
	lock sync.Mutex
}

func newRecordCollector() *recordCollector {
	return &recordCollector{rs: make(map[peer.ID]*recordSequence)}
}

// Store the record of the log.
func (r *recordCollector) Store(lid peer.ID, rec core.Record) {
	r.lock.Lock()
	defer r.lock.Unlock()

	seq, found := r.rs[lid]
	if !found {
		seq = newRecordSequence()
		r.rs[lid] = seq
	}

	seq.Store(rec)
}

// List all previously stored records in a proper order if the latter exists.
func (r *recordCollector) List() (map[peer.ID][]core.Record, error) {
	r.lock.Lock()
	defer r.lock.Unlock()

	logSeqs := make(map[peer.ID][]core.Record, len(r.rs))
	for id, seq := range r.rs {
		ordered, ok := seq.List()
		if !ok {
			return nil, fmt.Errorf("disjoint record sequence in log %s", id)
		}
		casted := make([]core.Record, len(ordered))
		for i := 0; i < len(ordered); i++ {
			casted[i] = ordered[i].(core.Record)
		}
		logSeqs[id] = casted
	}

	return logSeqs, nil
}

// Not a thread-safe structure
type recordSequence struct {
	fragments [][]linkedRecord
	set       map[cid.Cid]struct{}
}

func newRecordSequence() *recordSequence {
	return &recordSequence{set: make(map[cid.Cid]struct{})}
}

func (s *recordSequence) Store(rec linkedRecord) {
	// verify if record is already contained in some fragment
	if _, found := s.set[rec.Cid()]; found {
		return
	}
	s.set[rec.Cid()] = struct{}{}

	// now try to find a sequence to be attached to
	for i, fragment := range s.fragments {
		if fragment[len(fragment)-1].Cid() == rec.PrevID() {
			s.fragments[i] = append(fragment, rec)
			return
		} else if fragment[0].PrevID() == rec.Cid() {
			s.fragments[i] = append([]linkedRecord{rec}, fragment...)
			return
		}
	}

	// start a new fragment
	s.fragments = append(s.fragments, []linkedRecord{rec})
}

// return reconstructed sequence and success flag
func (s *recordSequence) List() ([]linkedRecord, bool) {
LOOP:
	// avoid recursion as sequences could be pretty large
	for {
		if len(s.fragments) == 1 {
			return s.fragments[0], true
		}

		// take a fragment ...
		fragment := s.fragments[0]
		fHead, fTail := fragment[len(fragment)-1], fragment[0]

		// ... and try to compose it with another one
		for i, candidate := range s.fragments[1:] {
			cHead, cTail := candidate[len(candidate)-1], candidate[0]
			// index shifted by slicing
			i += 1

			if fHead.Cid() == cTail.PrevID() {
				// composition: (tail) <- fragment <- candidate <- (head)
				s.fragments[0] = append(fragment, candidate...)
				s.fragments = append(s.fragments[:i], s.fragments[i+1:]...)
				continue LOOP

			} else if fTail.PrevID() == cHead.Cid() {
				// composition: (tail) <- candidate <- fragment <- (head)
				s.fragments[i] = append(candidate, fragment...)
				s.fragments = s.fragments[1:]
				continue LOOP
			}
		}

		// no composition found, hence there are at least two disjoint fragments
		return nil, false
	}
}
