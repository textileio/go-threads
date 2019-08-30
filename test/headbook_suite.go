package test

import (
	"sort"
	"strconv"
	"testing"

	"github.com/ipfs/go-cid"
	ic "github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	pt "github.com/libp2p/go-libp2p-core/test"
	mh "github.com/multiformats/go-multihash"
	"github.com/textileio/go-textile-core/thread"
	tstore "github.com/textileio/go-textile-core/threadstore"
)

var logHeadBookSuite = map[string]func(hb tstore.LogHeadBook) func(*testing.T){
	"AddGetHeads": testHeadBookAddHeads,
	"SetGetHeads": testHeadBookSetHeads,
	"ClearHeads":  testHeadBookClearHeads,
}

type LogHeadBookFactory func() (tstore.LogHeadBook, func())

func LogHeadBookTest(t *testing.T, factory LogHeadBookFactory) {
	for name, test := range logHeadBookSuite {
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

func testHeadBookAddHeads(hb tstore.LogHeadBook) func(t *testing.T) {
	return func(t *testing.T) {
		tid := thread.NewIDV1(thread.Raw, 24)

		_, pub, _ := pt.RandTestKeyPair(ic.RSA, 512)
		p, _ := peer.IDFromPublicKey(pub)

		if heads := hb.LogHeads(tid, p); len(heads) > 0 {
			t.Error("expected heads to be empty on init")
		}

		heads := make([]cid.Cid, 0)
		for i := 0; i < 2; i++ {
			hash, _ := mh.Encode([]byte("foo"+strconv.Itoa(i)), mh.SHA2_256)
			head := cid.NewCidV1(cid.DagCBOR, hash)

			hb.AddLogHeads(tid, p, []cid.Cid{head})
			heads = append(heads, head)
		}

		hbHeads := hb.LogHeads(tid, p)
		for _, h := range heads {
			var found bool
			for _, b := range hbHeads {
				if b == h {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("head %s not found in book", h.String())
			}
		}
	}
}

func testHeadBookSetHeads(hb tstore.LogHeadBook) func(t *testing.T) {
	return func(t *testing.T) {
		tid := thread.NewIDV1(thread.Raw, 24)

		_, pub, _ := pt.RandTestKeyPair(ic.RSA, 512)
		p, _ := peer.IDFromPublicKey(pub)

		if heads := hb.LogHeads(tid, p); len(heads) > 0 {
			t.Error("expected heads to be empty on init")
		}

		heads := make([]cid.Cid, 0)
		for i := 0; i < 2; i++ {
			hash, _ := mh.Encode([]byte("foo"+strconv.Itoa(i)), mh.SHA2_256)
			head := cid.NewCidV1(cid.DagCBOR, hash)
			heads = append(heads, head)
		}
		hb.SetLogHeads(tid, p, heads)

		hbHeads := hb.LogHeads(tid, p)
		for _, h := range heads {
			var found bool
			for _, b := range hbHeads {
				if b == h {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("head %s not found in book", h.String())
			}
		}
	}
}

func testHeadBookClearHeads(hb tstore.LogHeadBook) func(t *testing.T) {
	return func(t *testing.T) {
		tid := thread.NewIDV1(thread.Raw, 24)

		_, pub, _ := pt.RandTestKeyPair(ic.RSA, 512)
		p, _ := peer.IDFromPublicKey(pub)

		if heads := hb.LogHeads(tid, p); len(heads) > 0 {
			t.Error("expected heads to be empty on init")
		}

		heads := make([]cid.Cid, 0)
		for i := 0; i < 2; i++ {
			hash, _ := mh.Encode([]byte("foo"+strconv.Itoa(i)), mh.SHA2_256)
			head := cid.NewCidV1(cid.DagCBOR, hash)

			hb.AddLogHead(tid, p, head)
			heads = append(heads, head)
		}

		len1 := len(hb.LogHeads(tid, p))
		if len1 != 2 {
			t.Errorf("incorrect heads length %d", len1)
		}

		hb.ClearLogHeads(tid, p)

		len2 := len(hb.LogHeads(tid, p))
		if len2 != 0 {
			t.Errorf("incorrect heads length %d", len2)
		}
	}
}

var logHeadbookBenchmarkSuite = map[string]func(hb tstore.LogHeadBook) func(*testing.B){
	"LogHeads":      benchmarkHeads,
	"AddLogHeads":   benchmarkAddHeads,
	"SetLogHeads":   benchmarkSetHeads,
	"ClearLogHeads": benchmarkClearHeads,
}

func BenchmarkLogHeadBook(b *testing.B, factory LogHeadBookFactory) {
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

func benchmarkHeads(hb tstore.LogHeadBook) func(*testing.B) {
	return func(b *testing.B) {
		tid := thread.NewIDV1(thread.Raw, 24)

		_, pub, err := pt.RandTestKeyPair(ic.RSA, 512)
		if err != nil {
			b.Error(err)
		}

		id, err := peer.IDFromPublicKey(pub)
		if err != nil {
			b.Error(err)
		}

		hash, _ := mh.Encode([]byte("foo"), mh.SHA2_256)
		head := cid.NewCidV1(cid.DagCBOR, hash)

		hb.AddLogHead(tid, id, head)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			hb.LogHeads(tid, id)
		}
	}
}

func benchmarkAddHeads(hb tstore.LogHeadBook) func(*testing.B) {
	return func(b *testing.B) {
		tid := thread.NewIDV1(thread.Raw, 24)

		_, pub, err := pt.RandTestKeyPair(ic.RSA, 512)
		if err != nil {
			b.Error(err)
		}

		id, err := peer.IDFromPublicKey(pub)
		if err != nil {
			b.Error(err)
		}

		hash, _ := mh.Encode([]byte("foo"), mh.SHA2_256)
		head := cid.NewCidV1(cid.DagCBOR, hash)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			hb.AddLogHeads(tid, id, []cid.Cid{head})
		}
	}
}

func benchmarkSetHeads(hb tstore.LogHeadBook) func(*testing.B) {
	return func(b *testing.B) {
		tid := thread.NewIDV1(thread.Raw, 24)

		_, pub, err := pt.RandTestKeyPair(ic.RSA, 512)
		if err != nil {
			b.Error(err)
		}

		id, err := peer.IDFromPublicKey(pub)
		if err != nil {
			b.Error(err)
		}

		hash, _ := mh.Encode([]byte("foo"), mh.SHA2_256)
		head := cid.NewCidV1(cid.DagCBOR, hash)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			hb.SetLogHeads(tid, id, []cid.Cid{head})
		}
	}
}

func benchmarkClearHeads(hb tstore.LogHeadBook) func(*testing.B) {
	return func(b *testing.B) {
		tid := thread.NewIDV1(thread.Raw, 24)

		_, pub, err := pt.RandTestKeyPair(ic.RSA, 512)
		if err != nil {
			b.Error(err)
		}

		id, err := peer.IDFromPublicKey(pub)
		if err != nil {
			b.Error(err)
		}

		hash, _ := mh.Encode([]byte("foo"), mh.SHA2_256)
		head := cid.NewCidV1(cid.DagCBOR, hash)
		hb.SetLogHeads(tid, id, []cid.Cid{head})

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			hb.ClearLogHeads(tid, id)
		}
	}
}
