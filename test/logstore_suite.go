package test

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	pstore "github.com/libp2p/go-libp2p-core/peerstore"
	ma "github.com/multiformats/go-multiaddr"
	core "github.com/textileio/go-threads/core/logstore"
	"github.com/textileio/go-threads/core/thread"
	sym "github.com/textileio/go-threads/crypto/symmetric"
)

var threadstoreSuite = map[string]func(core.Logstore) func(*testing.T){
	"AddrStream":              testAddrStream,
	"GetStreamBeforeLogAdded": testGetStreamBeforeLogAdded,
	"AddStreamDuplicates":     testAddrStreamDuplicates,
	"BasicLogstore":           testBasicLogstore,
	"Metadata":                testMetadata,
}

type LogstoreFactory func() (core.Logstore, func())

func LogstoreTest(t *testing.T, factory LogstoreFactory) {
	for name, test := range threadstoreSuite {
		// Create a new threadstore.
		ps, closeFunc := factory()

		// Run the test.
		t.Run(name, test(ps))

		// Cleanup.
		if closeFunc != nil {
			closeFunc()
		}
	}
}

func testAddrStream(ls core.Logstore) func(t *testing.T) {
	return func(t *testing.T) {
		tid := thread.NewIDV1(thread.Raw, 24)

		addrs, pid := getAddrs(t, 100), peer.ID("testlog")
		err := ls.AddAddrs(tid, pid, addrs[:10], time.Hour)
		check(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		addrch, err := ls.AddrStream(ctx, tid, pid)
		if err != nil {
			t.Fatalf("errro when adding stream: %v", err)
		}

		// while that subscription is active, publish ten more addrs
		// this tests that it doesnt hang
		for i := 10; i < 20; i++ {
			err = ls.AddAddr(tid, pid, addrs[i], time.Hour)
			check(t, err)
		}

		// now receive them (without hanging)
		timeout := time.After(time.Second * 10)
		for i := 0; i < 20; i++ {
			select {
			case <-addrch:
			case <-timeout:
				t.Fatal("timed out")
			}
		}

		// start a second stream
		ctx2, cancel2 := context.WithCancel(context.Background())
		addrch2, err := ls.AddrStream(ctx2, tid, pid)
		if err != nil {
			t.Fatalf("error when adding stream: %v", err)
		}

		done := make(chan struct{})
		go func() {
			defer close(done)
			// now send the rest of the addresses
			for _, a := range addrs[20:80] {
				err := ls.AddAddr(tid, pid, a, time.Hour)
				check(t, err)
			}
		}()

		// receive some concurrently with the goroutine
		timeout = time.After(time.Second * 10)
		for i := 0; i < 40; i++ {
			select {
			case <-addrch:
			case <-timeout:
			}
		}

		<-done

		// receive some more after waiting for that goroutine to complete
		timeout = time.After(time.Second * 10)
		for i := 0; i < 20; i++ {
			select {
			case <-addrch:
			case <-timeout:
			}
		}

		// now cancel it
		cancel()

		// now check the *second* subscription. We should see 80 addresses.
		for i := 0; i < 80; i++ {
			<-addrch2
		}

		cancel2()

		// and add a few more addresses it doesnt hang afterwards
		for _, a := range addrs[80:] {
			err = ls.AddAddr(tid, pid, a, time.Hour)
			check(t, err)
		}
	}
}

func testGetStreamBeforeLogAdded(ls core.Logstore) func(t *testing.T) {
	return func(t *testing.T) {
		tid := thread.NewIDV1(thread.Raw, 24)

		addrs, pid := getAddrs(t, 10), peer.ID("testlog")

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		ach, err := ls.AddrStream(ctx, tid, pid)
		if err != nil {
			t.Fatalf("error when adding stream: %v", err)
		}
		for i := 0; i < 10; i++ {
			err = ls.AddAddr(tid, pid, addrs[i], time.Hour)
			check(t, err)
		}

		received := make(map[string]bool)
		var count int

		for i := 0; i < 10; i++ {
			a, ok := <-ach
			if !ok {
				t.Fatal("channel shouldnt be closed yet")
			}
			if a == nil {
				t.Fatal("got a nil address, thats weird")
			}
			count++
			if received[a.String()] {
				t.Fatal("received duplicate address")
			}
			received[a.String()] = true
		}

		select {
		case <-ach:
			t.Fatal("shouldnt have received any more addresses")
		default:
		}

		if count != 10 {
			t.Fatal("should have received exactly ten addresses, got ", count)
		}

		for _, a := range addrs {
			if !received[a.String()] {
				t.Log(received)
				t.Fatalf("expected to receive address %s but didnt", a)
			}
		}
	}
}

func testAddrStreamDuplicates(ls core.Logstore) func(t *testing.T) {
	return func(t *testing.T) {
		tid := thread.NewIDV1(thread.Raw, 24)

		addrs, pid := getAddrs(t, 10), peer.ID("testlog")

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		ach, err := ls.AddrStream(ctx, tid, pid)
		if err != nil {
			t.Fatalf("error when adding stream: %v", err)
		}
		go func() {
			for i := 0; i < 10; i++ {
				err := ls.AddAddr(tid, pid, addrs[i], time.Hour)
				check(t, err)
				err = ls.AddAddr(tid, pid, addrs[rand.Intn(10)], time.Hour)
				check(t, err)
			}

			// make sure that all addresses get processed before context is cancelled
			time.Sleep(time.Millisecond * 50)
			cancel()
		}()

		received := make(map[string]bool)
		var count int
		for a := range ach {
			if a == nil {
				t.Fatal("got a nil address, thats weird")
			}
			count++
			if received[a.String()] {
				t.Fatal("received duplicate address")
			}
			received[a.String()] = true
		}

		if count != 10 {
			t.Fatal("should have received exactly ten addresses")
		}
	}
}

func testBasicLogstore(ls core.Logstore) func(t *testing.T) {
	return func(t *testing.T) {
		tids := make([]thread.ID, 0)
		addrs := getAddrs(t, 10)

		for _, a := range addrs {
			tid := thread.NewIDV1(thread.Raw, 24)
			tids = append(tids, tid)
			err := ls.AddServiceKey(tid, sym.New())
			check(t, err)
			err = ls.AddReadKey(tid, sym.New())
			check(t, err)
			priv, pub, _ := crypto.GenerateKeyPair(crypto.RSA, crypto.MinRsaKeyBits)
			p, _ := peer.IDFromPrivateKey(priv)
			err = ls.AddAddr(tid, p, a, pstore.PermanentAddrTTL)
			check(t, err)
			err = ls.AddPubKey(tid, p, pub)
			check(t, err)
			err = ls.AddPrivKey(tid, p, priv)
			check(t, err)
		}

		threads, err := ls.Threads()
		check(t, err)
		if len(threads) != 10 {
			t.Fatal("expected ten threads, got", len(threads))
		}

		info, err := ls.GetThread(tids[0])
		check(t, err)
		tsAddrs, err := ls.Addrs(info.ID, info.Logs[0].ID)
		if err != nil {
			t.Fatalf("error when getting addresses: %v", err)
		}
		if !tsAddrs[0].Equal(addrs[0]) {
			t.Fatal("stored wrong address")
		}

		log, err := ls.GetLog(info.ID, info.Logs[0].ID)
		check(t, err)
		if !log.Addrs[0].Equal(addrs[0]) {
			t.Fatal("stored wrong address")
		}

		// Test add entire log
		tid := thread.NewIDV1(thread.Raw, 24)
		priv, pub, _ := crypto.GenerateKeyPair(crypto.RSA, crypto.MinRsaKeyBits)
		p, _ := peer.IDFromPrivateKey(priv)
		err = ls.AddLog(tid, thread.LogInfo{
			ID:      p,
			PubKey:  pub,
			PrivKey: priv,
			Addrs:   getAddrs(t, 1),
		})
		check(t, err)
		log, err = ls.GetLog(tid, p)
		check(t, err)

		// Test delete log
		err = ls.DeleteLog(tid, p)
		check(t, err)
		if _, err = ls.GetLog(tid, p); err != core.ErrLogNotFound {
			t.Fatal("log was not deleted")
		}

		// Test delete thread (add the log back to the store)
		err = ls.AddLog(tid, thread.LogInfo{
			ID:      p,
			PubKey:  pub,
			PrivKey: priv,
			Addrs:   getAddrs(t, 1),
		})
		check(t, err)
		log, err = ls.GetLog(tid, p)
		check(t, err)

		err = ls.DeleteThread(tid)
		check(t, err)
		if _, err = ls.GetThread(tid); err != core.ErrThreadNotFound {
			t.Fatal("thread was not deleted")
		}
	}
}

func testLogstoreManaged(ls core.Logstore) func(t *testing.T) {
	return func(t *testing.T) {
		tid := thread.NewIDV1(thread.Raw, 24)
		addrs := getAddrs(t, 1)
		err := ls.AddServiceKey(tid, sym.New())
		check(t, err)
		err = ls.AddReadKey(tid, sym.New())
		check(t, err)
		priv, pub, _ := crypto.GenerateKeyPair(crypto.RSA, crypto.MinRsaKeyBits)
		p, _ := peer.IDFromPrivateKey(priv)
		err = ls.AddAddr(tid, p, addrs[0], pstore.PermanentAddrTTL)
		check(t, err)
		err = ls.AddPubKey(tid, p, pub)
		check(t, err)
		err = ls.AddPrivKey(tid, p, priv)
		check(t, err)

		// Check that log is managed
		info, err := ls.GetThread(tid)
		check(t, err)

		log, err := ls.GetLog(info.ID, info.Logs[0].ID)
		check(t, err)
		if log.Managed != true {
			t.Fatal("log not managed")
		}

		// Test adding owned log to existing thread
		priv, pub, _ = crypto.GenerateKeyPair(crypto.RSA, crypto.MinRsaKeyBits)
		p, _ = peer.IDFromPrivateKey(priv)
		err = ls.AddLog(tid, thread.LogInfo{
			ID:      p,
			PubKey:  pub,
			PrivKey: priv, // This should cause a failure
			Addrs:   getAddrs(t, 1),
		})
		if err == nil {
			t.Fatal("can't add more than one owned log")
		}

		// Test adding managed log to existing thread
		err = ls.AddLog(tid, thread.LogInfo{
			ID:     p,
			PubKey: pub,
			Addrs:  getAddrs(t, 1),
		})
		check(t, err)

		log, err = ls.GetLog(tid, p)
		check(t, err)
		if log.Managed != true {
			t.Fatal("log not managed")
		}
	}
}

func testMetadata(ls core.Logstore) func(t *testing.T) {
	return func(t *testing.T) {
		tids := make([]thread.ID, 10)
		for i := range tids {
			tids[i] = thread.NewIDV1(thread.Raw, 24)
		}
		for _, p := range tids {
			if err := ls.PutString(p, "AgentVersion", "string"); err != nil {
				t.Errorf("failed to put %q: %s", "AgentVersion", err)
			}
			if err := ls.PutInt64(p, "bar", 1); err != nil {
				t.Errorf("failed to put %q: %s", "bar", err)
			}
		}
		for _, p := range tids {
			v, err := ls.GetString(p, "AgentVersion")
			if err != nil {
				t.Errorf("failed to find %q: %s", "AgentVersion", err)
				continue
			}
			if v != nil && *v != "string" {
				t.Errorf("expected %q, got %q", "string", p)
				continue
			}

			vi, err := ls.GetInt64(p, "bar")
			if err != nil {
				t.Errorf("failed to find %q: %s", "bar", err)
				continue
			}
			if vi != nil && *vi != 1 {
				t.Errorf("expected %q, got %v", 1, v)
				continue
			}
		}
	}
}

func getAddrs(t *testing.T, n int) []ma.Multiaddr {
	var addrs []ma.Multiaddr
	for i := 0; i < n; i++ {
		a, err := ma.NewMultiaddr(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", i))
		if err != nil {
			t.Fatal(err)
		}

		addrs = append(addrs, a)
	}
	return addrs
}
