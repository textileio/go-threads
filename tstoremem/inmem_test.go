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

func TestInMemoryAddrBook(t *testing.T) {
	tt.AddrBookTest(t, func() (tstore.AddrBook, func()) {
		return m.NewAddrBook(), nil
	})
}

func TestInMemoryKeyBook(t *testing.T) {
	tt.KeyBookTest(t, func() (tstore.KeyBook, func()) {
		return m.NewKeyBook(), nil
	})
}

func TestInMemoryHeadBook(t *testing.T) {
	tt.HeadBookTest(t, func() (tstore.HeadBook, func()) {
		return m.NewHeadBook(), nil
	})
}

func BenchmarkInMemoryThreadstore(b *testing.B) {
	tt.BenchmarkThreadstore(b, func() (tstore.Threadstore, func()) {
		return m.NewThreadstore(), nil
	}, "InMem")
}

func BenchmarkInMemoryKeyBook(b *testing.B) {
	tt.BenchmarkKeyBook(b, func() (tstore.KeyBook, func()) {
		return m.NewKeyBook(), nil
	})
}

func BenchmarkInMemoryHeadBook(b *testing.B) {
	tt.BenchmarkHeadBook(b, func() (tstore.HeadBook, func()) {
		return m.NewHeadBook(), nil
	})
}
