package main

import (
	"flag"
	"fmt"

	logging "github.com/ipfs/go-log"
	"github.com/textileio/go-textile-threads/exe/util"
)

var log = logging.Logger("daemon")

func main() {
	repo := flag.String("repo", ".threads", "repo location")
	port := flag.Int("port", 4006, "host port")
	proxyAddr := flag.String("proxy", "", "proxy server address")
	flag.Parse()

	if err := logging.SetLogLevel("daemon", "debug"); err != nil {
		panic(err)
	}

	_, cancel, _, h, dht, api := util.Build(*repo, *port, *proxyAddr, true)

	defer cancel()
	defer dht.Close()
	defer api.Close()

	fmt.Println("Welcome to Threads!")
	fmt.Println("Your peer ID is " + h.ID().String())

	log.Debug("daemon started")

	select {}
}
