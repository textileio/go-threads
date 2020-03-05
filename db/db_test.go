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
	core "github.com/textileio/go-threads/core/store"
)

func TestE2EWithThreads(t *testing.T) {
	t.Parallel()

	// peer1: Create db1, register a model, create and update an instance.
	tmpDir1, err := ioutil.TempDir("", "")
	checkErr(t, err)
	defer os.RemoveAll(tmpDir1)

	ts1, err := DefaultService(tmpDir1)
	checkErr(t, err)
	defer ts1.Close()

	d1, err := NewDB(ts1, WithRepoPath(tmpDir1))
	checkErr(t, err)
	defer d1.Close()
	m1, err := d1.Register("dummy", &dummyModel{})
	checkErr(t, err)
	checkErr(t, d1.Start())
	dummyInstance := &dummyModel{Name: "Textile", Counter: 0}
	checkErr(t, m1.Create(dummyInstance))
	dummyInstance.Counter += 42
	checkErr(t, m1.Save(dummyInstance))

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
	m2, err := d2.Register("dummy", &dummyModel{})
	checkErr(t, err)
	checkErr(t, d2.StartFromAddr(threadAddr, threadInfo.FollowKey, threadInfo.ReadKey))

	time.Sleep(time.Second * 3) // Wait a bit for sync

	dummyInstance2 := &dummyModel{}
	checkErr(t, m2.FindByID(dummyInstance.ID, dummyInstance2))
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

	m, err := d.Register("dummy", &dummyModel{})
	checkErr(t, err)
	checkErr(t, m.Create(&dummyModel{Name: "Textile"}))

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
			{Model: "Model1", Type: ActionSave, ID: "id-i1"},
			{Model: "Model1", Type: ActionCreate, ID: "id-i2"},
			{Model: "Model2", Type: ActionCreate, ID: "id-j1"},
			{Model: "Model1", Type: ActionSave, ID: "id-i1"},
			{Model: "Model1", Type: ActionSave, ID: "id-i2"},
			{Model: "Model2", Type: ActionSave, ID: "id-j1"},
			{Model: "Model1", Type: ActionDelete, ID: "id-i1"},
			{Model: "Model2", Type: ActionDelete, ID: "id-j1"},
			{Model: "Model1", Type: ActionDelete, ID: "id-i2"},
		}
		assertActions(actions, expected)
	})
	t.Run("AnyModel1Events", func(t *testing.T) {
		t.Parallel()
		actions := runListenersComplexUseCase(t, ListenOption{Model: "Model1"})
		expected := []Action{
			{Model: "Model1", Type: ActionSave, ID: "id-i1"},
			{Model: "Model1", Type: ActionCreate, ID: "id-i2"},
			{Model: "Model1", Type: ActionSave, ID: "id-i1"},
			{Model: "Model1", Type: ActionSave, ID: "id-i2"},
			{Model: "Model1", Type: ActionDelete, ID: "id-i1"},
			{Model: "Model1", Type: ActionDelete, ID: "id-i2"},
		}
		assertActions(actions, expected)
	})
	t.Run("AnyModel2Events", func(t *testing.T) {
		t.Parallel()
		actions := runListenersComplexUseCase(t, ListenOption{Model: "Model2"})
		expected := []Action{
			{Model: "Model2", Type: ActionCreate, ID: "id-j1"},
			{Model: "Model2", Type: ActionSave, ID: "id-j1"},
			{Model: "Model2", Type: ActionDelete, ID: "id-j1"},
		}
		assertActions(actions, expected)
	})
	t.Run("AnyCreateEvent", func(t *testing.T) {
		t.Parallel()
		actions := runListenersComplexUseCase(t, ListenOption{Type: ListenCreate})
		expected := []Action{
			{Model: "Model1", Type: ActionCreate, ID: "id-i2"},
			{Model: "Model2", Type: ActionCreate, ID: "id-j1"},
		}
		assertActions(actions, expected)
	})
	t.Run("AnySaveEvent", func(t *testing.T) {
		t.Parallel()
		actions := runListenersComplexUseCase(t, ListenOption{Type: ListenSave})
		expected := []Action{
			{Model: "Model1", Type: ActionSave, ID: "id-i1"},
			{Model: "Model1", Type: ActionSave, ID: "id-i1"},
			{Model: "Model1", Type: ActionSave, ID: "id-i2"},
			{Model: "Model2", Type: ActionSave, ID: "id-j1"},
		}
		assertActions(actions, expected)
	})
	t.Run("AnyDeleteEvent", func(t *testing.T) {
		t.Parallel()
		actions := runListenersComplexUseCase(t, ListenOption{Type: ListenDelete})
		expected := []Action{
			{Model: "Model1", Type: ActionDelete, ID: "id-i1"},
			{Model: "Model2", Type: ActionDelete, ID: "id-j1"},
			{Model: "Model1", Type: ActionDelete, ID: "id-i2"},
		}
		assertActions(actions, expected)
	})
	t.Run("AnyModel1OrDeleteModel2Events", func(t *testing.T) {
		t.Parallel()
		actions := runListenersComplexUseCase(t, ListenOption{Model: "Model1"}, ListenOption{Model: "Model2", Type: ListenDelete})
		expected := []Action{
			{Model: "Model1", Type: ActionSave, ID: "id-i1"},
			{Model: "Model1", Type: ActionCreate, ID: "id-i2"},
			{Model: "Model1", Type: ActionSave, ID: "id-i1"},
			{Model: "Model1", Type: ActionSave, ID: "id-i2"},
			{Model: "Model1", Type: ActionDelete, ID: "id-i1"},
			{Model: "Model2", Type: ActionDelete, ID: "id-j1"},
			{Model: "Model1", Type: ActionDelete, ID: "id-i2"},
		}
		assertActions(actions, expected)
	})
	t.Run("EmptyFilterEvent", func(t *testing.T) {
		t.Parallel()
		actions := runListenersComplexUseCase(t, ListenOption{Model: "Model3"})
		var expected []Action
		assertActions(actions, expected)
	})
	t.Run("MixedComplexEvent", func(t *testing.T) {
		t.Parallel()
		actions := runListenersComplexUseCase(t,
			ListenOption{Model: "Model2", Type: ListenSave},
			ListenOption{Model: "Model1", Type: ListenSave, ID: "id-i2"},
			ListenOption{Model: "Model1", Type: ListenDelete, ID: "id-i1"},
			ListenOption{Model: "Model2", Type: ListenDelete},
		)
		expected := []Action{
			{Model: "Model1", Type: ActionSave, ID: "id-i2"},
			{Model: "Model2", Type: ActionSave, ID: "id-j1"},
			{Model: "Model1", Type: ActionDelete, ID: "id-i1"},
			{Model: "Model2", Type: ActionDelete, ID: "id-j1"},
		}
		assertActions(actions, expected)
	})
}

