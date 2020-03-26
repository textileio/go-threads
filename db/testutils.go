package db

import (
	"context"
	"crypto/rand"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p-core/crypto"
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
	n, err := DefaultNetwork(dir)
	checkErr(t, err)
	opts = append(opts, WithRepoPath(dir))
	id := thread.NewIDV1(thread.Raw, 32)
	author, _, err := crypto.GenerateEd25519Key(rand.Reader)
	checkErr(t, err)
	creds := credentials{threadID: id, privKey: author}
	d, err := NewDB(context.Background(), n, creds, opts...)
	checkErr(t, err)
	return d, func() {
		time.Sleep(time.Second) // Give threads a chance to finish work
		if err := n.Close(); err != nil {
			panic(err)
		}
		_ = os.RemoveAll(dir)
	}
}
