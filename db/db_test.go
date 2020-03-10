package db

import (
	"context"
	"io/ioutil"
	"os"
	"reflect"
	"testing"
	"time"

	ds "github.com/ipfs/go-datastore"
	format "github.com/ipfs/go-ipld-format"
	"github.com/multiformats/go-multiaddr"
	core "github.com/textileio/go-threads/core/db"
)

func TestE2EWithThreads(t *testing.T) {
	t.Parallel()

	// peer1: Create db1, register a collection, create and update an instance.
	tmpDir1, err := ioutil.TempDir("", "")
	checkErr(t, err)
	defer os.RemoveAll(tmpDir1)

	ts1, err := DefaultService(tmpDir1)
	checkErr(t, err)
	defer ts1.Close()

	d1, err := NewDB(ts1, WithRepoPath(tmpDir1))
	checkErr(t, err)
	defer d1.Close()
	c1, err := d1.NewCollectionFromInstance("dummy", &dummy{})
	checkErr(t, err)
	checkErr(t, d1.Start())
	dummyInstance := &dummy{Name: "Textile", Counter: 0}
	checkErr(t, c1.Create(dummyInstance))
	dummyInstance.Counter += 42
	checkErr(t, c1.Save(dummyInstance))

	// Boilerplate to generate peer1 thread-addr and get follow/read keys
	threadID, _, err := d1.ThreadID()
	checkErr(t, err)
	threadInfo, err := d1.Service().GetThread(context.Background(), threadID)
	checkErr(t, err)
	peer1Addr := d1.Service().Host().Addrs()[0]
	peer1ID, err := multiaddr.NewComponent("p2p", d1.Service().Host().ID().String())
	checkErr(t, err)
	threadComp, err := multiaddr.NewComponent("thread", threadID.String())
	checkErr(t, err)
	threadAddr := peer1Addr.Encapsulate(peer1ID).Encapsulate(threadComp)

	// Create a completely parallel db, which will sync with the previous one
	// and should have the same state of dummyInstance.
	tmpDir2, err := ioutil.TempDir("", "")
	checkErr(t, err)
	defer os.RemoveAll(tmpDir2)
	ts2, err := DefaultService(tmpDir2)
	checkErr(t, err)
	defer ts2.Close()

	d2, err := NewDB(ts2, WithRepoPath(tmpDir2))
	checkErr(t, err)
	defer d2.Close()
	c2, err := d2.NewCollectionFromInstance("dummy", &dummy{})
	checkErr(t, err)
	checkErr(t, d2.StartFromAddr(threadAddr, threadInfo.FollowKey, threadInfo.ReadKey))

	time.Sleep(time.Second * 3) // Wait a bit for sync

	dummyInstance2 := &dummy{}
	checkErr(t, c2.FindByID(dummyInstance.ID, dummyInstance2))
	if dummyInstance2.Name != dummyInstance.Name || dummyInstance2.Counter != dummyInstance.Counter {
		t.Fatalf("instances of both peers must be equal after sync")
	}
}

func TestOptions(t *testing.T) {
	t.Parallel()
	tmpDir, err := ioutil.TempDir("", "")
	checkErr(t, err)
	defer os.RemoveAll(tmpDir)

	ts, err := DefaultService(tmpDir)
	checkErr(t, err)

	ec := &mockEventCodec{}
	d, err := NewDB(ts, WithRepoPath(tmpDir), WithEventCodec(ec))
	checkErr(t, err)

	m, err := d.NewCollectionFromInstance("dummy", &dummy{})
	checkErr(t, err)
	checkErr(t, m.Create(&dummy{Name: "Textile"}))

	if !ec.called {
		t.Fatalf("custom event codec wasn't called")
	}

	// Re-do again to re-use key. If something wasn't closed correctly, would fail
	checkErr(t, ts.Close())
	checkErr(t, d.Close())

	time.Sleep(time.Second * 3)
	ts, err = DefaultService(tmpDir)
	checkErr(t, err)
	defer ts.Close()
	d, err = NewDB(ts, WithRepoPath(tmpDir), WithEventCodec(ec))
	checkErr(t, err)
	checkErr(t, d.Close())
}

