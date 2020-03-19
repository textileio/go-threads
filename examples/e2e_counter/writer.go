package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/multiformats/go-multiaddr"
	core "github.com/textileio/go-threads/core/db"
	"github.com/textileio/go-threads/core/net"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/go-threads/db"
)

type myCounter struct {
	ID    core.InstanceID
	Name  string
	Count int
}

func runWriterPeer(repo string) {
	fmt.Printf("I'm a writer\n")

	n, err := db.DefaultNetwork(repo)
	checkErr(err)
	defer n.Close()
	id := thread.NewIDV1(thread.Raw, 32)
	d, err := db.NewDB(context.Background(), n, id, db.WithRepoPath(repo))
	checkErr(err)
	defer d.Close()

	m, err := d.NewCollectionFromInstance("counter", &myCounter{})
	checkErr(err)

	var counter *myCounter
	var counters []*myCounter
	checkErr(m.Find(&counters, db.Where("Name").Eq("TestCounter")))
	if len(counters) > 0 {
		counter = counters[0]
	} else {
		counter = &myCounter{Name: "TestCounter", Count: 0}
		checkErr(m.Create(counter))
	}

	saveThreadMultiaddrForOtherPeer(n, id)

	ticker1 := time.NewTicker(time.Millisecond * 1000)
	for range ticker1.C {
		err = m.WriteTxn(func(txn *db.Txn) error {
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

func saveThreadMultiaddrForOtherPeer(n net.Net, threadID thread.ID) {
	tinfo, err := n.GetThread(context.Background(), threadID)
	checkErr(err)

	// Create listen addr
	id, _ := multiaddr.NewComponent("p2p", n.Host().ID().String())
	threadComp, _ := multiaddr.NewComponent("thread", threadID.String())

	listenAddr := n.Host().Addrs()[0].Encapsulate(id).Encapsulate(threadComp).String()
	key := tinfo.Key.String()

	data := fmt.Sprintf("%s %s", listenAddr, key)
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
