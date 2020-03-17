package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/mr-tron/base58"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/textileio/go-threads/crypto/symmetric"
	"github.com/textileio/go-threads/db"
)

func runReaderPeer(repo string) {
	fmt.Printf("I'm a collection reader.\n")
	writerAddr, fkey, rkey := getWriterAddr()

	n, err := db.DefaultNetwork(repo)
	checkErr(err)
	defer n.Close()

	d, err := db.NewDBFromAddr(context.Background(), n, writerAddr, fkey, rkey, db.WithRepoPath(repo))
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

func getWriterAddr() (ma.Multiaddr, *symmetric.Key, *symmetric.Key) {
	// Read the multiaddr of the writer which saved it in .full_simple file.
	mb, err := ioutil.ReadFile(".e2e_counter_writeraddr")
	checkErr(err)
	data := strings.Split(string(mb), " ")

	fmt.Printf("Will connect to: %s\n", data[0])
	addr, err := ma.NewMultiaddr(data[0])
	checkErr(err)

	fkeyBytes, err := base58.Decode(data[1])
	checkErr(err)
	rkeyBytes, err := base58.Decode(data[2])
	checkErr(err)

	fkey, err := symmetric.FromBytes(fkeyBytes)
	checkErr(err)
	rkey, err := symmetric.FromBytes(rkeyBytes)
	checkErr(err)

	return addr, fkey, rkey
}
