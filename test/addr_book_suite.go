package test

import (
	"testing"
	"time"

	pstore "github.com/libp2p/go-libp2p-core/peerstore"
)

var addressBookSuite = map[string]func(book pstore.AddrBook) func(*testing.T){
	"AddAddress":           testAddAddress,
	"Clear":                testClearWorks,
	"SetNegativeTTLClears": testSetNegativeTTLClears,
	"UpdateTTLs":           testUpdateTTLs,
	"NilAddrsDontBreak":    testNilAddrsDontBreak,
	"AddressesExpire":      testAddressesExpire,
	"ClearWithIter":        testClearWithIterator,
	"PeersWithAddresses":   testPeersWithAddrs,
}

type AddrBookFactory func() (pstore.AddrBook, func())

func TestAddrBook(t *testing.T, factory AddrBookFactory) {
	for name, test := range addressBookSuite {
		// Create a new peerstore.
		ab, closeFunc := factory()

		// Run the test.
		t.Run(name, test(ab))

		// Cleanup.
		if closeFunc != nil {
			closeFunc()
		}
	}
}

func testAddAddress(ab pstore.AddrBook) func(*testing.T) {
	return func(t *testing.T) {
		t.Run("add a single address", func(t *testing.T) {
			id := GeneratePeerIDs(1)[0]
			addrs := GenerateAddrs(1)

			ab.AddAddr(id, addrs[0], time.Hour)

			AssertAddressesEqual(t, addrs, ab.Addrs(id))
		})

		t.Run("idempotent add single address", func(t *testing.T) {
			id := GeneratePeerIDs(1)[0]
			addrs := GenerateAddrs(1)

			ab.AddAddr(id, addrs[0], time.Hour)
			ab.AddAddr(id, addrs[0], time.Hour)

			AssertAddressesEqual(t, addrs, ab.Addrs(id))
		})

		t.Run("add multiple addresses", func(t *testing.T) {
			id := GeneratePeerIDs(1)[0]
			addrs := GenerateAddrs(3)

			ab.AddAddrs(id, addrs, time.Hour)
			AssertAddressesEqual(t, addrs, ab.Addrs(id))
		})

		t.Run("idempotent add multiple addresses", func(t *testing.T) {
			id := GeneratePeerIDs(1)[0]
			addrs := GenerateAddrs(3)

			ab.AddAddrs(id, addrs, time.Hour)
			ab.AddAddrs(id, addrs, time.Hour)

			AssertAddressesEqual(t, addrs, ab.Addrs(id))
		})

		t.Run("adding an existing address with a later expiration extends its ttl", func(t *testing.T) {
			id := GeneratePeerIDs(1)[0]
			addrs := GenerateAddrs(3)

			ab.AddAddrs(id, addrs, time.Second)

			// same address as before but with a higher TTL
			ab.AddAddrs(id, addrs[2:], time.Hour)

			// after the initial TTL has expired, check that only the third address is present.
			time.Sleep(1200 * time.Millisecond)
			AssertAddressesEqual(t, addrs[2:], ab.Addrs(id))

			// make sure we actually set the TTL
			ab.UpdateAddrs(id, time.Hour, 0)
			AssertAddressesEqual(t, nil, ab.Addrs(id))
		})

		t.Run("adding an existing address with an earlier expiration never reduces the expiration", func(t *testing.T) {
			id := GeneratePeerIDs(1)[0]
			addrs := GenerateAddrs(3)

			ab.AddAddrs(id, addrs, time.Hour)

			// same address as before but with a lower TTL
			ab.AddAddrs(id, addrs[2:], time.Second)

			// after the initial TTL has expired, check that all three addresses are still present (i.e. the TTL on
			// the modified one was not shortened).
			time.Sleep(2100 * time.Millisecond)
			AssertAddressesEqual(t, addrs, ab.Addrs(id))
		})

		t.Run("adding an existing address with an earlier expiration never reduces the TTL", func(t *testing.T) {
			id := GeneratePeerIDs(1)[0]
			addrs := GenerateAddrs(1)

			ab.AddAddrs(id, addrs, 4*time.Second)
			// 4 seconds left
			time.Sleep(3 * time.Second)
			// 1 second left
			ab.AddAddrs(id, addrs, 3*time.Second)
			// 3 seconds left
			time.Sleep(2)
			// 1 seconds left.

			// We still have the address.
			AssertAddressesEqual(t, addrs, ab.Addrs(id))

			// The TTL wasn't reduced
			ab.UpdateAddrs(id, 4*time.Second, 0)
			AssertAddressesEqual(t, nil, ab.Addrs(id))
		})
	}
}

