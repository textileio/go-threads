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
		_, _, err := man.NewStore()
		checkErr(t, err)
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

func TestManager_GetStore(t *testing.T) {
	t.Parallel()

	dir, err := ioutil.TempDir("", "")
	checkErr(t, err)
	ts, err := DefaultThreadservice(dir, ProxyPort(0))
	checkErr(t, err)
	man, err := NewManager(ts, WithRepoPath(dir))
	checkErr(t, err)
	defer func() {
		_ = ts.Close()
		_ = os.RemoveAll(dir)
	}()

	id, _, err := man.NewStore()
	checkErr(t, err)
	store := man.GetStore(id)
	if store == nil {
		t.Fatal("store not found")
	}

	// Register a model so that a key is saved in the datastore
	// under the manager prefix
	model, err := store.Register("Person", &Person{})
	checkErr(t, err)
	newPerson := &Person{Name: "Foo", Age: 21}
	err = model.Create(newPerson)
	checkErr(t, err)
	assertPersonInModel(t, model, newPerson)

	// Close it down, restart next
	err = man.Close()
	checkErr(t, err)
	err = ts.Close()
	checkErr(t, err)

	t.Run("GetHydrated", func(t *testing.T) {
		ts, err := DefaultThreadservice(dir, ProxyPort(0))
		checkErr(t, err)
		defer ts.Close()
		man, err := NewManager(ts, WithRepoPath(dir))
		checkErr(t, err)

		store := man.GetStore(id)
		if store == nil {
			t.Fatal("store was not hydrated")
		}
	})
}

func createTestManager(t *testing.T) (*Manager, func()) {
	dir, err := ioutil.TempDir("", "")
	checkErr(t, err)
	ts, err := DefaultThreadservice(dir, ProxyPort(0))
	checkErr(t, err)
	m, err := NewManager(ts, WithRepoPath(dir))
	checkErr(t, err)
	return m, func() {
		if err := ts.Close(); err != nil {
			panic(err)
		}
		_ = os.RemoveAll(dir)
	}
}
