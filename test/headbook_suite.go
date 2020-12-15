package test

import (
	"sort"
	"strconv"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	pt "github.com/libp2p/go-libp2p-core/test"
	mh "github.com/multiformats/go-multihash"
	core "github.com/textileio/go-threads/core/logstore"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/go-threads/util"
)

var headBookSuite = map[string]func(hb core.HeadBook) func(*testing.T){
	"AddGetHeads": testHeadBookAddHeads,
	"SetGetHeads": testHeadBookSetHeads,
	"ClearHeads":  testHeadBookClearHeads,
	"ExportHeads": testHeadBookExport,
	"ThreadEdge":  testHeadBookThreadEdge,
}

type HeadBookFactory func() (core.HeadBook, func())

func HeadBookTest(t *testing.T, factory HeadBookFactory) {
	for name, test := range headBookSuite {
		// Create a new book.
		hb, closeFunc := factory()

		// Run the test.
		t.Run(name, test(hb))

		// Cleanup.
		if closeFunc != nil {
			closeFunc()
		}
	}
}

func testHeadBookAddHeads(hb core.HeadBook) func(t *testing.T) {
	return func(t *testing.T) {
		var (
			numLogs   = 2
			numHeads  = 3
			tid, logs = genHeads(numLogs, numHeads)
		)

		for lid, heads := range logs {
			if stored, err := hb.Heads(tid, lid); err != nil || len(stored) > 0 {
				t.Error("expected heads to be empty on init without errors")
			}

			if err := hb.AddHeads(tid, lid, heads); err != nil {
				t.Fatalf("error when adding heads: %v", err)
			}
		}

		for lid, expected := range logs {
			heads, err := hb.Heads(tid, lid)
			if err != nil {
				t.Fatalf("error while getting heads: %v", err)
			}

			if !equalHeads(expected, heads) {
				t.Fatalf("heads not equal, expected: %v, actual: %v", expected, heads)
			}
		}
	}
}

func testHeadBookSetHeads(hb core.HeadBook) func(t *testing.T) {
	return func(t *testing.T) {
		var (
			numLogs   = 2
			numHeads  = 3
			tid, logs = genHeads(numLogs, numHeads)
		)

		for lid, heads := range logs {
			if stored, err := hb.Heads(tid, lid); err != nil || len(stored) > 0 {
				t.Error("expected heads to be empty on init without errors")
			}

			if err := hb.SetHeads(tid, lid, heads); err != nil {
				t.Fatalf("error when adding heads: %v", err)
			}
		}

		for lid, expected := range logs {
			heads, err := hb.Heads(tid, lid)
			if err != nil {
				t.Fatalf("error while getting heads: %v", err)
			}

			if !equalHeads(expected, heads) {
				t.Fatalf("heads not equal, expected: %v, actual: %v", expected, heads)
			}
		}
	}
}

func testHeadBookClearHeads(hb core.HeadBook) func(t *testing.T) {
	return func(t *testing.T) {
		var (
			numLogs   = 2
			numHeads  = 2
			tid, logs = genHeads(numLogs, numHeads)
		)

		for lid, heads := range logs {
			if stored, err := hb.Heads(tid, lid); err != nil || len(stored) > 0 {
				t.Error("expected heads to be empty on init without errors")
			}

			if err := hb.AddHeads(tid, lid, heads); err != nil {
				t.Fatalf("error when adding heads: %v", err)
			}
		}

		for lid, expected := range logs {
			heads, err := hb.Heads(tid, lid)
			if err != nil {
				t.Fatalf("error while getting heads: %v", err)
			}

			if !equalHeads(expected, heads) {
				t.Fatalf("heads not equal, expected: %v, actual: %v", expected, heads)
			}
		}

		for lid := range logs {
			if err := hb.ClearHeads(tid, lid); err != nil {
				t.Fatalf("error when clearing heads: %v", err)
			}
		}

		for lid := range logs {
			heads, err := hb.Heads(tid, lid)
			if err != nil {
				t.Fatalf("error while getting heads: %v", err)
			}

			if len(heads) > 0 {
				t.Fatalf("heads not empty after clear")
			}
		}
	}
}

