package main

import (
	"context"
	"fmt"

	"github.com/libp2p/go-libp2p-core/host"
	kaddht "github.com/libp2p/go-libp2p-kad-dht"
	tserv "github.com/textileio/go-textile-core/threadservice"
	"github.com/textileio/go-textile-threads/exe/util"
)

var (
	dht *kaddht.IpfsDHT
	api tserv.Threadservice

	bootstrapPeers = []string{
		"/ip4/104.210.43.77/tcp/4001/ipfs/12D3KooWSdGmRz5JQidqrtmiPGVHkStXpbSAMnbCcW8abq6zuiDP", // us-west
		"/ip4/20.39.232.27/tcp/4001/ipfs/12D3KooWLnUv9MWuRM6uHirRPBM4NwRj54n4gNNnBtiFiwPiv3Up",  // eu-west
		"/ip4/34.87.103.105/tcp/4001/ipfs/12D3KooWA5z2C3z1PNKi36Bw1MxZhBD8nv7UbB7YQP6WcSWYNwRQ", // as-southeast
	}
)

func main() {
	var cancel context.CancelFunc
	var h host.Host
	_, cancel, _, h, dht, api = util.Build(bootstrapPeers)

	defer cancel()
	defer h.Close()
	defer dht.Close()
	defer api.Close()

	fmt.Println("Welcome to Threads!")
	fmt.Println("Your peer ID is " + h.ID().String())

	select {}
}
