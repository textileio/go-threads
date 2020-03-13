package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/mr-tron/base58"
	"github.com/multiformats/go-multiaddr"
	core "github.com/textileio/go-threads/core/db"
	s "github.com/textileio/go-threads/core/service"
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

	ts, err := db.DefaultService(repo)
	checkErr(err)
	defer ts.Close()
	id := thread.NewIDV1(thread.Raw, 32)
	d, err := db.NewDB(context.Background(), ts, id, db.WithRepoPath(repo))
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

	saveThreadMultiaddrForOtherPeer(ts, id)

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

func saveThreadMultiaddrForOtherPeer(ts s.Service, threadID thread.ID) {
	tinfo, err := ts.GetThread(context.Background(), threadID)
	checkErr(err)

	// Create listen addr
	id, _ := multiaddr.NewComponent("p2p", ts.Host().ID().String())
	threadComp, _ := multiaddr.NewComponent("thread", threadID.String())

	listenAddr := ts.Host().Addrs()[0].Encapsulate(id).Encapsulate(threadComp).String()
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
