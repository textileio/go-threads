package test

import (
	"bytes"
	"math/rand"
	"sort"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	pt "github.com/libp2p/go-libp2p-core/test"
	core "github.com/textileio/go-threads/core/logstore"
	"github.com/textileio/go-threads/core/thread"
	sym "github.com/textileio/go-threads/crypto/symmetric"
)

var keyBookSuite = map[string]func(kb core.KeyBook) func(*testing.T){
	"AddGetPrivKey":           testKeyBookPrivKey,
	"AddGetPubKey":            testKeyBookPubKey,
	"AddGetReadKey":           testKeyBookReadKey,
	"AddGetServiceKey":        testKeyBookServiceKey,
	"LogsWithKeys":            testKeyBookLogs,
	"testKeyBookClearKeys":    testKeyBookClearKeys,
	"testKeyBookClearLogKeys": testKeyBookClearLogKeys,
	"ThreadsFromKeys":         testKeyBookThreads,
	"PubKeyAddedOnRetrieve":   testInlinedPubKeyAddedOnRetrieve,
	"ExportKeyBook":           testKeyBookExport,
}

type KeyBookFactory func() (core.KeyBook, func())

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

func testKeyBookPrivKey(kb core.KeyBook) func(t *testing.T) {
	return func(t *testing.T) {
		tid := thread.NewIDV1(thread.Raw, 24)

		if logs, err := kb.LogsWithKeys(tid); err != nil || len(logs) > 0 {
			t.Error("expected logs to be empty on init without erros")
		}

		priv, _, err := pt.RandTestKeyPair(crypto.RSA, crypto.MinRsaKeyBits)
		if err != nil {
			t.Error(err)
		}

		id, err := peer.IDFromPrivateKey(priv)
		if err != nil {
			t.Error(err)
		}

		if res, err := kb.PrivKey(tid, id); err != nil || res != nil {
			t.Error("retrieving private key should have failed without errors")
		}

		err = kb.AddPrivKey(tid, id, priv)
		if err != nil {
			t.Error(err)
		}

		if res, err := kb.PrivKey(tid, id); err != nil || !priv.Equals(res) {
			t.Error("retrieved private key did not match stored private key without errors")
		}

		if logs, err := kb.LogsWithKeys(tid); err != nil || len(logs) != 1 || logs[0] != id {
			t.Error("list of logs did not include test log without errors")
		}
	}
}

func testKeyBookPubKey(kb core.KeyBook) func(t *testing.T) {
	return func(t *testing.T) {
		tid := thread.NewIDV1(thread.Raw, 24)

		if logs, err := kb.LogsWithKeys(tid); err != nil || len(logs) > 0 {
			t.Error("expected logs to be empty on init without errors")
		}

		_, pub, err := pt.RandTestKeyPair(crypto.RSA, crypto.MinRsaKeyBits)
		if err != nil {
			t.Error(err)
		}

		id, err := peer.IDFromPublicKey(pub)
		if err != nil {
			t.Error(err)
		}

		if res, err := kb.PubKey(tid, id); err != nil || res != nil {
			t.Error("retrieving public key should have failed without errors")
		}

		err = kb.AddPubKey(tid, id, pub)
		if err != nil {
			t.Error(err)
		}

		if res, err := kb.PubKey(tid, id); err != nil || !pub.Equals(res) {
			t.Error("retrieved public key did not match stored public key without errors")
		}

		if logs, err := kb.LogsWithKeys(tid); err != nil || len(logs) != 1 || logs[0] != id {
			t.Error("list of logs did not include test log without errors")
		}
	}
}

func testKeyBookReadKey(kb core.KeyBook) func(t *testing.T) {
	return func(t *testing.T) {
		tid := thread.NewIDV1(thread.Raw, 24)

		if logs, err := kb.LogsWithKeys(tid); err != nil || len(logs) > 0 {
			t.Error("expected logs to be empty on init without errors")
		}

		key, err := sym.NewRandom()
		if err != nil {
			t.Error(err)
		}

		err = kb.AddReadKey(tid, key)
		if err != nil {
			t.Error(err)
		}

		if res, err := kb.ReadKey(tid); err != nil || !bytes.Equal(res.Bytes(), key.Bytes()) {
			t.Error("retrieved read key did not match stored read key without errors")
		}
	}
}

