package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"strings"

	ma "github.com/multiformats/go-multiaddr"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/go-threads/db"
)

func runReaderPeer(repo string) {
	fmt.Printf("I'm a collection reader.\n")
	writerAddr, key := getWriterAddr()

	n, err := db.DefaultNetwork(repo)
	checkErr(err)
	defer n.Close()

	d, err := db.NewDBFromAddr(context.Background(), n, writerAddr, key, db.WithRepoPath(repo))
	checkErr(err)
	defer d.Close()

	c, err := d.NewCollectionFromInstance("counter", &myCounter{})
	checkErr(err)

	l, err := d.Listen()
	checkErr(err)
	for range l.Channel() {
		err := c.ReadTxn(func(txn *db.Txn) error {
			var res []*myCounter
			if err := txn.Find(&res, nil); err != nil {
				return err
			}
			for _, c := range res {
				fmt.Printf("Counter %s: has value %d\n", c.Name, c.Count)
			}
			return nil
		})
		checkErr(err)
	}
}

func getWriterAddr() (ma.Multiaddr, thread.Key) {
	// Read the multiaddr of the writer which saved it in .full_simple file.
	mb, err := ioutil.ReadFile(".e2e_counter_writeraddr")
	checkErr(err)
	data := strings.Split(string(mb), " ")

	fmt.Printf("Will connect to: %s\n", data[0])
	addr, err := ma.NewMultiaddr(data[0])
	checkErr(err)

	key, err := thread.KeyFromString(data[1])
	checkErr(err)

	return addr, key
}
