package test

import (
	"bytes"
	"testing"

	"github.com/textileio/go-textile-core/thread"
	tstore "github.com/textileio/go-textile-core/threadstore"
)

const (
	errStrNotFoundKey      = "should return nil without errors"
	errStrPut              = "put failed for key %s: %v"
	errStrGet              = "get failed for key %s: %v"
	errStrValueMatch       = "expected %v, got %v"
	errStrValueShouldExist = "value should exist for key"
)

var metadataBookSuite = map[string]func(mb tstore.ThreadMetadata) func(*testing.T){
	"Int64":    testMetadataBookInt64,
	"String":   testMetadataBookString,
	"Byte":     testMetadataBookBytes,
	"NotFound": testMetadataBookNotFound,
}

type MetadataBookFactory func() (tstore.ThreadMetadata, func())

func MetadataBookTest(t *testing.T, factory MetadataBookFactory) {
	for name, test := range metadataBookSuite {
		mb, closeFunc := factory()
		t.Run(name, test(mb))
		if closeFunc != nil {
			closeFunc()
		}
	}
}

func testMetadataBookInt64(mb tstore.ThreadMetadata) func(*testing.T) {
	return func(t *testing.T) {
		t.Run("Put&Get", func(t *testing.T) {
			t.Parallel()
			tid := thread.NewIDV1(thread.Raw, 24)

			key, value := "key1", int64(42)
			if err := mb.PutInt64(tid, key, value); err != nil {
				t.Fatalf(errStrPut, key, err)
			}
			v, err := mb.GetInt64(tid, key)
			if err != nil {
				t.Fatalf(errStrGet, key, err)
			}
			if v == nil {
				t.Fatalf(errStrValueShouldExist)
			}
			if *v != value {

				t.Fatalf(errStrValueMatch, value, v)
			}
		})
	}
}

func testMetadataBookString(mb tstore.ThreadMetadata) func(*testing.T) {
	return func(t *testing.T) {
		t.Run("Put&Get", func(t *testing.T) {
			t.Parallel()
			tid := thread.NewIDV1(thread.Raw, 24)

			key, value := "key1", "textile"
			if err := mb.PutString(tid, key, value); err != nil {
				t.Fatalf(errStrPut, key, err)
			}
			v, err := mb.GetString(tid, key)
			if err != nil {
				t.Fatalf(errStrGet, key, err)
			}
			if v == nil {
				t.Fatalf(errStrValueShouldExist)
			}
			if *v != value {

				t.Fatalf(errStrValueMatch, value, v)
			}
		})
	}
}

func testMetadataBookBytes(mb tstore.ThreadMetadata) func(*testing.T) {
	return func(t *testing.T) {
		t.Run("Put&Get", func(t *testing.T) {
			t.Parallel()
			tid := thread.NewIDV1(thread.Raw, 24)

			key, value := "key1", []byte("textile")
			if err := mb.PutBytes(tid, key, value); err != nil {
				t.Fatalf(errStrPut, key, err)
			}
			v, err := mb.GetBytes(tid, key)
			if err != nil {
				t.Fatalf(errStrGet, key, err)
			}
			if v == nil {
				t.Fatalf(errStrValueShouldExist)
			}
			if !bytes.Equal(*v, value) {
				t.Fatalf(errStrValueMatch, value, v)
			}
		})
		t.Run("Immutable Put", func(t *testing.T) {
			t.Parallel()
			tid := thread.NewIDV1(thread.Raw, 24)

			key, value := "key1", []byte("textile")
			if err := mb.PutBytes(tid, key, value); err != nil {
				t.Fatalf(errStrPut, key, err)
			}
			insertedValue := append(value[:0:0], value...)
			value[0] = byte('T')
			v, err := mb.GetBytes(tid, key)
			if err != nil {
				t.Fatalf(errStrGet, key, err)
			}
			if v == nil {
				t.Fatalf(errStrValueShouldExist)
			}
			if !bytes.Equal(*v, insertedValue) {
				t.Fatalf(errStrValueMatch, value, v)
			}
		})
	}
}

func testMetadataBookNotFound(mb tstore.ThreadMetadata) func(*testing.T) {
	return func(t *testing.T) {
		t.Run("Int64", func(t *testing.T) {
			t.Parallel()
			tid := thread.NewIDV1(thread.Raw, 24)

			if v, err := mb.GetInt64(tid, "textile"); v != nil || err != nil {
				t.Fatalf(errStrNotFoundKey)
			}
		})
		t.Run("String", func(t *testing.T) {
			t.Parallel()
			tid := thread.NewIDV1(thread.Raw, 24)

			if v, err := mb.GetInt64(tid, "textile"); v != nil || err != nil {
				t.Fatalf(errStrNotFoundKey)
			}
		})
		t.Run("Bytes", func(t *testing.T) {
			t.Parallel()
			tid := thread.NewIDV1(thread.Raw, 24)

			if v, err := mb.GetInt64(tid, "textile"); v != nil || err != nil {
				t.Fatalf(errStrNotFoundKey)
			}
		})
	}
}