func TestListeners(t *testing.T) {
	t.Parallel()

	assertActions := func(actions, expected []Action) {
		if len(actions) != len(expected) {
			t.Fatalf("number of actions isn't correct, expected %d, got %d", len(expected), len(actions))
		}
		for i := range actions {
			if !reflect.DeepEqual(actions[i], expected[i]) {
				t.Fatalf("wrong action detect, expected %v, got %v", expected[i], actions[i])
			}
		}
	}

	t.Run("AllDBEvents", func(t *testing.T) {
		t.Parallel()
		actions := runListenersComplexUseCase(t)
		expected := []Action{
			{Collection: "Collection1", Type: ActionSave, ID: "id-i1"},
			{Collection: "Collection1", Type: ActionCreate, ID: "id-i2"},
			{Collection: "Collection2", Type: ActionCreate, ID: "id-j1"},
			{Collection: "Collection1", Type: ActionSave, ID: "id-i1"},
			{Collection: "Collection1", Type: ActionSave, ID: "id-i2"},
			{Collection: "Collection2", Type: ActionSave, ID: "id-j1"},
			{Collection: "Collection1", Type: ActionDelete, ID: "id-i1"},
			{Collection: "Collection2", Type: ActionDelete, ID: "id-j1"},
			{Collection: "Collection1", Type: ActionDelete, ID: "id-i2"},
		}
		assertActions(actions, expected)
	})
	t.Run("AnyCollection1Events", func(t *testing.T) {
		t.Parallel()
		actions := runListenersComplexUseCase(t, ListenOption{Collection: "Collection1"})
		expected := []Action{
			{Collection: "Collection1", Type: ActionSave, ID: "id-i1"},
			{Collection: "Collection1", Type: ActionCreate, ID: "id-i2"},
			{Collection: "Collection1", Type: ActionSave, ID: "id-i1"},
			{Collection: "Collection1", Type: ActionSave, ID: "id-i2"},
			{Collection: "Collection1", Type: ActionDelete, ID: "id-i1"},
			{Collection: "Collection1", Type: ActionDelete, ID: "id-i2"},
		}
		assertActions(actions, expected)
	})
	t.Run("AnyCollection2Events", func(t *testing.T) {
		t.Parallel()
		actions := runListenersComplexUseCase(t, ListenOption{Collection: "Collection2"})
		expected := []Action{
			{Collection: "Collection2", Type: ActionCreate, ID: "id-j1"},
			{Collection: "Collection2", Type: ActionSave, ID: "id-j1"},
			{Collection: "Collection2", Type: ActionDelete, ID: "id-j1"},
		}
		assertActions(actions, expected)
	})
	t.Run("AnyCreateEvent", func(t *testing.T) {
		t.Parallel()
		actions := runListenersComplexUseCase(t, ListenOption{Type: ListenCreate})
		expected := []Action{
			{Collection: "Collection1", Type: ActionCreate, ID: "id-i2"},
			{Collection: "Collection2", Type: ActionCreate, ID: "id-j1"},
		}
		assertActions(actions, expected)
	})
	t.Run("AnySaveEvent", func(t *testing.T) {
		t.Parallel()
		actions := runListenersComplexUseCase(t, ListenOption{Type: ListenSave})
		expected := []Action{
			{Collection: "Collection1", Type: ActionSave, ID: "id-i1"},
			{Collection: "Collection1", Type: ActionSave, ID: "id-i1"},
			{Collection: "Collection1", Type: ActionSave, ID: "id-i2"},
			{Collection: "Collection2", Type: ActionSave, ID: "id-j1"},
		}
		assertActions(actions, expected)
	})
	t.Run("AnyDeleteEvent", func(t *testing.T) {
		t.Parallel()
		actions := runListenersComplexUseCase(t, ListenOption{Type: ListenDelete})
		expected := []Action{
			{Collection: "Collection1", Type: ActionDelete, ID: "id-i1"},
			{Collection: "Collection2", Type: ActionDelete, ID: "id-j1"},
			{Collection: "Collection1", Type: ActionDelete, ID: "id-i2"},
		}
		assertActions(actions, expected)
	})
	t.Run("AnyCollection1OrDeleteCollection2Events", func(t *testing.T) {
		t.Parallel()
		actions := runListenersComplexUseCase(t, ListenOption{Collection: "Collection1"}, ListenOption{Collection: "Collection2", Type: ListenDelete})
		expected := []Action{
			{Collection: "Collection1", Type: ActionSave, ID: "id-i1"},
			{Collection: "Collection1", Type: ActionCreate, ID: "id-i2"},
			{Collection: "Collection1", Type: ActionSave, ID: "id-i1"},
			{Collection: "Collection1", Type: ActionSave, ID: "id-i2"},
			{Collection: "Collection1", Type: ActionDelete, ID: "id-i1"},
			{Collection: "Collection2", Type: ActionDelete, ID: "id-j1"},
			{Collection: "Collection1", Type: ActionDelete, ID: "id-i2"},
		}
		assertActions(actions, expected)
	})
	t.Run("EmptyFilterEvent", func(t *testing.T) {
		t.Parallel()
		actions := runListenersComplexUseCase(t, ListenOption{Collection: "Collection3"})
		var expected []Action
		assertActions(actions, expected)
	})
	t.Run("MixedComplexEvent", func(t *testing.T) {
		t.Parallel()
		actions := runListenersComplexUseCase(t,
			ListenOption{Collection: "Collection2", Type: ListenSave},
			ListenOption{Collection: "Collection1", Type: ListenSave, ID: "id-i2"},
			ListenOption{Collection: "Collection1", Type: ListenDelete, ID: "id-i1"},
			ListenOption{Collection: "Collection2", Type: ListenDelete},
		)
		expected := []Action{
			{Collection: "Collection1", Type: ActionSave, ID: "id-i2"},
			{Collection: "Collection2", Type: ActionSave, ID: "id-j1"},
			{Collection: "Collection1", Type: ActionDelete, ID: "id-i1"},
			{Collection: "Collection2", Type: ActionDelete, ID: "id-j1"},
		}
		assertActions(actions, expected)
	})
}

