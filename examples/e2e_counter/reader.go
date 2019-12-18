package main

import (
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/mr-tron/base58"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/textileio/go-textile-core/crypto/symmetric"
	es "github.com/textileio/go-threads/eventstore"
)

func runReaderPeer(repo string) {
	fmt.Printf("I'm a model reader.\n")
	writerAddr, fkey, rkey := getWriterAddr()

	ts, err := es.DefaultThreadservice(repo)
	checkErr(err)
	defer ts.Close()
	store, err := es.NewStore(ts, es.WithRepoPath(repo))
	checkErr(err)
	defer store.Close()

	m, err := store.Register("counter", &myCounter{})
	checkErr(err)

	l, err := store.Listen()
	checkErr(err)
	checkErr(store.StartFromAddr(writerAddr, fkey, rkey))
	for range l.Channel() {
		err := m.ReadTxn(func(txn *es.Txn) error {
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

	fkey, err := symmetric.NewKey(fkeyBytes)
	checkErr(err)
	rkey, err := symmetric.NewKey(rkeyBytes)
	checkErr(err)

	return addr, fkey, rkey
}
