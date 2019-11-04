package main

import (
	"context"
	"fmt"

	logging "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p-core/host"
	kaddht "github.com/libp2p/go-libp2p-kad-dht"
	tserv "github.com/textileio/go-textile-core/threadservice"
	"github.com/textileio/go-textile-threads/exe/util"
)

var (
	dht *kaddht.IpfsDHT
	api tserv.Threadservice

	log = logging.Logger("shell")
)

func main() {
	if err := logging.SetLogLevel("daemon", "debug"); err != nil {
		panic(err)
	}

	var cancel context.CancelFunc
	var h host.Host
	_, cancel, _, h, dht, api = util.Build()

	defer cancel()
	defer h.Close()
	defer dht.Close()
	defer api.Close()

	fmt.Println("Welcome to Threads!")
	fmt.Println("Your peer ID is " + h.ID().String())

	log.Debug("daemon started")

	select {}
}
