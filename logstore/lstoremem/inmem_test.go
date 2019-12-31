package lstoremem_test

import (
	"testing"

	core "github.com/textileio/go-threads/core/logstore"
	m "github.com/textileio/go-threads/logstore/lstoremem"
	pt "github.com/textileio/go-threads/test"
)

func TestInMemoryLogstore(t *testing.T) {
	pt.LogstoreTest(t, func() (core.Logstore, func()) {
		return m.NewLogstore(), nil
	})
}

func TestInMemoryAddrBook(t *testing.T) {
	pt.AddrBookTest(t, func() (core.AddrBook, func()) {
		return m.NewAddrBook(), nil
	})
}

func TestInMemoryKeyBook(t *testing.T) {
	pt.KeyBookTest(t, func() (core.KeyBook, func()) {
		return m.NewKeyBook(), nil
	})
}

func TestInMemoryHeadBook(t *testing.T) {
	pt.HeadBookTest(t, func() (core.HeadBook, func()) {
		return m.NewHeadBook(), nil
	})
}

func TestInMemoryMetadataBook(t *testing.T) {
	pt.MetadataBookTest(t, func() (core.ThreadMetadata, func()) {
		return m.NewThreadMetadata(), nil
	})
}

func BenchmarkInMemoryLogstore(b *testing.B) {
	pt.BenchmarkLogstore(b, func() (core.Logstore, func()) {
		return m.NewLogstore(), nil
	}, "InMem")
}

func BenchmarkInMemoryKeyBook(b *testing.B) {
	pt.BenchmarkKeyBook(b, func() (core.KeyBook, func()) {
		return m.NewKeyBook(), nil
	})
}

func BenchmarkInMemoryHeadBook(b *testing.B) {
	pt.BenchmarkHeadBook(b, func() (core.HeadBook, func()) {
		return m.NewHeadBook(), nil
	})
}
