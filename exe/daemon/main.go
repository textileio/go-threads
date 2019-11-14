package main

import (
	"context"
	"flag"
	"fmt"

	logging "github.com/ipfs/go-log"
	tserv "github.com/textileio/go-textile-core/threadservice"
	"github.com/textileio/go-textile-threads/exe/util"
)

var (
	api tserv.Threadservice

	log = logging.Logger("shell")
)

func main() {
	repo := flag.String("repo", ".threads", "repo location")
	port := flag.Int("port", 4006, "host port")
	proxyAddr := flag.String("proxy", "", "proxy server address")
	flag.Parse()

	if err := logging.SetLogLevel("daemon", "debug"); err != nil {
		panic(err)
	}

	var cancel context.CancelFunc
	_, cancel, _, api = util.Build(*repo, *port, *proxyAddr, true)

	defer cancel()
	defer api.Close()

	fmt.Println("Welcome to Threads!")
	fmt.Println("Your peer ID is " + api.DHT().Host().ID().String())

	log.Debug("daemon started")

	select {}
}
