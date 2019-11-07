package main

import (
	"context"
	"flag"
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
	repo := flag.String("repo", ".threads", "repo location")
	port := flag.Int("port", 4006, "host port")
	flag.Parse()

	if err := logging.SetLogLevel("daemon", "debug"); err != nil {
		panic(err)
	}

	var cancel context.CancelFunc
	var h host.Host
	_, cancel, _, h, dht, api = util.Build(*repo, *port, true)

	defer cancel()
	defer h.Close()
	defer dht.Close()
	defer api.Close()

	fmt.Println("Welcome to Threads!")
	fmt.Println("Your peer ID is " + h.ID().String())

	log.Debug("daemon started")

	select {}
}