func testHeadBookThreadEdge(hb core.HeadBook) func(t *testing.T) {
	return func(t *testing.T) {
		var (
			numLogs  = 3
			numHeads = 2

			genHeadSets = func(cases int) (thread.ID, []map[peer.ID][]cid.Cid) {
				tid, logs := genHeads(numLogs, cases*numHeads)
				splitted := make([]map[peer.ID][]cid.Cid, cases)
				for i := 0; i < cases; i++ {
					hs := make(map[peer.ID][]cid.Cid)
					for lid, cids := range logs {
						hs[lid] = append(hs[lid], cids[i*numHeads:(i+1)*numHeads]...)
					}
					splitted[i] = hs
				}
				return tid, splitted
			}

			equalEdges = func(edge uint64, logs map[peer.ID][]cid.Cid) bool {
				var heads []util.LogHead
				for lid, cids := range logs {
					for _, c := range cids {
						heads = append(heads, util.LogHead{LogID: lid, Head: c})
					}
				}
				return util.ComputeThreadEdge(heads) == edge
			}

			// generate 3 sets of heads
			tid, heads          = genHeadSets(3)
			hSet1, hSet2, hSet3 = heads[0], heads[1], heads[2]
		)

		if _, err := hb.ThreadEdge(tid); err != core.ErrThreadNotFound {
			t.Error("expected to get error on retrieving non-existing thread's edge")
		}
		for lid, hs := range hSet1 {
			if stored, err := hb.Heads(tid, lid); err != nil || len(stored) > 0 {
				t.Error("expected heads to be empty on init without errors")
			}
			if err := hb.SetHeads(tid, lid, hs); err != nil {
				t.Fatalf("error when adding heads: %v", err)
			}
		}

		edge1, err := hb.ThreadEdge(tid)
		if err != nil {
			t.Errorf("error while getting thread's edge: %v", err)
		}

		// set new heads
		for lid, hs := range hSet2 {
			if err := hb.SetHeads(tid, lid, hs); err != nil {
				t.Fatalf("error when adding heads: %v", err)
			}
		}
		edge2, err := hb.ThreadEdge(tid)
		if err != nil {
			t.Errorf("error while getting thread's edge: %v", err)
		}
		if edge1 == edge2 {
			t.Error("edges should not be equal after setting new heads")
		}
		if !equalEdges(edge2, hSet2) {
			t.Error("stored and manually computed edges doesn't match")
		}

		// add some more heads
		for lid, hs := range hSet3 {
			if err := hb.AddHeads(tid, lid, hs); err != nil {
				t.Fatalf("error when adding heads: %v", err)
			}
		}
		edge3, err := hb.ThreadEdge(tid)
		if err != nil {
			t.Errorf("error while getting thread's edge: %v", err)
		}
		if edge3 == edge1 || edge3 == edge2 {
			t.Error("edges should not be equal after setting/adding new heads")
		}
	}
}

func testHeadBookExport(hb core.HeadBook) func(t *testing.T) {
	return func(t *testing.T) {
		var (
			numLogs   = 2
			numHeads  = 3
			tid, logs = genHeads(numLogs, numHeads)
		)

		for lid, heads := range logs {
			if stored, err := hb.Heads(tid, lid); err != nil || len(stored) > 0 {
				t.Error("expected heads to be empty on init without errors")
			}

			if err := hb.AddHeads(tid, lid, heads); err != nil {
				t.Fatalf("error when adding heads: %v", err)
			}
		}

		dump, err := hb.DumpHeads()
		if err != nil {
			t.Fatalf("error dumping headbook: %v", err)
		}

		// clear storage explicitly to ensure it will be empty
		for lid := range logs {
			if err := hb.ClearHeads(tid, lid); err != nil {
				t.Fatalf("error when clearing heads: %v", err)
			}
		}

		if err := hb.RestoreHeads(dump); err != nil {
			t.Fatalf("error restoring headbook: %v", err)
		}

		for lid, expected := range logs {
			heads, err := hb.Heads(tid, lid)
			if err != nil {
				t.Fatalf("error while getting heads: %v", err)
			}

			if !equalHeads(expected, heads) {
				t.Fatalf("heads not equal, expected: %v, actual: %v", expected, heads)
			}
		}
	}
}