// runListenersComplexUseCase runs a complex db use-case, and returns
// Actions received with the ...ListenOption provided.
func runListenersComplexUseCase(t *testing.T, los ...ListenOption) []Action {
	t.Helper()
	s, cls := createTestDB(t)
	c1, err := s.NewCollectionFromInstance("Collection1", &dummy{})
	checkErr(t, err)
	c2, err := s.NewCollectionFromInstance("Collection2", &dummy{})
	checkErr(t, err)

	// Create some instance *before* any listener, just to test doesn't appear
	// on listener Action stream.
	i1 := &dummy{ID: "id-i1", Name: "Textile1"}
	checkErr(t, c1.Create(i1))

	l, err := s.Listen(los...)
	checkErr(t, err)
	var actions []Action
	go func() {
		for a := range l.Channel() {
			actions = append(actions, a)
		}
	}()

	// Collection1 Save i1
	i1.Name = "Textile0"
	checkErr(t, c1.Save(i1))

	// Collection1 Create i2
	i2 := &dummy{ID: "id-i2", Name: "Textile2"}
	checkErr(t, c1.Create(i2))

	// Collection2 Create j1
	j1 := &dummy{ID: "id-j1", Name: "Textile3"}
	checkErr(t, c2.Create(j1))

	// Collection1 Save i1
	// Collection1 Save i2
	err = c1.WriteTxn(func(txn *Txn) error {
		i1.Counter = 30
		i2.Counter = 11
		if err := txn.Save(i1, i2); err != nil {
			return nil
		}
		return nil
	})
	checkErr(t, err)

	// Collection2 Save j1
	j1.Counter = -1
	j1.Name = "Textile33"
	checkErr(t, c2.Save(j1))

	checkErr(t, c1.Delete(i1.ID))

	// Collection2 Delete
	checkErr(t, c2.Delete(j1.ID))

	// Collection2 Delete i2
	checkErr(t, c1.Delete(i2.ID))

	l.Close()
	cls()
	// Expected generated actions:
	// Collection1 Save i1
	// Collection1 Create i2
	// Collection2 Create j1
	// Save i1
	// Save i2
	// Collection2 Save j1
	// Delete i1
	// Collection2 Delete j1
	// Delete i2

	return actions
}

type dummy struct {
	ID      core.InstanceID
	Name    string
	Counter int
}

type mockEventCodec struct {
	called bool
}

var _ core.EventCodec = (*mockEventCodec)(nil)

func (dec *mockEventCodec) Reduce([]core.Event, ds.TxnDatastore, ds.Key, func(collection string, key ds.Key, oldData, newData []byte, txn ds.Txn) error) ([]core.ReduceAction, error) {
	dec.called = true
	return nil, nil
}
func (dec *mockEventCodec) Create([]core.Action) ([]core.Event, format.Node, error) {
	dec.called = true
	return nil, nil, nil
}
func (dec *mockEventCodec) EventsFromBytes([]byte) ([]core.Event, error) {
	dec.called = true
	return nil, nil
}
