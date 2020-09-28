package net

import (
	"fmt"
	"strings"
	"testing"

	"github.com/ipfs/go-cid"
	util "github.com/ipfs/go-ipfs-util"
	"golang.org/x/exp/rand"
)

type mockRecord struct {
	id, prevID cid.Cid
}

func (r mockRecord) Cid() cid.Cid {
	return r.id
}

func (r mockRecord) PrevID() cid.Cid {
	return r.prevID
}

func TestNet_RecordSequenceConsistent(t *testing.T) {
	seqLen := 100
	seq := generateSequence(cid.Undef, seqLen)
	recs := newRecordSequence()

	// emulate random order with 2x redundancy
	seqCpy := make([]linkedRecord, 2*len(seq))
	copy(seqCpy[:len(seq)], seq)
	copy(seqCpy[len(seq):], seq)
	rand.Shuffle(len(seqCpy), func(i, j int) { seqCpy[i], seqCpy[j] = seqCpy[j], seqCpy[i] })

	for _, rec := range seqCpy {
		recs.Store(rec)
	}

	collected, ok := recs.List()
	if !ok {
		t.Error("cannot reconstruct consistent record sequence")
	}

	if !equalSequences(collected, seq) {
		t.Errorf("reconstructed sequence doesn't match original one\noriginal: %s\nrestored: %s",
			formatSequence(seq), formatSequence(collected))
	}
}

func TestNet_RecordSequenceGaps(t *testing.T) {
	seqLen := 100
	seq := generateSequence(cid.Undef, seqLen)
	recs := newRecordSequence()

	// two disjoint subsequences with a single missing record between
	s1 := seq[:seqLen/2]
	s2 := seq[seqLen/2+1:]

	for _, rec := range s1 {
		recs.Store(rec)
	}

	for _, rec := range s2 {
		recs.Store(rec)
	}

	if _, ok := recs.List(); ok {
		t.Errorf("record sequence with gaps unexpectedly reconstructed")
	}
}

func generateSequence(from cid.Cid, size int) []linkedRecord {
	var (
		prev = from
		seq  = make([]linkedRecord, size)
	)

	for i := 0; i < size; i++ {
		rec := generateRecord([]byte(fmt.Sprintf("record:%d", i)), prev)
		seq[i] = rec
		prev = rec.Cid()
	}

	return seq
}

func generateRecord(data []byte, prev cid.Cid) linkedRecord {
	mh := util.Hash(data)
	id := cid.NewCidV0(mh)
	return mockRecord{id: id, prevID: prev}
}

func equalSequences(s1, s2 []linkedRecord) bool {
	if len(s1) != len(s2) {
		return false
	}

	for i := 0; i < len(s1); i++ {
		if s1[i].Cid() != s2[i].Cid() || s1[i].PrevID() != s2[i].PrevID() {
			return false
		}
	}

	return true
}

func formatSequence(seq []linkedRecord) string {
	var (
		recs      = make([]string, len(seq))
		formatCID = func(id cid.Cid) string {
			if id == cid.Undef {
				return "Undef"
			}
			ir := id.String()
			return fmt.Sprintf("...%s", ir[len(ir)-6:])
		}
	)

	for i, rec := range seq {
		recs[i] = fmt.Sprintf("(Prev: %s, ID: %s)", formatCID(rec.PrevID()), formatCID(rec.Cid()))
	}

	return strings.Join(recs, " -> ")
}
