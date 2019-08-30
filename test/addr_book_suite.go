package test

import (
	"testing"
	"time"

	pstore "github.com/libp2p/go-libp2p-core/peerstore"
	"github.com/textileio/go-textile-core/thread"
	tstore "github.com/textileio/go-textile-core/threadstore"
)

var logAddressBookSuite = map[string]func(book tstore.LogAddrBook) func(*testing.T){
	"AddAddress":           testAddAddress,
	"Clear":                testClearWorks,
	"SetNegativeTTLClears": testSetNegativeTTLClears,
	"UpdateTTLs":           testUpdateTTLs,
	"NilAddrsDontBreak":    testNilAddrsDontBreak,
	"AddressesExpire":      testAddressesExpire,
	"ClearWithIter":        testClearWithIterator,
	"LogsWithAddresses":    testLogsWithAddrs,
}

type LogAddrBookFactory func() (tstore.LogAddrBook, func())

func LogAddrBookTest(t *testing.T, factory LogAddrBookFactory) {
	for name, test := range logAddressBookSuite {
		// Create a new book.
		ab, closeFunc := factory()

		// Run the test.
		t.Run(name, test(ab))

		// Cleanup.
		if closeFunc != nil {
			closeFunc()
		}
	}
}

func testAddAddress(ab tstore.LogAddrBook) func(*testing.T) {
	return func(t *testing.T) {
		tid := thread.NewIDV1(thread.Raw, 24)

		t.Run("add a single address", func(t *testing.T) {
			id := GeneratePeerIDs(1)[0]
			addrs := GenerateAddrs(1)

			ab.AddAddr(tid, id, addrs[0], time.Hour)

			AssertAddressesEqual(t, addrs, ab.Addrs(tid, id))
		})

		t.Run("idempotent add single address", func(t *testing.T) {
			id := GeneratePeerIDs(1)[0]
			addrs := GenerateAddrs(1)

			ab.AddAddr(tid, id, addrs[0], time.Hour)
			ab.AddAddr(tid, id, addrs[0], time.Hour)

			AssertAddressesEqual(t, addrs, ab.Addrs(tid, id))
		})

		t.Run("add multiple addresses", func(t *testing.T) {
			id := GeneratePeerIDs(1)[0]
			addrs := GenerateAddrs(3)

			ab.AddAddrs(tid, id, addrs, time.Hour)
			AssertAddressesEqual(t, addrs, ab.Addrs(tid, id))
		})

		t.Run("idempotent add multiple addresses", func(t *testing.T) {
			id := GeneratePeerIDs(1)[0]
			addrs := GenerateAddrs(3)

			ab.AddAddrs(tid, id, addrs, time.Hour)
			ab.AddAddrs(tid, id, addrs, time.Hour)

			AssertAddressesEqual(t, addrs, ab.Addrs(tid, id))
		})

		t.Run("adding an existing address with a later expiration extends its ttl", func(t *testing.T) {
			id := GeneratePeerIDs(1)[0]
			addrs := GenerateAddrs(3)

			ab.AddAddrs(tid, id, addrs, time.Second)

			// same address as before but with a higher TTL
			ab.AddAddrs(tid, id, addrs[2:], time.Hour)

			// after the initial TTL has expired, check that only the third address is present.
			time.Sleep(1200 * time.Millisecond)
			AssertAddressesEqual(t, addrs[2:], ab.Addrs(tid, id))

			// make sure we actually set the TTL
			ab.UpdateAddrs(tid, id, time.Hour, 0)
			AssertAddressesEqual(t, nil, ab.Addrs(tid, id))
		})

		t.Run("adding an existing address with an earlier expiration never reduces the expiration", func(t *testing.T) {
			id := GeneratePeerIDs(1)[0]
			addrs := GenerateAddrs(3)

			ab.AddAddrs(tid, id, addrs, time.Hour)

			// same address as before but with a lower TTL
			ab.AddAddrs(tid, id, addrs[2:], time.Second)

			// after the initial TTL has expired, check that all three addresses are still present (i.e. the TTL on
			// the modified one was not shortened).
			time.Sleep(2100 * time.Millisecond)
			AssertAddressesEqual(t, addrs, ab.Addrs(tid, id))
		})

		t.Run("adding an existing address with an earlier expiration never reduces the TTL", func(t *testing.T) {
			id := GeneratePeerIDs(1)[0]
			addrs := GenerateAddrs(1)

			ab.AddAddrs(tid, id, addrs, 4*time.Second)
			// 4 seconds left
			time.Sleep(3 * time.Second)
			// 1 second left
			ab.AddAddrs(tid, id, addrs, 3*time.Second)
			// 3 seconds left
			time.Sleep(2)
			// 1 seconds left.

			// We still have the address.
			AssertAddressesEqual(t, addrs, ab.Addrs(tid, id))

			// The TTL wasn't reduced
			ab.UpdateAddrs(tid, id, 4*time.Second, 0)
			AssertAddressesEqual(t, nil, ab.Addrs(tid, id))
		})
	}
}

