package main

import (
	"fmt"
	"io/ioutil"
	"time"

	"github.com/mr-tron/base58"
	"github.com/multiformats/go-multiaddr"
	core "github.com/textileio/go-textile-core/store"
	"github.com/textileio/go-textile-core/thread"
	es "github.com/textileio/go-textile-threads/eventstore"
)

type myCounter struct {
	ID    core.EntityID
	Name  string
	Count int
}

func runWriterPeer(repo string) {
	fmt.Printf("I'm a writer\n")

	ts, err := es.DefaultThreadservice(repo)
	checkErr(err)
	defer ts.Close()
	store, err := es.NewStore(ts, es.WithRepoPath(repo))
	checkErr(err)
	defer store.Close()

	m, err := store.Register("counter", &myCounter{})
	checkErr(err)
	checkErr(store.Start())
	checkErr(err)

	var counter *myCounter
	var counters []*myCounter
	checkErr(m.Find(&counters, es.Where("Name").Eq("TestCounter")))
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
		err = m.WriteTxn(func(txn *es.Txn) error {
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

func saveThreadMultiaddrForOtherPeer(store *es.Store, threadID thread.ID) {
	host := store.Threadservice().Host()
	tinfo, err := store.Threadservice().Store().ThreadInfo(threadID)
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
