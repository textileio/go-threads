package test

import (
	"bytes"
	"math/rand"
	"sort"
	"testing"
	"time"

	ic "github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	pt "github.com/libp2p/go-libp2p-core/test"
	"github.com/textileio/go-textile-core/crypto"
	"github.com/textileio/go-textile-core/thread"
	tstore "github.com/textileio/go-textile-core/threadstore"
)

var keyBookSuite = map[string]func(kb tstore.KeyBook) func(*testing.T){
	"AddGetPrivKey":         testKeyBookPrivKey,
	"AddGetPubKey":          testKeyBookPubKey,
	"AddGetReadKey":         testKeyBookReadKey,
	"AddGetFollowKey":       testKeyBookFollowKey,
	"LogsWithKeys":          testKeyBookLogs,
	"ThreadsFromKeys":       testKeyBookThreads,
	"PubKeyAddedOnRetrieve": testInlinedPubKeyAddedOnRetrieve,
}

type KeyBookFactory func() (tstore.KeyBook, func())

func KeyBookTest(t *testing.T, factory KeyBookFactory) {
	for name, test := range keyBookSuite {
		// Create a new book.
		kb, closeFunc := factory()

		// Run the test.
		t.Run(name, test(kb))

		// Cleanup.
		if closeFunc != nil {
			closeFunc()
		}
	}
}

func testKeyBookPrivKey(kb tstore.KeyBook) func(t *testing.T) {
	return func(t *testing.T) {
		tid := thread.NewIDV1(thread.Raw, 24)

		if logs := kb.LogsWithKeys(tid); len(logs) > 0 {
			t.Error("expected logs to be empty on init")
		}

		priv, _, err := pt.RandTestKeyPair(ic.RSA, 512)
		if err != nil {
			t.Error(err)
		}

		id, err := peer.IDFromPrivateKey(priv)
		if err != nil {
			t.Error(err)
		}

		if res := kb.PrivKey(tid, id); res != nil {
			t.Error("retrieving private key should have failed")
		}

		err = kb.AddPrivKey(tid, id, priv)
		if err != nil {
			t.Error(err)
		}

		if res := kb.PrivKey(tid, id); !priv.Equals(res) {
			t.Error("retrieved private key did not match stored private key")
		}

		if logs := kb.LogsWithKeys(tid); len(logs) != 1 || logs[0] != id {
			t.Error("list of logs did not include test log")
		}
	}
}

func testKeyBookPubKey(kb tstore.KeyBook) func(t *testing.T) {
	return func(t *testing.T) {
		tid := thread.NewIDV1(thread.Raw, 24)

		if logs := kb.LogsWithKeys(tid); len(logs) > 0 {
			t.Error("expected logs to be empty on init")
		}

		_, pub, err := pt.RandTestKeyPair(ic.RSA, 512)
		if err != nil {
			t.Error(err)
		}

		id, err := peer.IDFromPublicKey(pub)
		if err != nil {
			t.Error(err)
		}

		if res := kb.PubKey(tid, id); res != nil {
			t.Error("retrieving public key should have failed")
		}

		err = kb.AddPubKey(tid, id, pub)
		if err != nil {
			t.Error(err)
		}

		if res := kb.PubKey(tid, id); !pub.Equals(res) {
			t.Error("retrieved public key did not match stored public key")
		}

		if logs := kb.LogsWithKeys(tid); len(logs) != 1 || logs[0] != id {
			t.Error("list of logs did not include test log")
		}
	}
}

func testKeyBookReadKey(kb tstore.KeyBook) func(t *testing.T) {
	return func(t *testing.T) {
		tid := thread.NewIDV1(thread.Raw, 24)

		if logs := kb.LogsWithKeys(tid); len(logs) > 0 {
			t.Error("expected logs to be empty on init")
		}

		_, pub, err := pt.RandTestKeyPair(ic.RSA, 512)
		if err != nil {
			t.Error(err)
		}

		id, err := peer.IDFromPublicKey(pub)
		if err != nil {
			t.Error(err)
		}

		key, err := crypto.GenerateAESKey()
		if err != nil {
			t.Error(err)
		}

		err = kb.AddReadKey(tid, id, key)
		if err != nil {
			t.Error(err)
		}

		if res := kb.ReadKey(tid, id); !bytes.Equal(res, key) {
			t.Error("retrieved read key did not match stored read key")
		}
	}
}

