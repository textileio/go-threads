package db

import (
	"context"
	"io/ioutil"
	"os"
	"testing"
	"time"

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
	id := thread.NewIDV1(thread.Raw, 32)
	d, err := NewDB(context.Background(), ts, id, opts...)
	checkErr(t, err)
	return d, func() {
		time.Sleep(time.Second) // Give threads a chance to finish work
		if err := ts.Close(); err != nil {
			panic(err)
		}
		_ = os.RemoveAll(dir)
	}
}