func testKeyBookServiceKey(kb core.KeyBook) func(t *testing.T) {
	return func(t *testing.T) {
		tid := thread.NewIDV1(thread.Raw, 24)

		if logs, err := kb.LogsWithKeys(tid); err != nil || len(logs) > 0 {
			t.Error("expected logs to be empty on init without errors")
		}

		key, err := sym.NewRandom()
		if err != nil {
			t.Error(err)
		}

		err = kb.AddServiceKey(tid, key)
		if err != nil {
			t.Error(err)
		}

		if res, err := kb.ServiceKey(tid); err != nil || !bytes.Equal(res.Bytes(), key.Bytes()) {
			t.Error("retrieved read key did not match stored read key without errors")
		}
	}
}

func testKeyBookClearKeys(kb core.KeyBook) func(t *testing.T) {
	return func(t *testing.T) {
		tid := thread.NewIDV1(thread.Raw, 24)

		key1, err := sym.NewRandom()
		if err != nil {
			t.Fatal(err)
		}
		err = kb.AddServiceKey(tid, key1)
		if err != nil {
			t.Fatal(err)
		}
		if res, err := kb.ServiceKey(tid); err != nil || res == nil {
			t.Error("missing service key")
		}

		key2, err := sym.NewRandom()
		if err != nil {
			t.Fatal(err)
		}
		err = kb.AddReadKey(tid, key2)
		if err != nil {
			t.Fatal(err)
		}
		if res, err := kb.ReadKey(tid); err != nil || res == nil {
			t.Error("missing read key")
		}

		priv, pub, err := pt.RandTestKeyPair(crypto.RSA, crypto.MinRsaKeyBits)
		if err != nil {
			t.Fatal(err)
		}
		id, err := peer.IDFromPrivateKey(priv)
		if err != nil {
			t.Fatal(err)
		}
		err = kb.AddPrivKey(tid, id, priv)
		if err != nil {
			t.Fatal(err)
		}
		if res, err := kb.PrivKey(tid, id); err != nil || res == nil {
			t.Error("missing private key")
		}
		err = kb.AddPubKey(tid, id, pub)
		if err != nil {
			t.Fatal(err)
		}
		if res, err := kb.PubKey(tid, id); err != nil || res == nil {
			t.Error("missing public key")
		}

		if err = kb.ClearKeys(tid); err != nil {
			t.Fatal(err)
		}

		if res, err := kb.ServiceKey(tid); err != nil || res != nil {
			t.Error("service key should have been deleted")
		}
		if res, err := kb.ReadKey(tid); err != nil || res != nil {
			t.Error("read key should have been deleted")
		}
		if res, err := kb.PrivKey(tid, id); err != nil || res != nil {
			t.Error("private key should have been deleted")
		}
		if res, err := kb.PubKey(tid, id); err != nil || res != nil {
			t.Error("public key should have been deleted")
		}
	}
}

func testKeyBookClearLogKeys(kb core.KeyBook) func(t *testing.T) {
	return func(t *testing.T) {
		tid := thread.NewIDV1(thread.Raw, 24)

		key1, err := sym.NewRandom()
		if err != nil {
			t.Fatal(err)
		}
		err = kb.AddServiceKey(tid, key1)
		if err != nil {
			t.Fatal(err)
		}
		if res, err := kb.ServiceKey(tid); err != nil || res == nil {
			t.Error("missing service key")
		}

		key2, err := sym.NewRandom()
		if err != nil {
			t.Fatal(err)
		}
		err = kb.AddReadKey(tid, key2)
		if err != nil {
			t.Fatal(err)
		}
		if res, err := kb.ReadKey(tid); err != nil || res == nil {
			t.Error("missing read key")
		}

		priv, pub, err := pt.RandTestKeyPair(crypto.RSA, crypto.MinRsaKeyBits)
		if err != nil {
			t.Fatal(err)
		}
		id, err := peer.IDFromPrivateKey(priv)
		if err != nil {
			t.Fatal(err)
		}
		err = kb.AddPrivKey(tid, id, priv)
		if err != nil {
			t.Fatal(err)
		}
		if res, err := kb.PrivKey(tid, id); err != nil || res == nil {
			t.Error("missing private key")
		}
		err = kb.AddPubKey(tid, id, pub)
		if err != nil {
			t.Fatal(err)
		}
		if res, err := kb.PubKey(tid, id); err != nil || res == nil {
			t.Error("missing public key")
		}

		if err = kb.ClearLogKeys(tid, id); err != nil {
			t.Fatal(err)
		}

		if res, err := kb.PrivKey(tid, id); err != nil || res != nil {
			t.Error("private key should have been deleted")
		}
		if res, err := kb.PubKey(tid, id); err != nil || res != nil {
			t.Error("public key should have been deleted")
		}
	}
}

