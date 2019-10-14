package test

import (
	"sort"
	"strconv"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/crypto"
	ic "github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	pt "github.com/libp2p/go-libp2p-core/test"
	mh "github.com/multiformats/go-multihash"
	"github.com/textileio/go-textile-core/thread"
	tstore "github.com/textileio/go-textile-core/threadstore"
)

var headBookSuite = map[string]func(hb tstore.HeadBook) func(*testing.T){
	"AddGetHeads": testHeadBookAddHeads,
	"SetGetHeads": testHeadBookSetHeads,
	"ClearHeads":  testHeadBookClearHeads,
}

type HeadBookFactory func() (tstore.HeadBook, func())

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

func testHeadBookAddHeads(hb tstore.HeadBook) func(t *testing.T) {
	return func(t *testing.T) {
		tid := thread.NewIDV1(thread.Raw, 24)

		_, pub, _ := pt.RandTestKeyPair(ic.RSA, crypto.MinRsaKeyBits)
		p, _ := peer.IDFromPublicKey(pub)

		if heads, err := hb.Heads(tid, p); err != nil || len(heads) > 0 {
			t.Error("expected heads to be empty on init without errors")
		}

		heads := make([]cid.Cid, 0)
		for i := 0; i < 2; i++ {
			hash, _ := mh.Encode([]byte("foo"+strconv.Itoa(i)), mh.SHA2_256)
			head := cid.NewCidV1(cid.DagCBOR, hash)

			if err := hb.AddHeads(tid, p, []cid.Cid{head}); err != nil {
				t.Fatalf("error when adding heads: %v", err)
			}
			heads = append(heads, head)
		}

		hbHeads, err := hb.Heads(tid, p)
		if err != nil {
			t.Fatalf("error while getting heads: %v", err)
		}
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

func testHeadBookSetHeads(hb tstore.HeadBook) func(t *testing.T) {
	return func(t *testing.T) {
		tid := thread.NewIDV1(thread.Raw, 24)

		_, pub, _ := pt.RandTestKeyPair(ic.RSA, crypto.MinRsaKeyBits)
		p, _ := peer.IDFromPublicKey(pub)

		if heads, err := hb.Heads(tid, p); err != nil || len(heads) > 0 {
			t.Error("expected heads to be empty on init without errors")
		}

		heads := make([]cid.Cid, 0)
		for i := 0; i < 2; i++ {
			hash, _ := mh.Encode([]byte("foo"+strconv.Itoa(i)), mh.SHA2_256)
			head := cid.NewCidV1(cid.DagCBOR, hash)
			heads = append(heads, head)
		}
		if err := hb.SetHeads(tid, p, heads); err != nil {
			t.Fatalf("error when setting heads: %v", err)
		}

		hbHeads, err := hb.Heads(tid, p)
		if err != nil {
			t.Fatalf("error when getting heads: %v", err)
		}
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

func testHeadBookClearHeads(hb tstore.HeadBook) func(t *testing.T) {
	return func(t *testing.T) {
		tid := thread.NewIDV1(thread.Raw, 24)

		_, pub, _ := pt.RandTestKeyPair(ic.RSA, crypto.MinRsaKeyBits)
		p, _ := peer.IDFromPublicKey(pub)

		if heads, err := hb.Heads(tid, p); err != nil || len(heads) > 0 {
			t.Error("expected heads to be empty on init without errors")
		}

		heads := make([]cid.Cid, 0)
		for i := 0; i < 2; i++ {
			hash, _ := mh.Encode([]byte("foo"+strconv.Itoa(i)), mh.SHA2_256)
			head := cid.NewCidV1(cid.DagCBOR, hash)

			if err := hb.AddHead(tid, p, head); err != nil {
				t.Fatalf("error when adding heads: %v", err)
			}
			heads = append(heads, head)
		}

		heads, err := hb.Heads(tid, p)
		if err != nil {
			t.Fatalf("error when getting heads: %v", err)
		}
		len1 := len(heads)
		if len1 != 2 {
			t.Errorf("incorrect heads length %d", len1)
		}

		if err = hb.ClearHeads(tid, p); err != nil {
			t.Fatalf("error when clearing heads: %v", err)
		}

		heads, err = hb.Heads(tid, p)
		if err != nil {
			t.Fatalf("error when getting heads: %v", err)
		}
		len2 := len(heads)
		if len2 != 0 {
			t.Errorf("incorrect heads length %d", len2)
		}
	}
}

var logHeadbookBenchmarkSuite = map[string]func(hb tstore.HeadBook) func(*testing.B){
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

func benchmarkHeads(hb tstore.HeadBook) func(*testing.B) {
	return func(b *testing.B) {
		tid := thread.NewIDV1(thread.Raw, 24)

		_, pub, err := pt.RandTestKeyPair(ic.RSA, crypto.MinRsaKeyBits)
		if err != nil {
			b.Error(err)
		}

		id, err := peer.IDFromPublicKey(pub)
		if err != nil {
			b.Error(err)
		}

		hash, _ := mh.Encode([]byte("foo"), mh.SHA2_256)
		head := cid.NewCidV1(cid.DagCBOR, hash)

		hb.AddHead(tid, id, head)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			hb.Heads(tid, id)
		}
	}
}

func benchmarkAddHeads(hb tstore.HeadBook) func(*testing.B) {
	return func(b *testing.B) {
		tid := thread.NewIDV1(thread.Raw, 24)

		_, pub, err := pt.RandTestKeyPair(ic.RSA, crypto.MinRsaKeyBits)
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
			hb.AddHeads(tid, id, []cid.Cid{head})
		}
	}
}

func benchmarkSetHeads(hb tstore.HeadBook) func(*testing.B) {
	return func(b *testing.B) {
		tid := thread.NewIDV1(thread.Raw, 24)

		_, pub, err := pt.RandTestKeyPair(ic.RSA, crypto.MinRsaKeyBits)
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
			hb.SetHeads(tid, id, []cid.Cid{head})
		}
	}
}

func benchmarkClearHeads(hb tstore.HeadBook) func(*testing.B) {
	return func(b *testing.B) {
		tid := thread.NewIDV1(thread.Raw, 24)

		_, pub, err := pt.RandTestKeyPair(ic.RSA, crypto.MinRsaKeyBits)
		if err != nil {
			b.Error(err)
		}

		id, err := peer.IDFromPublicKey(pub)
		if err != nil {
			b.Error(err)
		}

		hash, _ := mh.Encode([]byte("foo"), mh.SHA2_256)
		head := cid.NewCidV1(cid.DagCBOR, hash)
		hb.SetHeads(tid, id, []cid.Cid{head})

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			hb.ClearHeads(tid, id)
		}
	}
}
