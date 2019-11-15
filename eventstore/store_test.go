package eventstore

import (
	"context"
	"io/ioutil"
	"os"
	"testing"
	"time"

	ds "github.com/ipfs/go-datastore"
	"github.com/multiformats/go-multiaddr"
	"github.com/textileio/go-textile-threads/core"
)

func TestE2E(t *testing.T) {
	t.Parallel()

	// peer1: Create store1, register a model, create and update an instance.
	tmpDir1, err := ioutil.TempDir("", "")
	checkErr(t, err)
	defer os.RemoveAll(tmpDir1)
	s1, err := NewStore(WithRepoPath(tmpDir1), WithListenPort(8005))
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
	s2, err := NewStore(WithRepoPath(tmpDir2), WithListenPort(8006))
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

	ctx, cancel := context.WithCancel(context.Background())
	ts, err := newDefaultThreadservice(ctx, 8008, tmpDir, true)
	checkErr(t, err)
	ec := &mockEventCodec{}
	s, err := NewStore(WithDebug(true), WithEventCodec(ec), WithListenPort(8008), WithRepoPath(tmpDir), WithThreadservice(ts))
	checkErr(t, err)

	m, err := s.Register("dummy", &dummyModel{})
	checkErr(t, err)
	checkErr(t, m.Create(&dummyModel{Name: "Textile"}))

	if !ec.called {
		t.Fatalf("custom event codec wasn't called")
	}

	// Re-do again to re-use key. If something wasn't closed correctly, would fail
	s.Close()
	cancel()

	time.Sleep(time.Second * 1)
	ts, err = newDefaultThreadservice(context.Background(), 8008, tmpDir, true)
	checkErr(t, err)
	_, err = NewStore(WithDebug(true), WithEventCodec(ec), WithListenPort(8008), WithRepoPath(tmpDir), WithThreadservice(ts))
	checkErr(t, err)
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
