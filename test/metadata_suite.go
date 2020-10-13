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
	"Int64":          testMetadataBookInt64,
	"String":         testMetadataBookString,
	"Byte":           testMetadataBookBytes,
	"NotFound":       testMetadataBookNotFound,
	"ClearMetadata":  testClearMetadata,
	"ExportMetadata": testMetadataBookExport,
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

func testMetadataBookExport(mb core.ThreadMetadata) func(*testing.T) {
	return func(t *testing.T) {
		var (
			tid = thread.NewIDV1(thread.Raw, 24)

			k1, v1 = "k1", int64(123)
			k2, v2 = "k2", int64(-612)
			k3, v3 = "k3", true
			k4, v4 = "k4", false
			k5, v5 = "k5", "v5"
			k6, v6 = "k6", "value6"
			k7, v7 = "k7", []byte("bytestring value 7")
			k8, v8 = "k8", []byte("v8")

			check = func(err error, key string, tmpl string) {
				if err != nil {
					t.Fatalf(tmpl, key, err)
				}
			}

			compare = func(expected, actual interface{}) {
				e, ok1 := expected.([]byte)
				a, ok2 := actual.([]byte)
				if (ok1 && ok2 && !bytes.Equal(e, a)) || (!ok1 && !ok2 && expected != actual) {
					t.Fatalf(errStrValueMatch, e, a)
				}
			}
		)

		check(mb.PutInt64(tid, k1, v1), k1, errStrPut)
		check(mb.PutInt64(tid, k2, v2), k2, errStrPut)
		check(mb.PutBool(tid, k3, v3), k3, errStrPut)
		check(mb.PutBool(tid, k4, v4), k4, errStrPut)
		check(mb.PutString(tid, k5, v5), k5, errStrPut)
		check(mb.PutString(tid, k6, v6), k6, errStrPut)
		check(mb.PutBytes(tid, k7, v7), k7, errStrPut)
		check(mb.PutBytes(tid, k8, v8), k8, errStrPut)

		dump, err := mb.DumpMeta()
		if err != nil {
			t.Fatalf("dumping metadata: %v", err)
		}

		if err := mb.ClearMetadata(tid); err != nil {
			t.Fatalf("clearing metadata: %v", err)
		}

		if err := mb.RestoreMeta(dump); err != nil {
			t.Fatalf("restoring metadata from the dump: %v", err)
		}

		{
			v, err := mb.GetInt64(tid, k1)
			check(err, k1, errStrGet)
			compare(v1, *v)
		}
		{
			v, err := mb.GetInt64(tid, k2)
			check(err, k2, errStrGet)
			compare(v2, *v)
		}
		{
			v, err := mb.GetBool(tid, k3)
			check(err, k3, errStrGet)
			compare(v3, *v)
		}
		{
			v, err := mb.GetBool(tid, k4)
			check(err, k4, errStrGet)
			compare(v4, *v)
		}
		{
			v, err := mb.GetString(tid, k5)
			check(err, k5, errStrGet)
			compare(v5, *v)
		}
		{
			v, err := mb.GetString(tid, k6)
			check(err, k6, errStrGet)
			compare(v6, *v)
		}
		{
			v, err := mb.GetBytes(tid, k7)
			check(err, k7, errStrGet)
			compare(v7, *v)
		}
		{
			v, err := mb.GetBytes(tid, k8)
			check(err, k8, errStrGet)
			compare(v8, *v)
		}
	}
}