func testClearWorks(ab tstore.LogAddrBook) func(t *testing.T) {
	return func(t *testing.T) {
		tid := thread.NewIDV1(thread.Raw, 24)

		ids := GeneratePeerIDs(2)
		addrs := GenerateAddrs(5)

		ab.AddAddrs(tid, ids[0], addrs[0:3], time.Hour)
		ab.AddAddrs(tid, ids[1], addrs[3:], time.Hour)

		AssertAddressesEqual(t, addrs[0:3], ab.Addrs(tid, ids[0]))
		AssertAddressesEqual(t, addrs[3:], ab.Addrs(tid, ids[1]))

		ab.ClearAddrs(tid, ids[0])
		AssertAddressesEqual(t, nil, ab.Addrs(tid, ids[0]))
		AssertAddressesEqual(t, addrs[3:], ab.Addrs(tid, ids[1]))

		ab.ClearAddrs(tid, ids[1])
		AssertAddressesEqual(t, nil, ab.Addrs(tid, ids[0]))
		AssertAddressesEqual(t, nil, ab.Addrs(tid, ids[1]))
	}
}

func testSetNegativeTTLClears(ab tstore.LogAddrBook) func(t *testing.T) {
	return func(t *testing.T) {
		tid := thread.NewIDV1(thread.Raw, 24)

		id := GeneratePeerIDs(1)[0]
		addrs := GenerateAddrs(100)

		ab.SetAddrs(tid, id, addrs, time.Hour)
		AssertAddressesEqual(t, addrs, ab.Addrs(tid, id))

		// remove two addresses.
		ab.SetAddr(tid, id, addrs[50], -1)
		ab.SetAddr(tid, id, addrs[75], -1)

		// calculate the survivors
		survivors := append(addrs[0:50], addrs[51:]...)
		survivors = append(survivors[0:74], survivors[75:]...)

		AssertAddressesEqual(t, survivors, ab.Addrs(tid, id))
	}
}

func testUpdateTTLs(ab tstore.LogAddrBook) func(t *testing.T) {
	return func(t *testing.T) {
		tid := thread.NewIDV1(thread.Raw, 24)

		t.Run("update ttl of log with no addrs", func(t *testing.T) {
			id := GeneratePeerIDs(1)[0]

			// Shouldn't panic.
			ab.UpdateAddrs(tid, id, time.Hour, time.Minute)
		})

		t.Run("update ttls successfully", func(t *testing.T) {
			ids := GeneratePeerIDs(2)
			addrs1, addrs2 := GenerateAddrs(2), GenerateAddrs(2)

			// set two keys with different ttls for each log.
			ab.SetAddr(tid, ids[0], addrs1[0], time.Hour)
			ab.SetAddr(tid, ids[0], addrs1[1], time.Minute)
			ab.SetAddr(tid, ids[1], addrs2[0], time.Hour)
			ab.SetAddr(tid, ids[1], addrs2[1], time.Minute)

			// Sanity check.
			AssertAddressesEqual(t, addrs1, ab.Addrs(tid, ids[0]))
			AssertAddressesEqual(t, addrs2, ab.Addrs(tid, ids[1]))

			// Will only affect addrs1[0].
			// Badger does not support subsecond TTLs.
			// https://github.com/dgraph-io/badger/issues/339
			ab.UpdateAddrs(tid, ids[0], time.Hour, 1*time.Second)

			// No immediate effect.
			AssertAddressesEqual(t, addrs1, ab.Addrs(tid, ids[0]))
			AssertAddressesEqual(t, addrs2, ab.Addrs(tid, ids[1]))

			// After a wait, addrs[0] is gone.
			time.Sleep(1500 * time.Millisecond)
			AssertAddressesEqual(t, addrs1[1:2], ab.Addrs(tid, ids[0]))
			AssertAddressesEqual(t, addrs2, ab.Addrs(tid, ids[1]))

			// Will only affect addrs2[0].
			ab.UpdateAddrs(tid, ids[1], time.Hour, 1*time.Second)

			// No immediate effect.
			AssertAddressesEqual(t, addrs1[1:2], ab.Addrs(tid, ids[0]))
			AssertAddressesEqual(t, addrs2, ab.Addrs(tid, ids[1]))

			time.Sleep(1500 * time.Millisecond)

			// First addrs is gone in both.
			AssertAddressesEqual(t, addrs1[1:], ab.Addrs(tid, ids[0]))
			AssertAddressesEqual(t, addrs2[1:], ab.Addrs(tid, ids[1]))
		})

	}
}

func testNilAddrsDontBreak(ab tstore.LogAddrBook) func(t *testing.T) {
	return func(t *testing.T) {
		tid := thread.NewIDV1(thread.Raw, 24)

		id := GeneratePeerIDs(1)[0]

		ab.SetAddr(tid, id, nil, time.Hour)
		ab.AddAddr(tid, id, nil, time.Hour)
	}
}

