package eventstore

import (
	"io/ioutil"
	"os"
	"testing"
	"time"

	ds "github.com/ipfs/go-datastore"
	"github.com/multiformats/go-multiaddr"
	core "github.com/textileio/go-textile-core/store"
	"github.com/textileio/go-textile-threads/util"
)

func TestE2EWithThreads(t *testing.T) {
	t.Parallel()

	// peer1: Create store1, register a model, create and update an instance.
	tmpDir1, err := ioutil.TempDir("", "")
	checkErr(t, err)
	defer os.RemoveAll(tmpDir1)
	ts1, err := DefaultThreadservice(0, tmpDir1, false)
	checkErr(t, err)
	ts1.Bootstrap(util.DefaultBoostrapPeers())
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
	ts2, err := DefaultThreadservice(0, tmpDir2, false)
	checkErr(t, err)
	ts2.Bootstrap(util.DefaultBoostrapPeers())
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

	ts, err := DefaultThreadservice(0, tmpDir, false)
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
	ts, err = DefaultThreadservice(0, tmpDir, false)
	checkErr(t, err)
	defer ts.Close()
	s, err = NewStore(ts, WithRepoPath(tmpDir), WithEventCodec(ec))
	checkErr(t, err)
	defer s.Close()
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

func (dec *mockEventCodec) Reduce(e core.Event, datastore ds.Datastore, baseKey ds.Key) error {
	dec.called = true
	return nil
}
func (dec *mockEventCodec) Create(ops []core.Action) ([]core.Event, error) {
	dec.called = true
	return nil, nil
}
func (dec *mockEventCodec) EventFromBytes(data []byte) (core.Event, error) {
	dec.called = true
	return nil, nil
}
