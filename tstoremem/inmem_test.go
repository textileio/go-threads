package tstoremem

import (
	"testing"

	tstore "github.com/textileio/go-textile-core/threadstore"
	tt "github.com/textileio/go-textile-threads/test"
)

func TestInMemoryThreadstore(t *testing.T) {
	tt.ThreadstoreTest(t, func() (tstore.Threadstore, func()) {
		return NewThreadstore(), nil
	})
}

func TestInMemoryLogAddrBook(t *testing.T) {
	tt.LogAddrBookTest(t, func() (tstore.LogAddrBook, func()) {
		return NewLogAddrBook(), nil
	})
}

func TestInMemoryLogKeyBook(t *testing.T) {
	tt.LogKeyBookTest(t, func() (tstore.LogKeyBook, func()) {
		return NewLogKeyBook(), nil
	})
}

func BenchmarkInMemoryThreadstore(b *testing.B) {
	tt.BenchmarkThreadstore(b, func() (tstore.Threadstore, func()) {
		return NewThreadstore(), nil
	}, "InMem")
}

func BenchmarkInMemoryLogKeyBook(b *testing.B) {
	tt.BenchmarkLogKeyBook(b, func() (tstore.LogKeyBook, func()) {
		return NewLogKeyBook(), nil
	})
}