var logHeadbookBenchmarkSuite = map[string]func(hb core.HeadBook) func(*testing.B){
	"Heads":      benchmarkHeads,
	"AddHeads":   benchmarkAddHeads,
	"SetHeads":   benchmarkSetHeads,
	"ClearHeads": benchmarkClearHeads,
}

func BenchmarkHeadBook(b *testing.B, factory HeadBookFactory) {
	ordernames := make([]string, 0, len(logHeadbookBenchmarkSuite))
	for name := range logHeadbookBenchmarkSuite {
		ordernames = append(ordernames, name)
	}
	sort.Strings(ordernames)
	for _, name := range ordernames {
		bench := logHeadbookBenchmarkSuite[name]
		hb, closeFunc := factory()

		b.Run(name, bench(hb))

		if closeFunc != nil {
			closeFunc()
		}
	}
}

func benchmarkHeads(hb core.HeadBook) func(*testing.B) {
	return func(b *testing.B) {
		var (
			numLogs   = 1
			numHeads  = 1
			tid, logs = genHeads(numLogs, numHeads)
		)

		for lid, heads := range logs {
			_ = hb.AddHeads(tid, lid, heads)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			for lid := range logs {
				_, _ = hb.Heads(tid, lid)
			}
		}
	}
}

func benchmarkAddHeads(hb core.HeadBook) func(*testing.B) {
	return func(b *testing.B) {
		var (
			numLogs   = 1
			numHeads  = 1
			tid, logs = genHeads(numLogs, numHeads)
		)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			for lid, heads := range logs {
				_ = hb.AddHeads(tid, lid, heads)
			}
		}
	}
}

func benchmarkSetHeads(hb core.HeadBook) func(*testing.B) {
	return func(b *testing.B) {
		var (
			numLogs   = 1
			numHeads  = 1
			tid, logs = genHeads(numLogs, numHeads)
		)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			for lid, heads := range logs {
				_ = hb.SetHeads(tid, lid, heads)
			}
		}
	}
}

func benchmarkClearHeads(hb core.HeadBook) func(*testing.B) {
	return func(b *testing.B) {
		var (
			numLogs   = 1
			numHeads  = 1
			tid, logs = genHeads(numLogs, numHeads)
		)

		for lid, heads := range logs {
			_ = hb.SetHeads(tid, lid, heads)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			for lid := range logs {
				_ = hb.ClearHeads(tid, lid)
			}
		}
	}
}

func genHeads(numLogs, numHeads int) (thread.ID, map[peer.ID][]cid.Cid) {
	var (
		logs = make(map[peer.ID][]cid.Cid)
		tid  = thread.NewIDV1(thread.Raw, 32)
	)

	for i := 0; i < numLogs; i++ {
		_, pub, _ := pt.RandTestKeyPair(crypto.RSA, crypto.MinRsaKeyBits)
		lid, _ := peer.IDFromPublicKey(pub)

		heads := make([]cid.Cid, numHeads)
		for j := 0; j < numHeads; j++ {
			hash, _ := mh.Encode([]byte("h:"+strconv.Itoa(i)+":"+strconv.Itoa(j)), mh.SHA2_256)
			heads[j] = cid.NewCidV1(cid.DagCBOR, hash)
		}

		logs[lid] = heads
	}

	return tid, logs
}

func equalHeads(h1, h2 []cid.Cid) bool {
	if len(h1) != len(h2) {
		return false
	}

	sort.Slice(h1, func(i, j int) bool { return h1[i].String() < h1[j].String() })
	sort.Slice(h2, func(i, j int) bool { return h2[i].String() < h2[j].String() })

	for i := 0; i < len(h1); i++ {
		if !h1[i].Equals(h2[i]) {
			return false
		}
	}

	return true
}
