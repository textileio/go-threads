package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/improbable-eng/grpc-web/go/grpcweb"
	logging "github.com/ipfs/go-log"
	connmgr "github.com/libp2p/go-libp2p-connmgr"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/namsral/flag"
	"github.com/textileio/go-threads/api"
	pb "github.com/textileio/go-threads/api/pb"
	"github.com/textileio/go-threads/common"
	netapi "github.com/textileio/go-threads/net/api"
	netpb "github.com/textileio/go-threads/net/api/pb"
	"github.com/textileio/go-threads/util"
	"google.golang.org/grpc"
)

var log = logging.Logger("threadsd")

func main() {
	fs := flag.NewFlagSetWithEnvPrefix(os.Args[0], "THRDS", 0)

	repo := fs.String("repo", ".threads", "Repo location")
	hostAddrStr := fs.String("hostAddr", "/ip4/0.0.0.0/tcp/4006", "Libp2p host bind address")
	apiAddrStr := fs.String("apiAddr", "/ip4/127.0.0.1/tcp/6006", "gRPC API bind address")
	apiProxyAddrStr := fs.String("apiProxyAddr", "/ip4/127.0.0.1/tcp/6007", "gRPC API web proxy bind address")
	connLowWater := fs.Int("connLowWater", 100, "Low watermark of libp2p connections that'll be maintained")
	connHighWater := fs.Int("connHighWater", 400, "High watermark of libp2p connections that'll be maintained")
	connGracePeriod := fs.Duration("connGracePeriod", time.Second*20, "Duration a new opened connection is not subject to pruning")
	keepAliveInterval := fs.Duration("keepAliveInterval", time.Second*5, "Websocket keepalive interval (must be >= 1s)")
	enableNetPubsub := fs.Bool("enableNetPubsub", false, "Enables thread networking over libp2p pubsub")
	debug := fs.Bool("debug", false, "Enables debug logging")
	if err := fs.Parse(os.Args[1:]); err != nil {
		log.Fatal(err)
	}

	hostAddr, err := ma.NewMultiaddr(*hostAddrStr)
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

	if err := util.SetupDefaultLoggingConfig(*repo); err != nil {
		log.Fatal(err)
	}
	if *debug {
		if err := logging.SetLogLevel("threadsd", "debug"); err != nil {
			log.Fatal(err)
		}
	}

	log.Debugf("repo: %v", *repo)
	log.Debugf("hostAddr: %v", *hostAddrStr)
	log.Debugf("apiAddr: %v", *apiAddrStr)
	log.Debugf("apiProxyAddr: %v", *apiProxyAddrStr)
	log.Debugf("connLowWater: %v", *connLowWater)
	log.Debugf("connHighWater: %v", *connHighWater)
	log.Debugf("connGracePeriod: %v", *connGracePeriod)
	log.Debugf("keepAliveInterval: %v", *keepAliveInterval)
	log.Debugf("enableNetPubsub: %v", *enableNetPubsub)
	log.Debugf("debug: %v", *debug)

	n, err := common.DefaultNetwork(
		*repo,
		common.WithNetHostAddr(hostAddr),
		common.WithConnectionManager(connmgr.NewConnManager(*connLowWater, *connHighWater, *connGracePeriod)),
		common.WithNetPubSub(*enableNetPubsub),
		common.WithNetDebug(*debug))
	if err != nil {
		log.Fatal(err)
	}
	defer n.Close()
	n.Bootstrap(util.DefaultBoostrapPeers())

	service, err := api.NewService(n, api.Config{
		RepoPath: *repo,
		Debug:    *debug,
	})
	if err != nil {
		log.Fatal(err)
	}
	netService, err := netapi.NewService(n, netapi.Config{
		Debug: *debug,
	})
	if err != nil {
		log.Fatal(err)
	}

	target, err := util.TCPAddrFromMultiAddr(apiAddr)
	if err != nil {
		log.Fatal(err)
	}
	ptarget, err := util.TCPAddrFromMultiAddr(apiProxyAddr)
	if err != nil {
		log.Fatal(err)
	}

	server := grpc.NewServer()
	listener, err := net.Listen("tcp", target)
	if err != nil {
		log.Fatal(err)
	}
	go func() {
		pb.RegisterAPIServer(server, service)
		netpb.RegisterAPIServer(server, netService)
		if err := server.Serve(listener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			log.Fatalf("serve error: %v", err)
		}
	}()
	webrpc := grpcweb.WrapServer(
		server,
		grpcweb.WithOriginFunc(func(origin string) bool {
			return true
		}),
		grpcweb.WithWebsockets(true),
		grpcweb.WithWebsocketPingInterval(*keepAliveInterval),
		grpcweb.WithWebsocketOriginFunc(func(req *http.Request) bool {
			return true
		}))
	proxy := &http.Server{
		Addr: ptarget,
	}
	proxy.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if webrpc.IsGrpcWebRequest(r) ||
			webrpc.IsAcceptableGrpcCorsRequest(r) ||
			webrpc.IsGrpcWebSocketRequest(r) {
			webrpc.ServeHTTP(w, r)
		}
	})
	go func() {
		if err := proxy.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("proxy error: %v", err)
		}
	}()

	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := proxy.Shutdown(ctx); err != nil {
			log.Fatal(err)
		}
		server.GracefulStop()
		if err := n.Close(); err != nil {
			log.Fatal(err)
		}
	}()

	fmt.Println("Welcome to Threads!")
	fmt.Println("Your peer ID is " + n.Host().ID().String())

	log.Debug("threadsd started")

	select {}
}
