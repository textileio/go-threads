package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"strings"

	ma "github.com/multiformats/go-multiaddr"
	"github.com/textileio/go-threads/common"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/go-threads/db"
	"github.com/textileio/go-threads/util"
)

func runReaderPeer(repo string) {
	fmt.Printf("I'm a collection reader.\n")
	writerAddr, key := getWriterAddr()

	n, err := common.DefaultNetwork(repo, common.WithNetDebug(true), common.WithNetHostAddr(util.FreeLocalAddr()))
	checkErr(err)
	defer n.Close()

	cc := db.CollectionConfig{
		Name:   "counter",
		Schema: util.SchemaFromInstance(&myCounter{}, false),
	}

	d, err := db.NewDBFromAddr(context.Background(), n, writerAddr, key, db.WithNewRepoPath(repo), db.WithNewCollections(cc))
	checkErr(err)
	defer d.Close()

	c := d.GetCollection("counter")

	l, err := d.Listen()
	checkErr(err)
	for range l.Channel() {
		err := c.ReadTxn(func(txn *db.Txn) error {
			res, err := txn.Find(&db.Query{})
			if err != nil {
				return err
			}
			for i, c := range res {
				fmt.Printf("Counter %v: has value %v\n", i, c)
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