func testKeyBookFollowKey(kb tstore.KeyBook) func(t *testing.T) {
	return func(t *testing.T) {
		tid := thread.NewIDV1(thread.Raw, 24)

		if logs := kb.LogsWithKeys(tid); len(logs) > 0 {
			t.Error("expected logs to be empty on init")
		}

		_, pub, err := pt.RandTestKeyPair(ic.RSA, 512)
		if err != nil {
			t.Error(err)
		}

		id, err := peer.IDFromPublicKey(pub)
		if err != nil {
			t.Error(err)
		}

		key, err := crypto.GenerateAESKey()
		if err != nil {
			t.Error(err)
		}

		err = kb.AddFollowKey(tid, id, key)
		if err != nil {
			t.Error(err)
		}

		if res := kb.FollowKey(tid, id); !bytes.Equal(res, key) {
			t.Error("retrieved read key did not match stored read key")
		}
	}
}

func testKeyBookLogs(kb tstore.KeyBook) func(t *testing.T) {
	return func(t *testing.T) {
		tid := thread.NewIDV1(thread.Raw, 24)

		if logs := kb.LogsWithKeys(tid); len(logs) > 0 {
			t.Error("expected logs to be empty on init")
		}

		logs := make(peer.IDSlice, 0)
		for i := 0; i < 10; i++ {
			// Add a public key.
			_, pub, _ := pt.RandTestKeyPair(ic.RSA, 512)
			p1, _ := peer.IDFromPublicKey(pub)
			_ = kb.AddPubKey(tid, p1, pub)

			// Add a private key.
			priv, _, _ := pt.RandTestKeyPair(ic.RSA, 512)
			p2, _ := peer.IDFromPrivateKey(priv)
			_ = kb.AddPrivKey(tid, p2, priv)

			logs = append(logs, []peer.ID{p1, p2}...)
		}

		kbLogs := kb.LogsWithKeys(tid)
		sort.Sort(kbLogs)
		sort.Sort(logs)

		for i, p := range kbLogs {
			if p != logs[i] {
				t.Errorf("mismatch of log at index %d", i)
			}
		}
	}
}

func testKeyBookThreads(kb tstore.KeyBook) func(t *testing.T) {
	return func(t *testing.T) {
		if threads := kb.ThreadsFromKeys(); len(threads) > 0 {
			t.Error("expected threads to be empty on init")
		}

		threads := thread.IDSlice{
			thread.NewIDV1(thread.Raw, 16),
			thread.NewIDV1(thread.Raw, 24),
			thread.NewIDV1(thread.AccessControlled, 32),
		}
		rand.Seed(time.Now().Unix())
		for i := 0; i < 10; i++ {
			// Choose a random thread.
			tid := threads[rand.Intn(len(threads))]
			// Add a public key.
			_, pub, _ := pt.RandTestKeyPair(ic.RSA, 512)
			p1, _ := peer.IDFromPublicKey(pub)
			_ = kb.AddPubKey(tid, p1, pub)

			// Add a private key.
			priv, _, _ := pt.RandTestKeyPair(ic.RSA, 512)
			p2, _ := peer.IDFromPrivateKey(priv)
			_ = kb.AddPrivKey(tid, p2, priv)
		}

		kbThreads := kb.ThreadsFromKeys()
		sort.Sort(kbThreads)
		sort.Sort(threads)

		for i, p := range kbThreads {
			if p != threads[i] {
				t.Errorf("mismatch of thread at index %d", i)
			}
		}
	}
}

