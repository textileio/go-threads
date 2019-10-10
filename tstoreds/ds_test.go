package tstoreds

import (
	"context"
	"io/ioutil"
	"os"
	"testing"
	"time"

	ds "github.com/ipfs/go-datastore"
	badger "github.com/ipfs/go-ds-badger"
	leveldb "github.com/ipfs/go-ds-leveldb"
	tstore "github.com/textileio/go-textile-core/threadstore"
	pt "github.com/textileio/go-textile-threads/test"
)

type datastoreFactory func(tb testing.TB) (ds.Batching, func())

var dstores = map[string]datastoreFactory{
	"Badger":  badgerStore,
	"Leveldb": leveldbStore,
}

func TestDatastoreAddrBook(t *testing.T) {
	for name, dsFactory := range dstores {
		t.Run(name+" Cacheful", func(t *testing.T) {
			t.Parallel()
			opts := DefaultOpts()
			opts.GCPurgeInterval = 1 * time.Second
			opts.CacheSize = 1024

			pt.AddrBookTest(t, addressBookFactory(t, dsFactory, opts))
		})

		t.Run(name+" Cacheless", func(t *testing.T) {
			t.Parallel()
			opts := DefaultOpts()
			opts.GCPurgeInterval = 1 * time.Second
			opts.CacheSize = 0

			pt.AddrBookTest(t, addressBookFactory(t, dsFactory, opts))
		})
	}
}

func TestDatastoreKeyBook(t *testing.T) {
	for name, dsFactory := range dstores {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			pt.KeyBookTest(t, keyBookFactory(t, dsFactory))
		})
	}
}

func addressBookFactory(tb testing.TB, storeFactory datastoreFactory, opts Options) pt.AddrBookFactory {
	return func() (tstore.AddrBook, func()) {
		store, closeFunc := storeFactory(tb)
		ab, err := NewAddrBook(context.Background(), store, opts)
		if err != nil {
			tb.Fatal(err)
		}
		closer := func() {
			ab.Close()
			closeFunc()
		}
		return ab, closer
	}
}

func keyBookFactory(tb testing.TB, storeFactory datastoreFactory) pt.KeyBookFactory {
	return func() (tstore.KeyBook, func()) {
		store, closeFunc := storeFactory(tb)
		kb, err := NewKeyBook(store)
		if err != nil {
			tb.Fatal(err)
		}
		closer := func() {
			closeFunc()
		}
		return kb, closer
	}
}

func badgerStore(tb testing.TB) (ds.Batching, func()) {
	dataPath, err := ioutil.TempDir(os.TempDir(), "badger")
	if err != nil {
		tb.Fatal(err)
	}
	store, err := badger.NewDatastore(dataPath, nil)
	if err != nil {
		tb.Fatal(err)
	}
	closer := func() {
		store.Close()
		os.RemoveAll(dataPath)
	}
	return store, closer
}

func leveldbStore(tb testing.TB) (ds.Batching, func()) {
	dataPath, err := ioutil.TempDir(os.TempDir(), "leveldb")
	if err != nil {
		tb.Fatal(err)
	}
	store, err := leveldb.NewDatastore(dataPath, nil)
	if err != nil {
		tb.Fatal(err)
	}
	closer := func() {
		store.Close()
		os.RemoveAll(dataPath)
	}
	return store, closer
}
