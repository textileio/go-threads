package main

import (
	"context"
	"flag"
	"fmt"

	logging "github.com/ipfs/go-log"
	"github.com/textileio/go-textile-threads/api"
	es "github.com/textileio/go-textile-threads/eventstore"
	"github.com/textileio/go-textile-threads/util"
)

var log = logging.Logger("daemon")

func main() {
	repo := flag.String("repo", ".threads", "repo location")
	listenPort := flag.Int("port", 4006, "host port")
	proxyPort := flag.Int("proxyPort", 5050, "grpc proxy port")
	flag.Parse()

	if err := logging.SetLogLevel("daemon", "debug"); err != nil {
		log.Fatal(err)
	}

	ts, err := es.DefaultThreadservice(
		*repo,
		es.ListenPort(*listenPort),
		es.ProxyPort(*proxyPort),
		es.Debug(true))
	if err != nil {
		log.Fatal(err)
	}
	defer ts.Close()
	ts.Bootstrap(util.DefaultBoostrapPeers())

	server, err := api.NewServer(context.Background(), ts, api.Config{
		RepoPath: *repo,
		Debug:    true,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer server.Close()

	fmt.Println("Welcome to Threads!")
	fmt.Println("Your peer ID is " + ts.Host().ID().String())

	log.Debug("daemon started")

	select {}
}
