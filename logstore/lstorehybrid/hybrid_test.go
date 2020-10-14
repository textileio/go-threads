package lstorehybrid

import (
	"context"
	"io/ioutil"
	"os"
	"testing"

	badger "github.com/ipfs/go-ds-badger"
	core "github.com/textileio/go-threads/core/logstore"
	"github.com/textileio/go-threads/logstore/lstoreds"
	m "github.com/textileio/go-threads/logstore/lstoremem"
	pt "github.com/textileio/go-threads/test"
)

type storeFactory func(tb testing.TB) (core.Logstore, func())

var (
	persist = map[string]storeFactory{
		"lstoreds:Badger": lstoredsBadgerF,
	}

	inMem = map[string]storeFactory{
		"lstoremem": lstorememF,
	}
)

func TestHybridLogstore(t *testing.T) {
	for psName, psF := range persist {
		for msName, msF := range inMem {
			t.Run(psName+"+"+msName, func(t *testing.T) {
				t.Parallel()
				pt.LogstoreTest(t, logstoreFactory(t, psF, msF))
			})
		}
	}
}

func TestHybridAddrBook(t *testing.T) {
	for psName, psF := range persist {
		for msName, msF := range inMem {
			t.Run(psName+"+"+msName, func(t *testing.T) {
				t.Parallel()
				pt.AddrBookTest(t, adapterAddrBook(logstoreFactory(t, psF, msF)))
			})
		}
	}
}

func TestHybridKeyBook(t *testing.T) {
	for psName, psF := range persist {
		for msName, msF := range inMem {
			t.Run(psName+"+"+msName, func(t *testing.T) {
				t.Parallel()
				pt.KeyBookTest(t, adapterKeyBook(logstoreFactory(t, psF, msF)))
			})
		}
	}
}

func TestHybridHeadBook(t *testing.T) {
	for psName, psF := range persist {
		for msName, msF := range inMem {
			t.Run(psName+"+"+msName, func(t *testing.T) {
				t.Parallel()
				pt.HeadBookTest(t, adapterHeadBook(logstoreFactory(t, psF, msF)))
			})
		}
	}
}

func TestHybridMetadataBook(t *testing.T) {
	for psName, psF := range persist {
		for msName, msF := range inMem {
			t.Run(psName+"+"+msName, func(t *testing.T) {
				t.Parallel()
				pt.MetadataBookTest(t, adapterMetaBook(logstoreFactory(t, psF, msF)))
			})
		}
	}
}

/* store factories */

func logstoreFactory(tb testing.TB, persistF, memF storeFactory) pt.LogstoreFactory {
	return func() (core.Logstore, func()) {
		ps, psClose := persistF(tb)
		ms, msClose := memF(tb)

		ls, err := NewLogstore(ps, ms)
		if err != nil {
			tb.Fatal(err)
		}

		closer := func() {
			_ = ls.Close()
			psClose()
			msClose()
		}

		return ls, closer
	}
}

func lstoredsBadgerF(tb testing.TB) (core.Logstore, func()) {
	dataPath, err := ioutil.TempDir(os.TempDir(), "badger")
	if err != nil {
		tb.Fatal(err)
	}

	backend, err := badger.NewDatastore(dataPath, nil)
	if err != nil {
		tb.Fatal(err)
	}

	lstore, err := lstoreds.NewLogstore(
		context.Background(),
		backend,
		lstoreds.DefaultOpts(),
	)

	closer := func() {
		_ = lstore.Close()
		_ = backend.Close()
		_ = os.RemoveAll(dataPath)
	}

	return lstore, closer
}

func lstorememF(_ testing.TB) (core.Logstore, func()) {
	store := m.NewLogstore()
	return store, func() { _ = store.Close() }
}

/* component adapters */

func adapterAddrBook(f pt.LogstoreFactory) pt.AddrBookFactory {
	return func() (core.AddrBook, func()) { return f() }
}

func adapterKeyBook(f pt.LogstoreFactory) pt.KeyBookFactory {
	return func() (core.KeyBook, func()) { return f() }
}

func adapterHeadBook(f pt.LogstoreFactory) pt.HeadBookFactory {
	return func() (core.HeadBook, func()) { return f() }
}

func adapterMetaBook(f pt.LogstoreFactory) pt.MetadataBookFactory {
	return func() (core.ThreadMetadata, func()) { return f() }
}
