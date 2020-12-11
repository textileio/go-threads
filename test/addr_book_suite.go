package test

import (
	"testing"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
	pstore "github.com/libp2p/go-libp2p-core/peerstore"
	ma "github.com/multiformats/go-multiaddr"
	core "github.com/textileio/go-threads/core/logstore"
	"github.com/textileio/go-threads/core/thread"
)

var addressBookSuite = map[string]func(book core.AddrBook) func(*testing.T){
	"AddAddress":           testAddAddress,
	"Clear":                testClearWorks,
	"SetNegativeTTLClears": testSetNegativeTTLClears,
	"UpdateTTLs":           testUpdateTTLs,
	"NilAddrsDontBreak":    testNilAddrsDontBreak,
	"AddressesExpire":      testAddressesExpire,
	"ClearWithIter":        testClearWithIterator,
	"LogsWithAddresses":    testLogsWithAddrs,
	"ThreadsWithAddresses": testThreadsFromAddrs,
	"ExportAddressBook":    testExportAddressBook,
}

type AddrBookFactory func() (core.AddrBook, func())

func AddrBookTest(t *testing.T, factory AddrBookFactory) {
	for name, test := range addressBookSuite {
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

func testAddAddress(ab core.AddrBook) func(*testing.T) {
	return func(t *testing.T) {
		tid := thread.NewIDV1(thread.Raw, 24)

		t.Run("add a single address", func(t *testing.T) {
			id := GeneratePeerIDs(1)[0]
			addrs := GenerateAddrs(1)

			check(t, ab.AddAddr(tid, id, addrs[0], time.Hour))
			AssertAddressesEqual(t, addrs, checkedAddrs(t, ab, tid, id))
		})

		t.Run("idempotent add single address", func(t *testing.T) {
			id := GeneratePeerIDs(1)[0]
			addrs := GenerateAddrs(1)

			check(t, ab.AddAddr(tid, id, addrs[0], time.Hour))
			check(t, ab.AddAddr(tid, id, addrs[0], time.Hour))

			AssertAddressesEqual(t, addrs, checkedAddrs(t, ab, tid, id))
		})

		t.Run("add multiple addresses", func(t *testing.T) {
			id := GeneratePeerIDs(1)[0]
			addrs := GenerateAddrs(3)

			check(t, ab.AddAddrs(tid, id, addrs, time.Hour))
			AssertAddressesEqual(t, addrs, checkedAddrs(t, ab, tid, id))
		})

		t.Run("idempotent add multiple addresses", func(t *testing.T) {
			id := GeneratePeerIDs(1)[0]
			addrs := GenerateAddrs(3)

			check(t, ab.AddAddrs(tid, id, addrs, time.Hour))
			check(t, ab.AddAddrs(tid, id, addrs, time.Hour))

			AssertAddressesEqual(t, addrs, checkedAddrs(t, ab, tid, id))
		})

		t.Run("adding an existing address with a later expiration extends its ttl", func(t *testing.T) {
			id := GeneratePeerIDs(1)[0]
			addrs := GenerateAddrs(3)

			check(t, ab.AddAddrs(tid, id, addrs, time.Second))

			// same address as before but with a higher TTL
			check(t, ab.AddAddrs(tid, id, addrs[2:], time.Hour))

			// after the initial TTL has expired, check that only the third address is present.
			time.Sleep(1200 * time.Millisecond)
			AssertAddressesEqual(t, addrs[2:], checkedAddrs(t, ab, tid, id))

			// make sure we actually set the TTL
			check(t, ab.UpdateAddrs(tid, id, time.Hour, 0))

			AssertAddressesEqual(t, nil, checkedAddrs(t, ab, tid, id))
		})

		t.Run("adding an existing address with an earlier expiration never reduces the expiration", func(t *testing.T) {
			id := GeneratePeerIDs(1)[0]
			addrs := GenerateAddrs(3)

			check(t, ab.AddAddrs(tid, id, addrs, time.Hour))

			// same address as before but with a lower TTL
			check(t, ab.AddAddrs(tid, id, addrs[2:], time.Second))

			// after the initial TTL has expired, check that all three addresses are still present (i.e. the TTL on
			// the modified one was not shortened).
			time.Sleep(2100 * time.Millisecond)
			AssertAddressesEqual(t, addrs, checkedAddrs(t, ab, tid, id))
		})

		t.Run("adding an existing address with an earlier expiration never reduces the TTL", func(t *testing.T) {
			id := GeneratePeerIDs(1)[0]
			addrs := GenerateAddrs(1)

			check(t, ab.AddAddrs(tid, id, addrs, 4*time.Second))
			// 4 seconds left
			time.Sleep(3 * time.Second)
			// 1 second left
			check(t, ab.AddAddrs(tid, id, addrs, 3*time.Second))
			// 3 seconds left
			time.Sleep(2 * time.Second)
			// 1 seconds left.

			// We still have the address.
			AssertAddressesEqual(t, addrs, checkedAddrs(t, ab, tid, id))

			// The TTL wasn't reduced
			check(t, ab.UpdateAddrs(tid, id, 4*time.Second, 0))
			AssertAddressesEqual(t, nil, checkedAddrs(t, ab, tid, id))
		})
	}
}

func testClearWorks(ab core.AddrBook) func(t *testing.T) {
	return func(t *testing.T) {
		tid := thread.NewIDV1(thread.Raw, 24)

		ids := GeneratePeerIDs(2)
		addrs := GenerateAddrs(5)

		check(t, ab.AddAddrs(tid, ids[0], addrs[0:3], time.Hour))
		check(t, ab.AddAddrs(tid, ids[1], addrs[3:], time.Hour))

		AssertAddressesEqual(t, addrs[0:3], checkedAddrs(t, ab, tid, ids[0]))
		AssertAddressesEqual(t, addrs[3:], checkedAddrs(t, ab, tid, ids[1]))

		check(t, ab.ClearAddrs(tid, ids[0]))
		AssertAddressesEqual(t, nil, checkedAddrs(t, ab, tid, ids[0]))
		AssertAddressesEqual(t, addrs[3:], checkedAddrs(t, ab, tid, ids[1]))

		check(t, ab.ClearAddrs(tid, ids[1]))
		AssertAddressesEqual(t, nil, checkedAddrs(t, ab, tid, ids[0]))
		AssertAddressesEqual(t, nil, checkedAddrs(t, ab, tid, ids[1]))
	}
}

func testSetNegativeTTLClears(ab core.AddrBook) func(t *testing.T) {
	return func(t *testing.T) {
		tid := thread.NewIDV1(thread.Raw, 24)

		id := GeneratePeerIDs(1)[0]
		addrs := GenerateAddrs(100)

		check(t, ab.SetAddrs(tid, id, addrs, time.Hour))
		AssertAddressesEqual(t, addrs, checkedAddrs(t, ab, tid, id))

		// remove two addresses.
		check(t, ab.SetAddr(tid, id, addrs[50], -1))
		check(t, ab.SetAddr(tid, id, addrs[75], -1))

		// calculate the survivors
		survivors := append(addrs[0:50], addrs[51:]...)
		survivors = append(survivors[0:74], survivors[75:]...)

		AssertAddressesEqual(t, survivors, checkedAddrs(t, ab, tid, id))
	}
}

func testUpdateTTLs(ab core.AddrBook) func(t *testing.T) {
	return func(t *testing.T) {
		tid := thread.NewIDV1(thread.Raw, 24)

		t.Run("update ttl of log with no addrs", func(t *testing.T) {
			id := GeneratePeerIDs(1)[0]

			// Shouldn't panic.
			check(t, ab.UpdateAddrs(tid, id, time.Hour, time.Minute))
		})

		t.Run("update ttls successfully", func(t *testing.T) {
			ids := GeneratePeerIDs(2)
			addrs1, addrs2 := GenerateAddrs(2), GenerateAddrs(2)

			// set two keys with different ttls for each log.
			check(t, ab.SetAddr(tid, ids[0], addrs1[0], time.Hour))
			check(t, ab.SetAddr(tid, ids[0], addrs1[1], time.Minute))
			check(t, ab.SetAddr(tid, ids[1], addrs2[0], time.Hour))
			check(t, ab.SetAddr(tid, ids[1], addrs2[1], time.Minute))

			// Sanity check.
			AssertAddressesEqual(t, addrs1, checkedAddrs(t, ab, tid, ids[0]))
			AssertAddressesEqual(t, addrs2, checkedAddrs(t, ab, tid, ids[1]))

			// Will only affect addrs1[0].
			// Badger does not support subsecond TTLs.
			// https://github.com/dgraph-io/badger/issues/339
			check(t, ab.UpdateAddrs(tid, ids[0], time.Hour, 1*time.Second))

			// No immediate effect.
			AssertAddressesEqual(t, addrs1, checkedAddrs(t, ab, tid, ids[0]))
			AssertAddressesEqual(t, addrs2, checkedAddrs(t, ab, tid, ids[1]))

			// After a wait, addrs[0] is gone.
			time.Sleep(1500 * time.Millisecond)
			AssertAddressesEqual(t, addrs1[1:2], checkedAddrs(t, ab, tid, ids[0]))
			AssertAddressesEqual(t, addrs2, checkedAddrs(t, ab, tid, ids[1]))

			// Will only affect addrs2[0].
			check(t, ab.UpdateAddrs(tid, ids[1], time.Hour, 1*time.Second))

			// No immediate effect.
			AssertAddressesEqual(t, addrs1[1:2], checkedAddrs(t, ab, tid, ids[0]))
			AssertAddressesEqual(t, addrs2, checkedAddrs(t, ab, tid, ids[1]))

			time.Sleep(1500 * time.Millisecond)

			// First addrs is gone in both.
			AssertAddressesEqual(t, addrs1[1:], checkedAddrs(t, ab, tid, ids[0]))
			AssertAddressesEqual(t, addrs2[1:], checkedAddrs(t, ab, tid, ids[1]))
		})

	}
}

func testNilAddrsDontBreak(ab core.AddrBook) func(t *testing.T) {
	return func(t *testing.T) {
		tid := thread.NewIDV1(thread.Raw, 24)

		id := GeneratePeerIDs(1)[0]

		check(t, ab.SetAddr(tid, id, nil, time.Hour))
		check(t, ab.AddAddr(tid, id, nil, time.Hour))
	}
}

func testAddressesExpire(ab core.AddrBook) func(t *testing.T) {
	return func(t *testing.T) {
		tid := thread.NewIDV1(thread.Raw, 24)

		ids := GeneratePeerIDs(2)
		addrs1 := GenerateAddrs(3)
		addrs2 := GenerateAddrs(2)

		check(t, ab.AddAddrs(tid, ids[0], addrs1, time.Hour))
		check(t, ab.AddAddrs(tid, ids[1], addrs2, time.Hour))

		AssertAddressesEqual(t, addrs1, checkedAddrs(t, ab, tid, ids[0]))
		AssertAddressesEqual(t, addrs2, checkedAddrs(t, ab, tid, ids[1]))

		check(t, ab.AddAddrs(tid, ids[0], addrs1, 2*time.Hour))
		check(t, ab.AddAddrs(tid, ids[1], addrs2, 2*time.Hour))

		AssertAddressesEqual(t, addrs1, checkedAddrs(t, ab, tid, ids[0]))
		AssertAddressesEqual(t, addrs2, checkedAddrs(t, ab, tid, ids[1]))

		check(t, ab.SetAddr(tid, ids[0], addrs1[0], 100*time.Microsecond))
		<-time.After(100 * time.Millisecond)
		AssertAddressesEqual(t, addrs1[1:3], checkedAddrs(t, ab, tid, ids[0]))
		AssertAddressesEqual(t, addrs2, checkedAddrs(t, ab, tid, ids[1]))

		check(t, ab.SetAddr(tid, ids[0], addrs1[2], 100*time.Microsecond))
		<-time.After(100 * time.Millisecond)
		AssertAddressesEqual(t, addrs1[1:2], checkedAddrs(t, ab, tid, ids[0]))
		AssertAddressesEqual(t, addrs2, checkedAddrs(t, ab, tid, ids[1]))

		check(t, ab.SetAddr(tid, ids[1], addrs2[0], 100*time.Microsecond))
		<-time.After(100 * time.Millisecond)
		AssertAddressesEqual(t, addrs1[1:2], checkedAddrs(t, ab, tid, ids[0]))
		AssertAddressesEqual(t, addrs2[1:], checkedAddrs(t, ab, tid, ids[1]))

		check(t, ab.SetAddr(tid, ids[1], addrs2[1], 100*time.Microsecond))
		<-time.After(100 * time.Millisecond)
		AssertAddressesEqual(t, addrs1[1:2], checkedAddrs(t, ab, tid, ids[0]))
		AssertAddressesEqual(t, nil, checkedAddrs(t, ab, tid, ids[1]))

		check(t, ab.SetAddr(tid, ids[0], addrs1[1], 100*time.Microsecond))
		<-time.After(100 * time.Millisecond)
		AssertAddressesEqual(t, nil, checkedAddrs(t, ab, tid, ids[0]))
		AssertAddressesEqual(t, nil, checkedAddrs(t, ab, tid, ids[1]))
	}
}

func testClearWithIterator(ab core.AddrBook) func(t *testing.T) {
	return func(t *testing.T) {
		tid := thread.NewIDV1(thread.Raw, 24)

		ids := GeneratePeerIDs(2)
		addrs := GenerateAddrs(100)

		// Add the logs with 50 addresses each.
		check(t, ab.AddAddrs(tid, ids[0], addrs[:50], pstore.PermanentAddrTTL))
		check(t, ab.AddAddrs(tid, ids[1], addrs[50:], pstore.PermanentAddrTTL))

		if all := append(checkedAddrs(t, ab, tid, ids[0]), checkedAddrs(t, ab, tid, ids[1])...); len(all) != 100 {
			t.Fatal("expected tstore to contain both logs with all their maddrs")
		}

		// Since we don't fetch these logs, they won't be present in cache.

		check(t, ab.ClearAddrs(tid, ids[0]))
		if all := append(checkedAddrs(t, ab, tid, ids[0]), checkedAddrs(t, ab, tid, ids[1])...); len(all) != 50 {
			t.Fatal("expected tstore to contain only addrs of log 2")
		}

		check(t, ab.ClearAddrs(tid, ids[1]))
		if all := append(checkedAddrs(t, ab, tid, ids[0]), checkedAddrs(t, ab, tid, ids[1])...); len(all) != 0 {
			t.Fatal("expected tstore to contain no addresses")
		}
	}
}

func testLogsWithAddrs(ab core.AddrBook) func(t *testing.T) {
	return func(t *testing.T) {
		tid := thread.NewIDV1(thread.Raw, 24)

		// cannot run in parallel as the store is modified.
		// go runs sequentially in the specified order
		// see https://blog.golang.org/subtests

		t.Run("empty addrbook", func(t *testing.T) {
			if logs, err := ab.LogsWithAddrs(tid); err != nil || len(logs) != 0 {
				t.Fatal("expected to find no logs without errors")
			}
		})

		t.Run("non-empty addrbook", func(t *testing.T) {
			ids := GeneratePeerIDs(2)
			addrs := GenerateAddrs(10)

			err := ab.AddAddrs(tid, ids[0], addrs[:5], pstore.PermanentAddrTTL)
			check(t, err)
			err = ab.AddAddrs(tid, ids[1], addrs[5:], pstore.PermanentAddrTTL)
			check(t, err)

			if logs, err := ab.LogsWithAddrs(tid); err != nil || len(logs) != 2 {
				t.Fatal("expected to find 2 logs without errors")
			}
		})
	}
}

func testThreadsFromAddrs(ab core.AddrBook) func(t *testing.T) {
	return func(t *testing.T) {
		// cannot run in parallel as the store is modified.
		// go runs sequentially in the specified order
		// see https://blog.golang.org/subtests

		t.Run("empty addrbook", func(t *testing.T) {
			if logs, err := ab.ThreadsFromAddrs(); err != nil || len(logs) != 0 {
				t.Fatal("expected to find no threads without errors")
			}
		})

		t.Run("non-empty addrbook", func(t *testing.T) {
			tids := make([]thread.ID, 3)
			for i := range tids {
				tids[i] = thread.NewIDV1(thread.Raw, 24)
				ids := GeneratePeerIDs(2)
				addrs := GenerateAddrs(4)

				err := ab.AddAddrs(tids[i], ids[0], addrs[:2], pstore.PermanentAddrTTL)
				check(t, err)
				err = ab.AddAddrs(tids[i], ids[1], addrs[2:], pstore.PermanentAddrTTL)
				check(t, err)
			}

			if threads, err := ab.ThreadsFromAddrs(); err != nil || len(threads) != len(tids) {
				t.Fatalf("expected to find %d threads without errors, got %d with err: %v", len(tids), len(threads), err)
			}
		})
	}
}

func testExportAddressBook(ab core.AddrBook) func(*testing.T) {
	return func(t *testing.T) {
		tid := thread.NewIDV1(thread.Raw, 24)

		t.Run("dump and restore", func(t *testing.T) {
			ids := GeneratePeerIDs(2)
			addrs := GenerateAddrs(2)

			check(t, ab.AddAddr(tid, ids[0], addrs[0], time.Hour))
			check(t, ab.AddAddr(tid, ids[1], addrs[1], time.Hour))

			dump, err := ab.DumpAddrs()
			check(t, err)

			check(t, ab.ClearAddrs(tid, ids[0]))
			check(t, ab.ClearAddrs(tid, ids[1]))

			check(t, ab.RestoreAddrs(dump))

			AssertAddressesEqual(t, addrs[:1], checkedAddrs(t, ab, tid, ids[0]))
			AssertAddressesEqual(t, addrs[1:], checkedAddrs(t, ab, tid, ids[1]))
		})
	}
}

func checkedAddrs(t *testing.T, ab core.AddrBook, tid thread.ID, id peer.ID) []ma.Multiaddr {
	addrs, err := ab.Addrs(tid, id)
	if err != nil {
		t.Fatal("error when getting addresses")
	}
	return addrs
}
