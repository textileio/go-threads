package test

import (
	"bytes"
	"testing"

	core "github.com/textileio/go-threads/core/logstore"
	"github.com/textileio/go-threads/core/thread"
)

const (
	errStrNotFoundKey         = "should return nil without errors"
	errStrPut                 = "put failed for key %s: %v"
	errStrGet                 = "get failed for key %s: %v"
	errStrValueMatch          = "expected %v, got %v"
	errStrValueShouldExist    = "value should exist for key"
	errStrValueShouldNotExist = "value should not exist for key"
)

var metadataBookSuite = map[string]func(mb core.ThreadMetadata) func(*testing.T){
	"Int64":         testMetadataBookInt64,
	"String":        testMetadataBookString,
	"Byte":          testMetadataBookBytes,
	"NotFound":      testMetadataBookNotFound,
	"ClearMetadata": testClearMetadata,
}

type MetadataBookFactory func() (core.ThreadMetadata, func())

func MetadataBookTest(t *testing.T, factory MetadataBookFactory) {
	for name, test := range metadataBookSuite {
		mb, closeFunc := factory()
		t.Run(name, test(mb))
		if closeFunc != nil {
			closeFunc()
		}
	}
}

func testMetadataBookInt64(mb core.ThreadMetadata) func(*testing.T) {
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

func testMetadataBookString(mb core.ThreadMetadata) func(*testing.T) {
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

func testMetadataBookBytes(mb core.ThreadMetadata) func(*testing.T) {
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

func testMetadataBookNotFound(mb core.ThreadMetadata) func(*testing.T) {
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

func testClearMetadata(mb core.ThreadMetadata) func(*testing.T) {
	return func(t *testing.T) {
		tid := thread.NewIDV1(thread.Raw, 24)

		key, value := "key", []byte("textile")
		if err := mb.PutBytes(tid, key, value); err != nil {
			t.Fatalf(errStrPut, key, err)
		}
		if err := mb.ClearMetadata(tid); err != nil {
			t.Fatalf("clear metadata failed: %v", err)
		}
		v, err := mb.GetBytes(tid, key)
		if err != nil {
			t.Fatalf(errStrGet, key, err)
		}
		if v != nil {
			t.Fatalf(errStrValueShouldNotExist)
		}
	}
}
