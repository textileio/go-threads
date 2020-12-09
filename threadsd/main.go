package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"time"

	"github.com/improbable-eng/grpc-web/go/grpcweb"
	logging "github.com/ipfs/go-log"
	connmgr "github.com/libp2p/go-libp2p-connmgr"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/namsral/flag"
	mongods "github.com/textileio/go-ds-mongo"
	"github.com/textileio/go-threads/api"
	pb "github.com/textileio/go-threads/api/pb"
	"github.com/textileio/go-threads/common"
	kt "github.com/textileio/go-threads/db/keytransform"
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
	mongoUri := fs.String("mongoUri", "", "MongoDB URI (if not provided, an embedded Badger datastore will be used)")
	mongoDatabase := fs.String("mongoDatabase", "", "MongoDB database name (required with mongoUri")
	badgerLowMem := fs.Bool("badgerLowMem", false, "Use Badger's low memory settings")
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

	var parsedMongoUri *url.URL
	if len(*mongoUri) != 0 {
		parsedMongoUri, err = url.Parse(*mongoUri)
		if err != nil {
			log.Fatalf("parsing mongoUri: %v", err)
		}
		if len(*mongoDatabase) == 0 {
			log.Fatal("mongoDatabase is required with mongoUri")
		}
	} else {
		log.Debugf("badgerLowMem: %v", *badgerLowMem)
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
	if parsedMongoUri != nil {
		log.Debugf("mongoUri: %v", parsedMongoUri.Redacted())
		log.Debugf("mongoDatabase: %v", *mongoDatabase)
	} else {
		log.Debugf("badgerLowMem: %v", *badgerLowMem)
	}
	log.Debugf("debug: %v", *debug)

	opts := []common.NetOption{
		common.WithNetHostAddr(hostAddr),
		common.WithConnectionManager(connmgr.NewConnManager(*connLowWater, *connHighWater, *connGracePeriod)),
		common.WithNetPubSub(*enableNetPubsub),
		common.WithNetDebug(*debug),
	}
	if parsedMongoUri != nil {
		opts = append(opts, common.WithNetMongoPersistence(*mongoUri, *mongoDatabase))
	} else {
		opts = append(opts, common.WithNetBadgerPersistence(*repo))
	}
	n, err := common.DefaultNetwork(opts...)
	if err != nil {
		log.Fatal(err)
	}
	defer n.Close()
	n.Bootstrap(util.DefaultBoostrapPeers())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var store kt.TxnDatastoreExtended
	if *mongoUri != "" {
		store, err = mongods.New(ctx, *mongoUri, *mongoDatabase, mongods.WithCollName("eventstore"))
	} else {
		store, err = util.NewBadgerDatastore(*repo, "eventstore", *badgerLowMem)
	}
	if err != nil {
		log.Fatal(err)
	}
	service, err := api.NewService(store, n, api.Config{
		Debug: *debug,
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

	fmt.Println("Welcome to Threads!")
	fmt.Println("Your peer ID is " + n.Host().ID().String())

	handleInterrupt(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := proxy.Shutdown(ctx); err != nil {
			log.Fatal(err)
		}
		server.GracefulStop()
		if err := n.Close(); err != nil {
			log.Fatal(err)
		}
	})
}

func handleInterrupt(stop func()) {
	quit := make(chan os.Signal)
	signal.Notify(quit, os.Interrupt)
	<-quit
	fmt.Println("Gracefully stopping... (press Ctrl+C again to force)")
	stop()
	os.Exit(1)
}
