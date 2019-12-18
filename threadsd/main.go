package main

import (
	"context"
	"flag"
	"fmt"

	logging "github.com/ipfs/go-log"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/textileio/go-threads/api"
	es "github.com/textileio/go-threads/eventstore"
	"github.com/textileio/go-threads/util"
)

var log = logging.Logger("threadsd")

func main() {
	repo := flag.String("repo", ".threads", "repo location")
	hostAddrStr := flag.String("hostAddr", "/ip4/0.0.0.0/tcp/4006", "Threads host bind address")
	hostProxyAddrStr := flag.String("hostProxyAddr", "/ip4/0.0.0.0/tcp/5006", "Threads gRPC proxy bind address")
	apiAddrStr := flag.String("apiAddr", "/ip4/127.0.0.1/tcp/6006", "API bind address")
	apiProxyAddrStr := flag.String("apiProxyAddr", "/ip4/127.0.0.1/tcp/7006", "API gRPC proxy bind address")
	debug := flag.Bool("debug", false, "Enable debug logging")
	flag.Parse()

	hostAddr, err := ma.NewMultiaddr(*hostAddrStr)
	if err != nil {
		log.Fatal(err)
	}
	hostProxyAddr, err := ma.NewMultiaddr(*hostProxyAddrStr)
	if err != nil {
		log.Fatal(err)
	}
	apiAddr, err := ma.NewMultiaddr(*apiAddrStr)
	if err != nil {
		log.Fatal(err)
	}
	apiProxyAddr, err := ma.NewMultiaddr(*apiProxyAddrStr)
	if err != nil {
		log.Fatal(err)
	}

	util.SetupDefaultLoggingConfig(*repo)
	if *debug {
		if err := logging.SetLogLevel("threadsd", "debug"); err != nil {
			log.Fatal(err)
		}
	}

	ts, err := es.DefaultThreadservice(
		*repo,
		es.HostAddr(hostAddr),
		es.HostProxyAddr(hostProxyAddr),
		es.Debug(*debug))
	if err != nil {
		log.Fatal(err)
	}
	defer ts.Close()
	ts.Bootstrap(util.DefaultBoostrapPeers())

	server, err := api.NewServer(context.Background(), ts, api.Config{
		RepoPath:  *repo,
		Addr:      apiAddr,
		ProxyAddr: apiProxyAddr,
		Debug:     *debug,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer server.Close()

	fmt.Println("Welcome to Threads!")
	fmt.Println("Your peer ID is " + ts.Host().ID().String())

	log.Debug("threadsd started")

	select {}
}