// runListenersComplexUseCase runs a complex db use-case, and returns
// Actions received with the ...ListenOption provided.
func runListenersComplexUseCase(t *testing.T, los ...ListenOption) []Action {
	t.Helper()
	s, cls := createTestDB(t)
	m1, err := s.Register("Model1", &dummyModel{})
	checkErr(t, err)
	m2, err := s.Register("Model2", &dummyModel{})
	checkErr(t, err)

	// Create some instance *before* any listener, just to test doesn't appear
	// on listener Action stream.
	i1 := &dummyModel{ID: "id-i1", Name: "Textile1"}
	checkErr(t, m1.Create(i1))

	l, err := s.Listen(los...)
	checkErr(t, err)
	var actions []Action
	go func() {
		for a := range l.Channel() {
			actions = append(actions, a)
		}
	}()

	// Model1 Save i1
	i1.Name = "Textile0"
	checkErr(t, m1.Save(i1))

	// Model1 Create i2
	i2 := &dummyModel{ID: "id-i2", Name: "Textile2"}
	checkErr(t, m1.Create(i2))

	// Model2 Create j1
	j1 := &dummyModel{ID: "id-j1", Name: "Textile3"}
	checkErr(t, m2.Create(j1))

	// Model1 Save i1
	// Model1 Save i2
	err = m1.WriteTxn(func(txn *Txn) error {
		i1.Counter = 30
		i2.Counter = 11
		if err := txn.Save(i1, i2); err != nil {
			return nil
		}
		return nil
	})
	checkErr(t, err)

	// Model2 Save j1
	j1.Counter = -1
	j1.Name = "Textile33"
	checkErr(t, m2.Save(j1))

	checkErr(t, m1.Delete(i1.ID))

	// Model2 Delete
	checkErr(t, m2.Delete(j1.ID))

	// Model2 Delete i2
	checkErr(t, m1.Delete(i2.ID))

	l.Close()
	cls()
	// Expected generated actions:
	// Model1 Save i1
	// Model1 Create i2
	// Model2 Create j1
	// Save i1
	// Save i2
	// Model2 Save j1
	// Delete i1
	// Model2 Delete j1
	// Delete i2

	return actions
}

type dummyModel struct {
	ID      core.EntityID
	Name    string
	Counter int
}

type mockEventCodec struct {
	called bool
}

var _ core.EventCodec = (*mockEventCodec)(nil)

func (dec *mockEventCodec) Reduce([]core.Event, ds.TxnDatastore, ds.Key, func(model string, key ds.Key, oldData, newData []byte, txn ds.Txn) error) ([]core.ReduceAction, error) {
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
