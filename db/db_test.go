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
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/go-threads/util"
)

func TestE2EWithThreads(t *testing.T) {
	t.Parallel()

	// peer1: Create db1, register a collection, create and update an instance.
	tmpDir1, err := ioutil.TempDir("", "")
	checkErr(t, err)
	defer os.RemoveAll(tmpDir1)

	n1, err := DefaultNetwork(tmpDir1)
	checkErr(t, err)
	defer n1.Close()

	id1 := thread.NewIDV1(thread.Raw, 32)
	d1, err := NewDB(context.Background(), n1, id1, WithRepoPath(tmpDir1))
	checkErr(t, err)
	defer d1.Close()
	c1, err := d1.NewCollection(CollectionConfig{
		Name:   "dummy",
		Schema: util.SchemaFromInstance(&dummy{}),
	})
	checkErr(t, err)
	dummyInstance := &dummy{Name: "Textile", Counter: 0}
	dummyInstanceString := util.JSONStringFromInstance(dummyInstance)
	checkErr(t, c1.Create(dummyInstanceString))
	updatedDummyInstance := &dummy{}
	util.InstanceFromJSONString(dummyInstanceString, updatedDummyInstance)
	updatedDummyInstance.Counter += 42
	checkErr(t, c1.Save(util.JSONStringFromInstance(updatedDummyInstance)))

	// Boilerplate to generate peer1 thread-addr
	// @todo: This should be a network method
	peer1Addr := n1.Host().Addrs()[0]
	peer1ID, err := multiaddr.NewComponent("p2p", n1.Host().ID().String())
	checkErr(t, err)
	threadComp, err := multiaddr.NewComponent("thread", id1.String())
	checkErr(t, err)
	addr := peer1Addr.Encapsulate(peer1ID).Encapsulate(threadComp)

	// Create a completely parallel db, which will sync with the previous one
	// and should have the same state of dummyInstance.
	tmpDir2, err := ioutil.TempDir("", "")
	checkErr(t, err)
	defer os.RemoveAll(tmpDir2)
	n2, err := DefaultNetwork(tmpDir2)
	checkErr(t, err)
	defer n2.Close()

	ti, err := n1.GetThread(context.Background(), id1)
	checkErr(t, err)
	cc := CollectionConfig{
		Name:   "dummy",
		Schema: util.SchemaFromInstance(&dummy{}),
	}
	d2, err := NewDBFromAddr(context.Background(), n2, addr, ti.Key, WithRepoPath(tmpDir2), WithCollections(cc))
	checkErr(t, err)
	defer d2.Close()
	c2 := d1.GetCollection("dummy")
	checkErr(t, err)

	time.Sleep(time.Second * 3) // Wait a bit for sync

	var dummyInstance2String *string
	checkErr(t, c2.FindByID(updatedDummyInstance.ID, dummyInstance2String))
	dummyInstance2 := &dummy{}
	util.InstanceFromJSONString(dummyInstance2String, dummyInstance2)
	if dummyInstance2.Name != updatedDummyInstance.Name || dummyInstance2.Counter != updatedDummyInstance.Counter {
		t.Fatalf("instances of both peers must be equal after sync")
	}
}

func TestOptions(t *testing.T) {
	t.Parallel()
	tmpDir, err := ioutil.TempDir("", "")
	checkErr(t, err)
	defer os.RemoveAll(tmpDir)

	n, err := DefaultNetwork(tmpDir)
	checkErr(t, err)

	ec := &mockEventCodec{}
	id := thread.NewIDV1(thread.Raw, 32)
	d, err := NewDB(context.Background(), n, id, WithRepoPath(tmpDir), WithEventCodec(ec))
	checkErr(t, err)

	m, err := d.NewCollection(CollectionConfig{
		Name:   "dummy",
		Schema: util.SchemaFromInstance(&dummy{}),
	})
	checkErr(t, err)
	checkErr(t, m.Create(util.JSONStringFromInstance(&dummy{Name: "Textile"})))

	if !ec.called {
		t.Fatalf("custom event codec wasn't called")
	}

	// Re-do again to re-use key. If something wasn't closed correctly, would fail
	checkErr(t, n.Close())
	checkErr(t, d.Close())

	time.Sleep(time.Second * 3)
	n, err = DefaultNetwork(tmpDir)
	checkErr(t, err)
	defer n.Close()
	d, err = NewDB(context.Background(), n, id, WithRepoPath(tmpDir), WithEventCodec(ec))
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
	d, cls := createTestDB(t)
	cc1 := CollectionConfig{
		Name:   "Collection1",
		Schema: util.SchemaFromInstance(&dummy{}),
	}
	c1, err := d.NewCollection(cc1)
	checkErr(t, err)
	cc2 := CollectionConfig{
		Name:   "Collection2",
		Schema: util.SchemaFromInstance(&dummy{}),
	}
	c2, err := d.NewCollection(cc2)
	checkErr(t, err)

	// Create some instance *before* any listener, just to test doesn't appear
	// on listener Action stream.
	i1 := &dummy{ID: "id-i1", Name: "Textile1"}
	checkErr(t, c1.Create(util.JSONStringFromInstance(i1)))

	l, err := d.Listen(los...)
	checkErr(t, err)
	var actions []Action
	go func() {
		for a := range l.Channel() {
			actions = append(actions, a)
		}
	}()

	// Collection1 Save i1
	i1.Name = "Textile0"
	checkErr(t, c1.Save(util.JSONStringFromInstance(i1)))

	// Collection1 Create i2
	i2 := &dummy{ID: "id-i2", Name: "Textile2"}
	checkErr(t, c1.Create(util.JSONStringFromInstance(i2)))

	// Collection2 Create j1
	j1 := &dummy{ID: "id-j1", Name: "Textile3"}
	checkErr(t, c2.Create(util.JSONStringFromInstance(j1)))

	// Collection1 Save i1
	// Collection1 Save i2
	err = c1.WriteTxn(func(txn *Txn) error {
		i1.Counter = 30
		i2.Counter = 11
		if err := txn.Save(util.JSONStringFromInstance(i1), util.JSONStringFromInstance(i2)); err != nil {
			return nil
		}
		return nil
	})
	checkErr(t, err)

	// Collection2 Save j1
	j1.Counter = -1
	j1.Name = "Textile33"
	checkErr(t, c2.Save(util.JSONStringFromInstance(j1)))

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
