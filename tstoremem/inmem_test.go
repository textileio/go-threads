package tstoremem_test

import (
	"testing"

	tstore "github.com/textileio/go-textile-core/threadstore"
	tt "github.com/textileio/go-textile-threads/test"
	m "github.com/textileio/go-textile-threads/tstoremem"
)

func TestInMemoryThreadstore(t *testing.T) {
	tt.ThreadstoreTest(t, func() (tstore.Threadstore, func()) {
		return m.NewThreadstore(), nil
	})
}

func TestInMemoryLogAddrBook(t *testing.T) {
	tt.LogAddrBookTest(t, func() (tstore.LogAddrBook, func()) {
		return m.NewLogAddrBook(), nil
	})
}

func TestInMemoryLogKeyBook(t *testing.T) {
	tt.LogKeyBookTest(t, func() (tstore.LogKeyBook, func()) {
		return m.NewLogKeyBook(), nil
	})
}

func TestInMemoryLogHeadBook(t *testing.T) {
	tt.LogHeadBookTest(t, func() (tstore.LogHeadBook, func()) {
		return m.NewLogHeadBook(), nil
	})
}

func BenchmarkInMemoryThreadstore(b *testing.B) {
	tt.BenchmarkThreadstore(b, func() (tstore.Threadstore, func()) {
		return m.NewThreadstore(), nil
	}, "InMem")
}

func BenchmarkInMemoryLogKeyBook(b *testing.B) {
	tt.BenchmarkLogKeyBook(b, func() (tstore.LogKeyBook, func()) {
		return m.NewLogKeyBook(), nil
	})
}

func BenchmarkInMemoryLogHeadBook(b *testing.B) {
	tt.BenchmarkLogHeadBook(b, func() (tstore.LogHeadBook, func()) {
		return m.NewLogHeadBook(), nil
	})
}