func testKeyBookLogs(kb core.KeyBook) func(t *testing.T) {
	return func(t *testing.T) {
		tid := thread.NewIDV1(thread.Raw, 24)

		if logs, err := kb.LogsWithKeys(tid); err != nil || len(logs) > 0 {
			t.Error("expected logs to be empty on init without errors")
		}

		logs := make(peer.IDSlice, 0)
		for i := 0; i < 10; i++ {
			// Add a public key.
			_, pub, _ := pt.RandTestKeyPair(crypto.RSA, crypto.MinRsaKeyBits)
			p1, _ := peer.IDFromPublicKey(pub)
			_ = kb.AddPubKey(tid, p1, pub)

			// Add a private key.
			priv, _, _ := pt.RandTestKeyPair(crypto.RSA, crypto.MinRsaKeyBits)
			p2, _ := peer.IDFromPrivateKey(priv)
			_ = kb.AddPrivKey(tid, p2, priv)

			logs = append(logs, []peer.ID{p1, p2}...)
		}

		kbLogs, err := kb.LogsWithKeys(tid)
		if err != nil {
			t.Fatalf("getting logs with keys failed: %v", err)
		}

		for _, kbid := range kbLogs {
			found := false
			for _, id := range logs {
				if kbid.String() == id.String() {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("%s not found in store list", kbid.String())
			}
		}
	}
}

func testKeyBookThreads(kb core.KeyBook) func(t *testing.T) {
	return func(t *testing.T) {
		if threads, err := kb.ThreadsFromKeys(); err != nil || len(threads) > 0 {
			t.Error("expected threads to be empty on init without errors")
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
			_, pub, _ := pt.RandTestKeyPair(crypto.RSA, crypto.MinRsaKeyBits)
			p1, _ := peer.IDFromPublicKey(pub)
			_ = kb.AddPubKey(tid, p1, pub)

			// Add a private key.
			priv, _, _ := pt.RandTestKeyPair(crypto.RSA, crypto.MinRsaKeyBits)
			p2, _ := peer.IDFromPrivateKey(priv)
			_ = kb.AddPrivKey(tid, p2, priv)
		}

		kbThreads, err := kb.ThreadsFromKeys()
		if err != nil {
			t.Fatalf("error when getting thread from keys: %v", err)
		}

		for _, kbid := range kbThreads {
			found := false
			for _, id := range threads {
				if kbid.String() == id.String() {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("%s not found in store list", kbid.String())
			}
		}
	}
}

func testInlinedPubKeyAddedOnRetrieve(kb core.KeyBook) func(t *testing.T) {
	return func(t *testing.T) {
		t.Skip("key inlining disabled for now: see libp2p/specs#111")

		tid := thread.NewIDV1(thread.Raw, 24)

		if logs, err := kb.LogsWithKeys(tid); err != nil || len(logs) > 0 {
			t.Error("expected logs to be empty on init without errors")
		}

		// Key small enough for inlining.
		_, pub, err := crypto.GenerateKeyPair(crypto.Ed25519, 256)
		if err != nil {
			t.Error(err)
		}

		id, err := peer.IDFromPublicKey(pub)
		if err != nil {
			t.Error(err)
		}

		pubKey, err := kb.PubKey(tid, id)
		if err != nil {
			t.Fatalf("error when getting public key: %v", err)
		}
		if !pubKey.Equals(pub) {
			t.Error("mismatch between original public key and keybook-calculated one")
		}
	}
}

func testKeyBookExport(kb core.KeyBook) func(t *testing.T) {
	return func(t *testing.T) {
		var (
			numThreads = 2
			numLogs    = 3

			public  = make(map[thread.ID]map[peer.ID]crypto.PubKey)
			private = make(map[thread.ID]map[peer.ID]crypto.PrivKey)
			read    = make(map[thread.ID][]byte)
			service = make(map[thread.ID][]byte)
		)

		// generate key set
		for i := 0; i < numThreads; i++ {
			tid := thread.NewIDV1(thread.Raw, 24)

			readKey, err := sym.NewRandom()
			if err != nil {
				t.Error(err)
			}
			read[tid] = readKey.Bytes()
			if err := kb.AddReadKey(tid, readKey); err != nil {
				t.Fatal(err)
			}

			serviceKey, err := sym.NewRandom()
			if err != nil {
				t.Error(err)
			}
			service[tid] = serviceKey.Bytes()
			if err := kb.AddServiceKey(tid, serviceKey); err != nil {
				t.Fatal(err)
			}

			public[tid] = make(map[peer.ID]crypto.PubKey)
			private[tid] = make(map[peer.ID]crypto.PrivKey)

			for j := 0; j < numLogs; j++ {
				priv, pub, _ := pt.RandTestKeyPair(crypto.RSA, crypto.MinRsaKeyBits)
				lid, _ := peer.IDFromPublicKey(pub)

				public[tid][lid] = pub
				private[tid][lid] = priv

				if err := kb.AddPubKey(tid, lid, pub); err != nil {
					t.Fatal(err)
				}

				if err := kb.AddPrivKey(tid, lid, priv); err != nil {
					t.Fatal(err)
				}
			}
		}

		// make a dump
		dump, err := kb.DumpKeys()
		if err != nil {
			t.Fatal(err)
		}

		// purge all keys
		for tid := range read {
			if err := kb.ClearKeys(tid); err != nil {
				t.Fatal(err)
			}
		}

		// try to restore from the dump
		if err := kb.RestoreKeys(dump); err != nil {
			t.Fatal(err)
		}

		// compare public keys
		for tid, logs := range public {
			for lid, key := range logs {
				pk, err := kb.PubKey(tid, lid)
				if err != nil {
					t.Fatal(err)
				}
				if !pk.Equals(key) {
					t.Error("restored public key is different from the original one")
				}
			}
		}

		// compare private keys
		for tid, logs := range private {
			for lid, key := range logs {
				pk, err := kb.PrivKey(tid, lid)
				if err != nil {
					t.Fatal(err)
				}
				if !pk.Equals(key) {
					t.Error("restored private key is different from the original one")
				}
			}
		}

		// compare read keys
		for tid, key := range read {
			rk, err := kb.ReadKey(tid)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(rk.Bytes(), key) {
				t.Error("restored thread-read key is different from the original one")
			}
		}

		// compare service keys
		for tid, key := range service {
			sk, err := kb.ServiceKey(tid)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(sk.Bytes(), key) {
				t.Error("restored thread-service key is different from the original one")
			}
		}
	}
}

var logKeybookBenchmarkSuite = map[string]func(kb core.KeyBook) func(*testing.B){
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

func benchmarkPubKey(kb core.KeyBook) func(*testing.B) {
	return func(b *testing.B) {
		tid := thread.NewIDV1(thread.Raw, 24)

		_, pub, err := pt.RandTestKeyPair(crypto.RSA, crypto.MinRsaKeyBits)
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
			_, _ = kb.PubKey(tid, id)
		}
	}
}

func benchmarkAddPubKey(kb core.KeyBook) func(*testing.B) {
	return func(b *testing.B) {
		tid := thread.NewIDV1(thread.Raw, 24)

		_, pub, err := pt.RandTestKeyPair(crypto.RSA, crypto.MinRsaKeyBits)
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

func benchmarkPrivKey(kb core.KeyBook) func(*testing.B) {
	return func(b *testing.B) {
		tid := thread.NewIDV1(thread.Raw, 24)

		priv, _, err := pt.RandTestKeyPair(crypto.RSA, crypto.MinRsaKeyBits)
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
			_, _ = kb.PrivKey(tid, id)
		}
	}
}

func benchmarkAddPrivKey(kb core.KeyBook) func(*testing.B) {
	return func(b *testing.B) {
		tid := thread.NewIDV1(thread.Raw, 24)

		priv, _, err := pt.RandTestKeyPair(crypto.RSA, crypto.MinRsaKeyBits)
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

func benchmarkLogsWithKeys(kb core.KeyBook) func(*testing.B) {
	return func(b *testing.B) {
		tid := thread.NewIDV1(thread.Raw, 24)
		for i := 0; i < 10; i++ {
			priv, pub, err := pt.RandTestKeyPair(crypto.RSA, crypto.MinRsaKeyBits)
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
			_, _ = kb.LogsWithKeys(tid)
		}
	}
}