func testClearWorks(ab pstore.AddrBook) func(t *testing.T) {
	return func(t *testing.T) {
		ids := GeneratePeerIDs(2)
		addrs := GenerateAddrs(5)

		ab.AddAddrs(ids[0], addrs[0:3], time.Hour)
		ab.AddAddrs(ids[1], addrs[3:], time.Hour)

		AssertAddressesEqual(t, addrs[0:3], ab.Addrs(ids[0]))
		AssertAddressesEqual(t, addrs[3:], ab.Addrs(ids[1]))

		ab.ClearAddrs(ids[0])
		AssertAddressesEqual(t, nil, ab.Addrs(ids[0]))
		AssertAddressesEqual(t, addrs[3:], ab.Addrs(ids[1]))

		ab.ClearAddrs(ids[1])
		AssertAddressesEqual(t, nil, ab.Addrs(ids[0]))
		AssertAddressesEqual(t, nil, ab.Addrs(ids[1]))
	}
}

func testSetNegativeTTLClears(m pstore.AddrBook) func(t *testing.T) {
	return func(t *testing.T) {
		id := GeneratePeerIDs(1)[0]
		addrs := GenerateAddrs(100)

		m.SetAddrs(id, addrs, time.Hour)
		AssertAddressesEqual(t, addrs, m.Addrs(id))

		// remove two addresses.
		m.SetAddr(id, addrs[50], -1)
		m.SetAddr(id, addrs[75], -1)

		// calculate the survivors
		survivors := append(addrs[0:50], addrs[51:]...)
		survivors = append(survivors[0:74], survivors[75:]...)

		AssertAddressesEqual(t, survivors, m.Addrs(id))
	}
}

func testUpdateTTLs(m pstore.AddrBook) func(t *testing.T) {
	return func(t *testing.T) {
		t.Run("update ttl of peer with no addrs", func(t *testing.T) {
			id := GeneratePeerIDs(1)[0]

			// Shouldn't panic.
			m.UpdateAddrs(id, time.Hour, time.Minute)
		})

		t.Run("update ttls successfully", func(t *testing.T) {
			ids := GeneratePeerIDs(2)
			addrs1, addrs2 := GenerateAddrs(2), GenerateAddrs(2)

			// set two keys with different ttls for each peer.
			m.SetAddr(ids[0], addrs1[0], time.Hour)
			m.SetAddr(ids[0], addrs1[1], time.Minute)
			m.SetAddr(ids[1], addrs2[0], time.Hour)
			m.SetAddr(ids[1], addrs2[1], time.Minute)

			// Sanity check.
			AssertAddressesEqual(t, addrs1, m.Addrs(ids[0]))
			AssertAddressesEqual(t, addrs2, m.Addrs(ids[1]))

			// Will only affect addrs1[0].
			// Badger does not support subsecond TTLs.
			// https://github.com/dgraph-io/badger/issues/339
			m.UpdateAddrs(ids[0], time.Hour, 1*time.Second)

			// No immediate effect.
			AssertAddressesEqual(t, addrs1, m.Addrs(ids[0]))
			AssertAddressesEqual(t, addrs2, m.Addrs(ids[1]))

			// After a wait, addrs[0] is gone.
			time.Sleep(1500 * time.Millisecond)
			AssertAddressesEqual(t, addrs1[1:2], m.Addrs(ids[0]))
			AssertAddressesEqual(t, addrs2, m.Addrs(ids[1]))

			// Will only affect addrs2[0].
			m.UpdateAddrs(ids[1], time.Hour, 1*time.Second)

			// No immediate effect.
			AssertAddressesEqual(t, addrs1[1:2], m.Addrs(ids[0]))
			AssertAddressesEqual(t, addrs2, m.Addrs(ids[1]))

			time.Sleep(1500 * time.Millisecond)

			// First addrs is gone in both.
			AssertAddressesEqual(t, addrs1[1:], m.Addrs(ids[0]))
			AssertAddressesEqual(t, addrs2[1:], m.Addrs(ids[1]))
		})

	}
}

func testNilAddrsDontBreak(m pstore.AddrBook) func(t *testing.T) {
	return func(t *testing.T) {
		id := GeneratePeerIDs(1)[0]

		m.SetAddr(id, nil, time.Hour)
		m.AddAddr(id, nil, time.Hour)
	}
}