func testAddressesExpire(ab tstore.LogAddrBook) func(t *testing.T) {
	return func(t *testing.T) {
		tid := thread.NewIDV1(thread.Raw, 24)

		ids := GeneratePeerIDs(2)
		addrs1 := GenerateAddrs(3)
		addrs2 := GenerateAddrs(2)

		ab.AddAddrs(tid, ids[0], addrs1, time.Hour)
		ab.AddAddrs(tid, ids[1], addrs2, time.Hour)

		AssertAddressesEqual(t, addrs1, ab.Addrs(tid, ids[0]))
		AssertAddressesEqual(t, addrs2, ab.Addrs(tid, ids[1]))

		ab.AddAddrs(tid, ids[0], addrs1, 2*time.Hour)
		ab.AddAddrs(tid, ids[1], addrs2, 2*time.Hour)

		AssertAddressesEqual(t, addrs1, ab.Addrs(tid, ids[0]))
		AssertAddressesEqual(t, addrs2, ab.Addrs(tid, ids[1]))

		ab.SetAddr(tid, ids[0], addrs1[0], 100*time.Microsecond)
		<-time.After(100 * time.Millisecond)
		AssertAddressesEqual(t, addrs1[1:3], ab.Addrs(tid, ids[0]))
		AssertAddressesEqual(t, addrs2, ab.Addrs(tid, ids[1]))

		ab.SetAddr(tid, ids[0], addrs1[2], 100*time.Microsecond)
		<-time.After(100 * time.Millisecond)
		AssertAddressesEqual(t, addrs1[1:2], ab.Addrs(tid, ids[0]))
		AssertAddressesEqual(t, addrs2, ab.Addrs(tid, ids[1]))

		ab.SetAddr(tid, ids[1], addrs2[0], 100*time.Microsecond)
		<-time.After(100 * time.Millisecond)
		AssertAddressesEqual(t, addrs1[1:2], ab.Addrs(tid, ids[0]))
		AssertAddressesEqual(t, addrs2[1:], ab.Addrs(tid, ids[1]))

		ab.SetAddr(tid, ids[1], addrs2[1], 100*time.Microsecond)
		<-time.After(100 * time.Millisecond)
		AssertAddressesEqual(t, addrs1[1:2], ab.Addrs(tid, ids[0]))
		AssertAddressesEqual(t, nil, ab.Addrs(tid, ids[1]))

		ab.SetAddr(tid, ids[0], addrs1[1], 100*time.Microsecond)
		<-time.After(100 * time.Millisecond)
		AssertAddressesEqual(t, nil, ab.Addrs(tid, ids[0]))
		AssertAddressesEqual(t, nil, ab.Addrs(tid, ids[1]))
	}
}

func testClearWithIterator(ab tstore.LogAddrBook) func(t *testing.T) {
	return func(t *testing.T) {
		tid := thread.NewIDV1(thread.Raw, 24)

		ids := GeneratePeerIDs(2)
		addrs := GenerateAddrs(100)

		// Add the logs with 50 addresses each.
		ab.AddAddrs(tid, ids[0], addrs[:50], pstore.PermanentAddrTTL)
		ab.AddAddrs(tid, ids[1], addrs[50:], pstore.PermanentAddrTTL)

		if all := append(ab.Addrs(tid, ids[0]), ab.Addrs(tid, ids[1])...); len(all) != 100 {
			t.Fatal("expected tstore to contain both logs with all their maddrs")
		}

		// Since we don't fetch these logs, they won't be present in cache.

		ab.ClearAddrs(tid, ids[0])
		if all := append(ab.Addrs(tid, ids[0]), ab.Addrs(tid, ids[1])...); len(all) != 50 {
			t.Fatal("expected tstore to contain only addrs of log 2")
		}

		ab.ClearAddrs(tid, ids[1])
		if all := append(ab.Addrs(tid, ids[0]), ab.Addrs(tid, ids[1])...); len(all) != 0 {
			t.Fatal("expected tstore to contain no addresses")
		}
	}
}

func testLogsWithAddrs(ab tstore.LogAddrBook) func(t *testing.T) {
	return func(t *testing.T) {
		tid := thread.NewIDV1(thread.Raw, 24)

		// cannot run in parallel as the store is modified.
		// go runs sequentially in the specified order
		// see https://blog.golang.org/subtests

		t.Run("empty addrbook", func(t *testing.T) {
			if logs := ab.LogsWithAddrs(tid); len(logs) != 0 {
				t.Fatal("expected to find no logs")
			}
		})

		t.Run("non-empty addrbook", func(t *testing.T) {
			ids := GeneratePeerIDs(2)
			addrs := GenerateAddrs(10)

			ab.AddAddrs(tid, ids[0], addrs[:5], pstore.PermanentAddrTTL)
			ab.AddAddrs(tid, ids[1], addrs[5:], pstore.PermanentAddrTTL)

			if logs := ab.LogsWithAddrs(tid); len(logs) != 2 {
				t.Fatal("expected to find 2 logs")
			}
		})
	}
}
