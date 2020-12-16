package db

import (
	"context"
	"errors"
	"io/ioutil"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	ds "github.com/ipfs/go-datastore"
	format "github.com/ipfs/go-ipld-format"
	"github.com/multiformats/go-multiaddr"
	"github.com/textileio/go-threads/common"
	"github.com/textileio/go-threads/core/app"
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

	n1, err := common.DefaultNetwork(
		common.WithNetBadgerPersistence(tmpDir1),
		common.WithNetHostAddr(util.FreeLocalAddr()),
		common.WithNetDebug(true),
	)
	checkErr(t, err)
	defer n1.Close()

	store, err := util.NewBadgerDatastore(tmpDir1, "eventstore", false)
	checkErr(t, err)
	defer store.Close()

	id1 := thread.NewIDV1(thread.Raw, 32)
	d1, err := NewDB(context.Background(), store, n1, id1)
	checkErr(t, err)
	defer d1.Close()
	c1, err := d1.NewCollection(CollectionConfig{
		Name:   "dummy",
		Schema: util.SchemaFromInstance(&dummy{}, false),
	})
	checkErr(t, err)
	dummyJSON := util.JSONFromInstance(dummy{Name: "Textile", Counter: 0})
	res, err := c1.Create(dummyJSON)
	checkErr(t, err)
	dummyJSON = util.SetJSONID(res, dummyJSON)
	dummyJSON = util.SetJSONProperty("Counter", 42, dummyJSON)
	err = c1.Save(dummyJSON)
	checkErr(t, err)

	// Make sure the thread can't be deleted directly
	err = n1.DeleteThread(context.Background(), id1)
	if !errors.Is(err, app.ErrThreadInUse) {
		t.Fatal("thread should have been protected from deletion")
	}

	// Boilerplate to generate peer1 thread-addr
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
	n2, err := common.DefaultNetwork(
		common.WithNetBadgerPersistence(tmpDir2),
		common.WithNetHostAddr(util.FreeLocalAddr()),
		common.WithNetDebug(true),
	)
	checkErr(t, err)
	defer n2.Close()

	ti, err := n1.GetThread(context.Background(), id1)
	checkErr(t, err)
	cc := CollectionConfig{
		Name:   "dummy",
		Schema: util.SchemaFromInstance(&dummy{}, false),
	}

	store2, err := util.NewBadgerDatastore(tmpDir2, "eventstore", false)
	checkErr(t, err)
	defer store2.Close()

	d2, err := NewDBFromAddr(
		context.Background(),
		store2,
		n2,
		addr,
		ti.Key,
		WithNewCollections(cc),
	)
	checkErr(t, err)
	defer d2.Close()
	c2 := d1.GetCollection("dummy")
	checkErr(t, err)

	time.Sleep(time.Second * 3) // Wait a bit for sync

	dummy2JSON, err := c2.FindByID(res)
	checkErr(t, err)

	dummyInstance := &dummy{}
	util.InstanceFromJSON(dummyJSON, dummyInstance)
	dummy2Instance := &dummy{}
	util.InstanceFromJSON(dummy2JSON, dummy2Instance)

	if dummy2Instance.Name != dummyInstance.Name || dummy2Instance.Counter != dummyInstance.Counter {
		t.Fatalf("instances of both peers must be equal after sync")
	}
}

func TestMissingCollection(t *testing.T) {
	t.Parallel()

	tmpDir, err := ioutil.TempDir("", "")
	checkErr(t, err)
	defer os.RemoveAll(tmpDir)

	n, err := common.DefaultNetwork(
		common.WithNetBadgerPersistence(tmpDir),
		common.WithNetHostAddr(util.FreeLocalAddr()),
		common.WithNetDebug(true),
	)
	checkErr(t, err)
	defer n.Close()

	store, err := util.NewBadgerDatastore(tmpDir, "eventstore", false)
	checkErr(t, err)
	defer store.Close()

	id := thread.NewIDV1(thread.Raw, 32)
	db, err := NewDB(context.Background(), store, n, id)
	checkErr(t, err)
	defer db.Close()
	c, err := db.NewCollection(CollectionConfig{
		Name:   "dummy",
		Schema: util.SchemaFromInstance(&dummy{}, false),
	})
	checkErr(t, err)
	// Delete collection from map to "simulate" not having received it yet...
	// Motivated by https://github.com/textileio/go-threads/issues/379
	delete(db.collections, "dummy")
	dummyJSON := util.JSONFromInstance(dummy{Name: "Textile", Counter: 0})
	_, err = c.Create(dummyJSON)
	if err == nil || !strings.HasSuffix(err.Error(), "not found") {
		t.Fatal("expected error when indexing created data: collection (dummy) should not be found")
	}
}