func testAddressesExpire(m pstore.AddrBook) func(t *testing.T) {
	return func(t *testing.T) {
		ids := GeneratePeerIDs(2)
		addrs1 := GenerateAddrs(3)
		addrs2 := GenerateAddrs(2)

		m.AddAddrs(ids[0], addrs1, time.Hour)
		m.AddAddrs(ids[1], addrs2, time.Hour)

		AssertAddressesEqual(t, addrs1, m.Addrs(ids[0]))
		AssertAddressesEqual(t, addrs2, m.Addrs(ids[1]))

		m.AddAddrs(ids[0], addrs1, 2*time.Hour)
		m.AddAddrs(ids[1], addrs2, 2*time.Hour)

		AssertAddressesEqual(t, addrs1, m.Addrs(ids[0]))
		AssertAddressesEqual(t, addrs2, m.Addrs(ids[1]))

		m.SetAddr(ids[0], addrs1[0], 100*time.Microsecond)
		<-time.After(100 * time.Millisecond)
		AssertAddressesEqual(t, addrs1[1:3], m.Addrs(ids[0]))
		AssertAddressesEqual(t, addrs2, m.Addrs(ids[1]))

		m.SetAddr(ids[0], addrs1[2], 100*time.Microsecond)
		<-time.After(100 * time.Millisecond)
		AssertAddressesEqual(t, addrs1[1:2], m.Addrs(ids[0]))
		AssertAddressesEqual(t, addrs2, m.Addrs(ids[1]))

		m.SetAddr(ids[1], addrs2[0], 100*time.Microsecond)
		<-time.After(100 * time.Millisecond)
		AssertAddressesEqual(t, addrs1[1:2], m.Addrs(ids[0]))
		AssertAddressesEqual(t, addrs2[1:], m.Addrs(ids[1]))

		m.SetAddr(ids[1], addrs2[1], 100*time.Microsecond)
		<-time.After(100 * time.Millisecond)
		AssertAddressesEqual(t, addrs1[1:2], m.Addrs(ids[0]))
		AssertAddressesEqual(t, nil, m.Addrs(ids[1]))

		m.SetAddr(ids[0], addrs1[1], 100*time.Microsecond)
		<-time.After(100 * time.Millisecond)
		AssertAddressesEqual(t, nil, m.Addrs(ids[0]))
		AssertAddressesEqual(t, nil, m.Addrs(ids[1]))
	}
}

func testClearWithIterator(m pstore.AddrBook) func(t *testing.T) {
	return func(t *testing.T) {
		ids := GeneratePeerIDs(2)
		addrs := GenerateAddrs(100)

		// Add the peers with 50 addresses each.
		m.AddAddrs(ids[0], addrs[:50], pstore.PermanentAddrTTL)
		m.AddAddrs(ids[1], addrs[50:], pstore.PermanentAddrTTL)

		if all := append(m.Addrs(ids[0]), m.Addrs(ids[1])...); len(all) != 100 {
			t.Fatal("expected pstore to contain both peers with all their maddrs")
		}

		// Since we don't fetch these peers, they won't be present in cache.

		m.ClearAddrs(ids[0])
		if all := append(m.Addrs(ids[0]), m.Addrs(ids[1])...); len(all) != 50 {
			t.Fatal("expected pstore to contain only addrs of peer 2")
		}

		m.ClearAddrs(ids[1])
		if all := append(m.Addrs(ids[0]), m.Addrs(ids[1])...); len(all) != 0 {
			t.Fatal("expected pstore to contain no addresses")
		}
	}
}

func testPeersWithAddrs(m pstore.AddrBook) func(t *testing.T) {
	return func(t *testing.T) {
		// cannot run in parallel as the store is modified.
		// go runs sequentially in the specified order
		// see https://blog.golang.org/subtests

		t.Run("empty addrbook", func(t *testing.T) {
			if peers := m.PeersWithAddrs(); len(peers) != 0 {
				t.Fatal("expected to find no peers")
			}
		})

		t.Run("non-empty addrbook", func(t *testing.T) {
			ids := GeneratePeerIDs(2)
			addrs := GenerateAddrs(10)

			m.AddAddrs(ids[0], addrs[:5], pstore.PermanentAddrTTL)
			m.AddAddrs(ids[1], addrs[5:], pstore.PermanentAddrTTL)

			if peers := m.PeersWithAddrs(); len(peers) != 2 {
				t.Fatal("expected to find 2 peers")
			}
		})
	}
}
