package db

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/textileio/go-threads/core/thread"
)

func checkErr(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func createTestDB(t *testing.T, opts ...Option) (*DB, func()) {
	dir, err := ioutil.TempDir("", "")
	checkErr(t, err)
	ts, err := DefaultService(dir)
	checkErr(t, err)
	opts = append(opts, WithRepoPath(dir))
	d, err := NewDB(ts, thread.NewIDV1(thread.Raw, 32), opts...)
	checkErr(t, err)
	return d, func() {
		if err := ts.Close(); err != nil {
			panic(err)
		}
		_ = os.RemoveAll(dir)
	}
}