func testInlinedPubKeyAddedOnRetrieve(kb tstore.KeyBook) func(t *testing.T) {
	return func(t *testing.T) {
		t.Skip("key inlining disabled for now: see libp2p/specs#111")

		tid := thread.NewIDV1(thread.Raw, 24)

		if logs := kb.LogsWithKeys(tid); len(logs) > 0 {
			t.Error("expected logs to be empty on init")
		}

		// Key small enough for inlining.
		_, pub, err := ic.GenerateKeyPair(ic.Ed25519, 256)
		if err != nil {
			t.Error(err)
		}

		id, err := peer.IDFromPublicKey(pub)
		if err != nil {
			t.Error(err)
		}

		pubKey := kb.PubKey(tid, id)
		if !pubKey.Equals(pub) {
			t.Error("mismatch between original public key and keybook-calculated one")
		}
	}
}

var logKeybookBenchmarkSuite = map[string]func(kb tstore.KeyBook) func(*testing.B){
	"PubKey":       benchmarkPubKey,
	"AddPubKey":    benchmarkAddPubKey,
	"PrivKey":      benchmarkPrivKey,
	"AddPrivKey":   benchmarkAddPrivKey,
	"LogsWithKeys": benchmarkLogsWithKeys,
}

func BenchmarkKeyBook(b *testing.B, factory KeyBookFactory) {
	ordernames := make([]string, 0, len(logKeybookBenchmarkSuite))
	for name := range logKeybookBenchmarkSuite {
		ordernames = append(ordernames, name)
	}
	sort.Strings(ordernames)
	for _, name := range ordernames {
		bench := logKeybookBenchmarkSuite[name]
		kb, closeFunc := factory()

		b.Run(name, bench(kb))

		if closeFunc != nil {
			closeFunc()
		}
	}
}

func benchmarkPubKey(kb tstore.KeyBook) func(*testing.B) {
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

		err = kb.AddPubKey(tid, id, pub)
		if err != nil {
			b.Fatal(err)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			kb.PubKey(tid, id)
		}
	}
}

func benchmarkAddPubKey(kb tstore.KeyBook) func(*testing.B) {
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

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = kb.AddPubKey(tid, id, pub)
		}
	}
}

func benchmarkPrivKey(kb tstore.KeyBook) func(*testing.B) {
	return func(b *testing.B) {
		tid := thread.NewIDV1(thread.Raw, 24)

		priv, _, err := pt.RandTestKeyPair(ic.RSA, 512)
		if err != nil {
			b.Error(err)
		}

		id, err := peer.IDFromPrivateKey(priv)
		if err != nil {
			b.Error(err)
		}

		err = kb.AddPrivKey(tid, id, priv)
		if err != nil {
			b.Fatal(err)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			kb.PrivKey(tid, id)
		}
	}
}

func benchmarkAddPrivKey(kb tstore.KeyBook) func(*testing.B) {
	return func(b *testing.B) {
		tid := thread.NewIDV1(thread.Raw, 24)

		priv, _, err := pt.RandTestKeyPair(ic.RSA, 512)
		if err != nil {
			b.Error(err)
		}

		id, err := peer.IDFromPrivateKey(priv)
		if err != nil {
			b.Error(err)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = kb.AddPrivKey(tid, id, priv)
		}
	}
}

func benchmarkLogsWithKeys(kb tstore.KeyBook) func(*testing.B) {
	return func(b *testing.B) {
		tid := thread.NewIDV1(thread.Raw, 24)
		for i := 0; i < 10; i++ {
			priv, pub, err := pt.RandTestKeyPair(ic.RSA, 512)
			if err != nil {
				b.Error(err)
			}

			id, err := peer.IDFromPublicKey(pub)
			if err != nil {
				b.Error(err)
			}

			err = kb.AddPubKey(tid, id, pub)
			if err != nil {
				b.Fatal(err)
			}
			err = kb.AddPrivKey(tid, id, priv)
			if err != nil {
				b.Fatal(err)
			}
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			kb.LogsWithKeys(tid)
		}
	}
}
