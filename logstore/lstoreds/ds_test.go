package lstoreds

import (
	"context"
	"io/ioutil"
	"os"
	"testing"
	"time"

	ds "github.com/ipfs/go-datastore"
	badger "github.com/ipfs/go-ds-badger"
	core "github.com/textileio/go-threads/core/logstore"
	pt "github.com/textileio/go-threads/test"
)

type datastoreFactory func(tb testing.TB) (ds.Batching, func())

var dstores = map[string]datastoreFactory{
	"Badger": badgerStore,
	// "Leveldb": leveldbStore,
}

func TestDatastoreLogstore(t *testing.T) {
	for name, dsFactory := range dstores {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			pt.LogstoreTest(t, logstoreFactory(t, dsFactory, DefaultOpts()))
		})
	}
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

func TestDatastoreHeadBook(t *testing.T) {
	for name, dsFactory := range dstores {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			pt.HeadBookTest(t, headBookFactory(t, dsFactory))
		})
	}
}

func TestDatastoreMetadataBook(t *testing.T) {
	for name, dsFactory := range dstores {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			pt.MetadataBookTest(t, metadataBookFactory(t, dsFactory))
		})
	}
}

func logstoreFactory(tb testing.TB, storeFactory datastoreFactory, opts Options) pt.LogstoreFactory {
	return func() (core.Logstore, func()) {
		store, closeFunc := storeFactory(tb)
		ls, err := NewLogstore(context.Background(), store, opts)
		if err != nil {
			tb.Fatal(err)
		}
		closer := func() {
			_ = ls.Close()
			closeFunc()
		}
		return ls, closer
	}
}

func addressBookFactory(tb testing.TB, storeFactory datastoreFactory, opts Options) pt.AddrBookFactory {
	return func() (core.AddrBook, func()) {
		store, closeFunc := storeFactory(tb)
		ab, err := NewAddrBook(context.Background(), store, opts)
		if err != nil {
			tb.Fatal(err)
		}
		closer := func() {
			_ = ab.Close()
			closeFunc()
		}
		return ab, closer
	}
}

func keyBookFactory(tb testing.TB, storeFactory datastoreFactory) pt.KeyBookFactory {
	return func() (core.KeyBook, func()) {
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

func headBookFactory(tb testing.TB, storeFactory datastoreFactory) pt.HeadBookFactory {
	return func() (core.HeadBook, func()) {
		store, closeFunc := storeFactory(tb)
		hb := NewHeadBook(store.(ds.TxnDatastore))
		closer := func() {
			closeFunc()
		}
		return hb, closer
	}
}

func metadataBookFactory(tb testing.TB, storeFactory datastoreFactory) pt.MetadataBookFactory {
	return func() (core.ThreadMetadata, func()) {
		store, closeFunc := storeFactory(tb)
		tm := NewThreadMetadata(store)
		closer := func() {
			closeFunc()
		}
		return tm, closer
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
		_ = store.Close()
		_ = os.RemoveAll(dataPath)
	}
	return store, closer
}

// func leveldbStore(tb testing.TB) (ds.Datastore, func()) {
// 	dataPath, err := ioutil.TempDir(os.TempDir(), "leveldb")
// 	if err != nil {
// 		tb.Fatal(err)
// 	}
// 	store, err := leveldb.NewDatastore(dataPath, nil)
// 	if err != nil {
// 		tb.Fatal(err)
// 	}
// 	closer := func() {
// 		_ = store.Close()
// 		_ = os.RemoveAll(dataPath)
// 	}
// 	return store, closer
// }
