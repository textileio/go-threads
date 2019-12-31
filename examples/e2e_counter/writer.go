package main

import (
	"fmt"
	"io/ioutil"
	"time"

	"github.com/mr-tron/base58"
	"github.com/multiformats/go-multiaddr"
	"github.com/textileio/go-threads/core/service"
	core "github.com/textileio/go-threads/core/store"
	s "github.com/textileio/go-threads/store"
)

type myCounter struct {
	ID    core.EntityID
	Name  string
	Count int
}

func runWriterPeer(repo string) {
	fmt.Printf("I'm a writer\n")

	ts, err := s.DefaultService(repo)
	checkErr(err)
	defer ts.Close()
	store, err := s.NewStore(ts, s.WithRepoPath(repo))
	checkErr(err)
	defer store.Close()

	m, err := store.Register("counter", &myCounter{})
	checkErr(err)
	checkErr(store.Start())
	checkErr(err)

	var counter *myCounter
	var counters []*myCounter
	checkErr(m.Find(&counters, s.Where("Name").Eq("TestCounter")))
	if len(counters) > 0 {
		counter = counters[0]
	} else {
		counter = &myCounter{Name: "TestCounter", Count: 0}
		checkErr(m.Create(counter))
	}

	threadID, _, err := store.ThreadID()
	checkErr(err)
	saveThreadMultiaddrForOtherPeer(store, threadID)

	ticker1 := time.NewTicker(time.Millisecond * 1000)
	for range ticker1.C {
		err = m.WriteTxn(func(txn *s.Txn) error {
			c := &myCounter{}
			if err = txn.FindByID(counter.ID, c); err != nil {
				return err
			}
			c.Count++
			fmt.Printf("%d ,", c.Count)
			return txn.Save(c)
		})
		checkErr(err)
	}
}

func saveThreadMultiaddrForOtherPeer(store *s.Store, threadID service.ID) {
	host := store.Service().Host()
	tinfo, err := store.Service().Store().ThreadInfo(threadID)
	checkErr(err)

	// Create listen addr
	id, _ := multiaddr.NewComponent("p2p", host.ID().String())
	threadComp, _ := multiaddr.NewComponent("thread", threadID.String())

	listenAddr := host.Addrs()[0].Encapsulate(id).Encapsulate(threadComp).String()
	followKey := base58.Encode(tinfo.FollowKey.Bytes())
	readKey := base58.Encode(tinfo.ReadKey.Bytes())

	data := fmt.Sprintf("%s %s %s", listenAddr, followKey, readKey)
	if err := ioutil.WriteFile(".e2e_counter_writeraddr", []byte(data), 0644); err != nil {
		panic(err)
	}
	checkErr(err)
	fmt.Println("Connect info: " + data)
}

func checkErr(err error) {
	if err != nil {
		panic(err)
	}
}
