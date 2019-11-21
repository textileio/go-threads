package eventstore

import (
	"io/ioutil"
	"os"
	"testing"
)

func TestManager_NewStore(t *testing.T) {
	t.Parallel()
	t.Run("Single", func(t *testing.T) {
		t.Parallel()
		man, clean := createTestManager(t)
		defer clean()
		id, _, err := man.NewStore()
		checkErr(t, err)
		log.Debugf("added store %s", id.String())
	})
	t.Run("Multiple", func(t *testing.T) {
		t.Parallel()
		man, clean := createTestManager(t)
		defer clean()
		_, _, err := man.NewStore()
		checkErr(t, err)
		_, _, err = man.NewStore()
		checkErr(t, err)
	})
}

func createTestManager(t *testing.T) (*Manager, func()) {
	dir, err := ioutil.TempDir("", "")
	checkErr(t, err)
	m, err := NewManager(WithRepoPath(dir))
	checkErr(t, err)
	return m, func() { _ = os.RemoveAll(dir) }
}
