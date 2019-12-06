package eventstore

import (
	"io/ioutil"
	"os"
	"reflect"
	"testing"
	"time"

	ds "github.com/ipfs/go-datastore"
	ipldformat "github.com/ipfs/go-ipld-format"
	"github.com/multiformats/go-multiaddr"
	core "github.com/textileio/go-textile-core/store"
)

func TestE2EWithThreads(t *testing.T) {
	t.Parallel()

	// peer1: Create store1, register a model, create and update an instance.
	tmpDir1, err := ioutil.TempDir("", "")
	checkErr(t, err)
	defer os.RemoveAll(tmpDir1)
	ts1, err := DefaultThreadservice(tmpDir1, ProxyPort(0))
	checkErr(t, err)
	defer ts1.Close()

	s1, err := NewStore(ts1, WithRepoPath(tmpDir1))
	checkErr(t, err)
	defer s1.Close()
	m1, err := s1.Register("dummy", &dummyModel{})
	checkErr(t, err)
	checkErr(t, s1.Start())
	dummyInstance := &dummyModel{Name: "Textile", Counter: 0}
	checkErr(t, m1.Create(dummyInstance))
	dummyInstance.Counter += 42
	checkErr(t, m1.Save(dummyInstance))

	// Boilerplate to generate peer1 thread-addr and get follow/read keys
	peer1ThreadStore := s1.Threadservice().Store()
	threadID, _, err := s1.ThreadID()
	checkErr(t, err)
	threadInfo, err := peer1ThreadStore.ThreadInfo(threadID)
	checkErr(t, err)
	peer1Addr := s1.Threadservice().Host().Addrs()[0]
	peer1ID, err := multiaddr.NewComponent("p2p", s1.Threadservice().Host().ID().String())
	checkErr(t, err)
	threadComp, err := multiaddr.NewComponent("thread", threadID.String())
	checkErr(t, err)
	threadAddr := peer1Addr.Encapsulate(peer1ID).Encapsulate(threadComp)

	// Create a completely parallel store, which will sync with the previous one
	// and should have the same state of dummyInstance.
	tmpDir2, err := ioutil.TempDir("", "")
	checkErr(t, err)
	defer os.RemoveAll(tmpDir2)
	ts2, err := DefaultThreadservice(tmpDir2, ProxyPort(0))
	checkErr(t, err)
	defer ts2.Close()

	s2, err := NewStore(ts2, WithRepoPath(tmpDir2))
	checkErr(t, err)
	defer s2.Close()
	m2, err := s2.Register("dummy", &dummyModel{})
	checkErr(t, err)
	checkErr(t, s2.StartFromAddr(threadAddr, threadInfo.FollowKey, threadInfo.ReadKey))

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

	ts, err := DefaultThreadservice(tmpDir, ProxyPort(0))
	checkErr(t, err)

	ec := &mockEventCodec{}
	s, err := NewStore(ts, WithRepoPath(tmpDir), WithEventCodec(ec))
	checkErr(t, err)

	m, err := s.Register("dummy", &dummyModel{})
	checkErr(t, err)
	checkErr(t, m.Create(&dummyModel{Name: "Textile"}))

	if !ec.called {
		t.Fatalf("custom event codec wasn't called")
	}

	// Re-do again to re-use key. If something wasn't closed correctly, would fail
	ts.Close()
	s.Close()

	time.Sleep(time.Second * 3)
	ts, err = DefaultThreadservice(tmpDir, ProxyPort(0))
	checkErr(t, err)
	defer ts.Close()
	s, err = NewStore(ts, WithRepoPath(tmpDir), WithEventCodec(ec))
	checkErr(t, err)
	defer s.Close()
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

	t.Run("AllStoreEvents", func(t *testing.T) {
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
		expected := []Action{}
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

// runListenersComplexUseCase runs a complex store use-case, and returns
// Actions received with the ...ListenOption provided.
func runListenersComplexUseCase(t *testing.T, los ...ListenOption) []Action {
	t.Helper()
	s, close := createTestStore(t)
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
	close()
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

func (dec *mockEventCodec) Reduce(events []core.Event, datastore ds.TxnDatastore, baseKey ds.Key) ([]core.ReduceAction, error) {
	dec.called = true
	return nil, nil
}
func (dec *mockEventCodec) Create(ops []core.Action) ([]core.Event, ipldformat.Node, error) {
	dec.called = true
	return nil, nil, nil
}
func (dec *mockEventCodec) EventsFromBytes(data []byte) ([]core.Event, error) {
	dec.called = true
	return nil, nil
}
