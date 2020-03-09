package main

import (
	"context"
	"fmt"
	"os"

	logging "github.com/ipfs/go-log"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/namsral/flag"
	"github.com/textileio/go-threads/api"
	"github.com/textileio/go-threads/db"
	serviceapi "github.com/textileio/go-threads/service/api"
	"github.com/textileio/go-threads/util"
)

var log = logging.Logger("threadsd")

func main() {
	fs := flag.NewFlagSetWithEnvPrefix(os.Args[0], "THRDS", 0)

	repo := fs.String("repo", ".threads", "repo location")
	hostAddrStr := fs.String("hostAddr", "/ip4/0.0.0.0/tcp/4006", "Threads host bind address")
	serviceApiAddrStr := fs.String("serviceApiAddr", "/ip4/127.0.0.1/tcp/5006", "Threads service API bind address")
	serviceApiProxyAddrStr := fs.String("serviceApiProxyAddr", "/ip4/127.0.0.1/tcp/5007", "Threads service API gRPC proxy bind address")
	apiAddrStr := fs.String("apiAddr", "/ip4/127.0.0.1/tcp/6006", "API bind address")
	apiProxyAddrStr := fs.String("apiProxyAddr", "/ip4/127.0.0.1/tcp/6007", "API gRPC proxy bind address")
	debug := fs.Bool("debug", false, "Enable debug logging")
	if err := fs.Parse(os.Args[1:]); err != nil {
		log.Fatal(err)
	}

	hostAddr, err := ma.NewMultiaddr(*hostAddrStr)
	if err != nil {
		log.Fatal(err)
	}
	serviceApiAddr, err := ma.NewMultiaddr(*serviceApiAddrStr)
	if err != nil {
		log.Fatal(err)
	}
	serviceApiProxyAddr, err := ma.NewMultiaddr(*serviceApiProxyAddrStr)
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

	log.Debugf("repo: %v", *repo)
	log.Debugf("hostAddr: %v", *hostAddrStr)
	log.Debugf("serviceApiAddr: %v", *serviceApiAddrStr)
	log.Debugf("serviceApiProxyAddr: %v", *serviceApiProxyAddrStr)
	log.Debugf("apiAddr: %v", *apiAddrStr)
	log.Debugf("apiProxyAddr: %v", *apiProxyAddrStr)
	log.Debugf("debug: %v", *debug)

	ts, err := db.DefaultService(
		*repo,
		db.WithServiceHostAddr(hostAddr),
		db.WithServiceDebug(*debug))
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

	serviceServer, err := serviceapi.NewServer(context.Background(), ts, serviceapi.Config{
		Addr:      serviceApiAddr,
		ProxyAddr: serviceApiProxyAddr,
		Debug:     *debug,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer serviceServer.Close()

	fmt.Println("Welcome to Threads!")
	fmt.Println("Your peer ID is " + ts.Host().ID().String())

	log.Debug("threadsd started")

	select {}
}