func TestWithNewName(t *testing.T) {
	t.Parallel()
	tmpDir, err := ioutil.TempDir("", "")
	checkErr(t, err)
	defer os.RemoveAll(tmpDir)

	n, err := common.DefaultNetwork(
		common.WithNetBadgerPersistence(tmpDir),
		common.WithNetHostAddr(util.FreeLocalAddr()),
		common.WithNetDebug(true),
	)
	checkErr(t, err)

	store, err := util.NewBadgerDatastore(tmpDir, "eventstore", false)
	checkErr(t, err)
	defer store.Close()

	name := "my-db"
	id := thread.NewIDV1(thread.Raw, 32)
	d, err := NewDB(context.Background(), store, n, id, WithNewName(name))
	checkErr(t, err)
	if d.name != name {
		t.Fatalf("expected name %s, got %s", name, d.name)
	}

	info, err := d.GetDBInfo()
	checkErr(t, err)

	// Re-do again to re-use key. If something wasn't closed correctly, would fail
	checkErr(t, n.Close())
	checkErr(t, d.Close())

	time.Sleep(time.Second * 3)
	n, err = common.DefaultNetwork(
		common.WithNetBadgerPersistence(tmpDir),
		common.WithNetHostAddr(util.FreeLocalAddr()),
		common.WithNetDebug(true),
	)
	checkErr(t, err)
	defer n.Close()
	d, err = NewDB(context.Background(), store, n, id, WithNewKey(info.Key))
	checkErr(t, err)
	defer d.Close()
	if d.name != name {
		t.Fatalf("expected name %s, got %s", name, d.name)
	}
}

func TestWithNewEventCodec(t *testing.T) {
	t.Parallel()
	tmpDir, err := ioutil.TempDir("", "")
	checkErr(t, err)
	defer os.RemoveAll(tmpDir)

	n, err := common.DefaultNetwork(
		common.WithNetBadgerPersistence(tmpDir),
		common.WithNetHostAddr(util.FreeLocalAddr()),
		common.WithNetDebug(true),
	)
	checkErr(t, err)

	store, err := util.NewBadgerDatastore(tmpDir, "eventstore", false)
	checkErr(t, err)
	defer store.Close()

	ec := &mockEventCodec{}
	id := thread.NewIDV1(thread.Raw, 32)
	d, err := NewDB(context.Background(), store, n, id, WithNewEventCodec(ec))
	checkErr(t, err)

	m, err := d.NewCollection(CollectionConfig{
		Name:   "dummy",
		Schema: util.SchemaFromInstance(&dummy{}, false),
	})
	checkErr(t, err)
	_, err = m.Create(util.JSONFromInstance(dummy{Name: "Textile"}))
	checkErr(t, err)

	if !ec.called {
		t.Fatalf("custom event codec wasn't called")
	}

	info, err := d.GetDBInfo()
	checkErr(t, err)

	// Re-do again to re-use key. If something wasn't closed correctly, would fail
	checkErr(t, n.Close())
	checkErr(t, d.Close())

	time.Sleep(time.Second * 3)
	n, err = common.DefaultNetwork(
		common.WithNetBadgerPersistence(tmpDir),
		common.WithNetHostAddr(util.FreeLocalAddr()),
		common.WithNetDebug(true),
	)
	checkErr(t, err)
	defer n.Close()
	d, err = NewDB(context.Background(), store, n, id, WithNewEventCodec(ec), WithNewKey(info.Key))
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
		actions := runListenersComplexUseCase(
			t,
			ListenOption{Collection: "Collection1"},
			ListenOption{Collection: "Collection2", Type: ListenDelete},
		)
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
		Schema: util.SchemaFromInstance(&dummy{}, false),
	}
	c1, err := d.NewCollection(cc1)
	checkErr(t, err)
	cc2 := CollectionConfig{
		Name:   "Collection2",
		Schema: util.SchemaFromInstance(&dummy{}, false),
	}
	c2, err := d.NewCollection(cc2)
	checkErr(t, err)

	// Create some instance *before* any listener, just to test doesn't appear
	// on listener Action stream.
	i1 := util.JSONFromInstance(dummy{ID: "id-i1", Name: "Textile1"})
	i1Ids, err := c1.Create(i1)
	checkErr(t, err)

	l, err := d.Listen(los...)
	checkErr(t, err)
	var actions []Action
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		for a := range l.Channel() {
			actions = append(actions, a)
		}
		wg.Done()
	}()

	// Collection1 Save i1
	i1 = util.SetJSONProperty("Name", "Textile0", i1)
	err = c1.Save(i1)
	checkErr(t, err)

	// Collection1 Create i2
	i2 := util.JSONFromInstance(dummy{ID: "id-i2", Name: "Textile2"})
	i2Ids, err := c1.Create(i2)
	checkErr(t, err)

	// Collection2 Create j1
	j1 := util.JSONFromInstance(dummy{ID: "id-j1", Name: "Textile3"})
	j1Ids, err := c2.Create(j1)
	checkErr(t, err)

	// Collection1 Save i1
	// Collection1 Save i2
	err = c1.WriteTxn(func(txn *Txn) error {
		i1 = util.SetJSONProperty("Counter", 30, i1)
		i2 = util.SetJSONProperty("Counter", 11, i2)
		return txn.Save(i1, i2)
	})
	checkErr(t, err)

	// Collection2 Save j1
	j1 = util.SetJSONProperty("Counter", -1, j1)
	j1 = util.SetJSONProperty("Name", "Textile33", j1)
	err = c2.Save(j1)
	checkErr(t, err)

	checkErr(t, c1.Delete(i1Ids))

	// Collection2 Delete
	checkErr(t, c2.Delete(j1Ids))

	// Collection2 Delete i2
	checkErr(t, c1.Delete(i2Ids))

	l.Close()
	cls()
	wg.Wait()
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
	ID      core.InstanceID `json:"_id"`
	Name    string
	Counter int
}

type mockEventCodec struct {
	called bool
}

var _ core.EventCodec = (*mockEventCodec)(nil)

func (dec *mockEventCodec) Reduce(
	[]core.Event,
	ds.TxnDatastore,
	ds.Key,
	core.IndexFunc,
) ([]core.ReduceAction, error) {
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
