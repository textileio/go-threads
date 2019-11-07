package util

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"time"

	ipfslite "github.com/hsanjuan/ipfs-lite"
	"github.com/ipfs/go-datastore"
	logging "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p"
	connmgr "github.com/libp2p/go-libp2p-connmgr"
	ic "github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	kaddht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p-peerstore/pstoreds"
	"github.com/mitchellh/go-homedir"
	ma "github.com/multiformats/go-multiaddr"
	tserv "github.com/textileio/go-textile-core/threadservice"
	t "github.com/textileio/go-textile-threads"
	"github.com/textileio/go-textile-threads/tstoreds"
	"gopkg.in/natefinch/lumberjack.v2"
)

// BootstrapPeers to bootstrap from.
var bootstrapPeers = []string{
	"/ip4/104.210.43.77/tcp/4001/ipfs/12D3KooWSdGmRz5JQidqrtmiPGVHkStXpbSAMnbCcW8abq6zuiDP", // us-west
	"/ip4/20.39.232.27/tcp/4001/ipfs/12D3KooWLnUv9MWuRM6uHirRPBM4NwRj54n4gNNnBtiFiwPiv3Up",  // eu-west
	"/ip4/34.87.103.105/tcp/4001/ipfs/12D3KooWA5z2C3z1PNKi36Bw1MxZhBD8nv7UbB7YQP6WcSWYNwRQ", // as-southeast
}

// Build an instance of threads.
func Build(debug bool) (
	ctx context.Context,
	cancel context.CancelFunc,
	ds datastore.Batching,
	h host.Host,
	dht *kaddht.IpfsDHT,
	api tserv.Threadservice) {
	repo := flag.String("repo", ".threads", "repo location")
	port := flag.Int("port", 4006, "host port")
	flag.Parse()

	repop, err := homedir.Expand(*repo)
	if err != nil {
		panic(err)
	}
	if err = os.MkdirAll(repop, os.ModePerm); err != nil {
		panic(err)
	}

	ctx, cancel = context.WithCancel(context.Background())

	// Build an IPFS-Lite peer
	priv := LoadKey(filepath.Join(repop, "key"))

	ds, err = ipfslite.BadgerDatastore(repop)
	if err != nil {
		panic(err)
	}
	pstore, err := pstoreds.NewPeerstore(ctx, ds, pstoreds.DefaultOpts())
	if err != nil {
		panic(err)
	}

	listen, err := ma.NewMultiaddr(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", *port))
	if err != nil {
		panic(err)
	}

	h, dht, err = ipfslite.SetupLibp2p(
		ctx,
		priv,
		nil,
		[]ma.Multiaddr{listen},
		libp2p.ConnectionManager(connmgr.NewConnManager(100, 400, time.Minute)),
		libp2p.Peerstore(pstore),
	)
	if err != nil {
		panic(err)
	}

	lite, err := ipfslite.New(ctx, ds, h, dht, nil)
	if err != nil {
		panic(err)
	}

	// Build a threadstore
	tstore, err := tstoreds.NewThreadstore(ctx, ds, tstoreds.DefaultOpts())
	if err != nil {
		panic(err)
	}

	// Build a threadservice
	writer := &lumberjack.Logger{
		Filename:   path.Join(repop, "log"),
		MaxSize:    10, // megabytes
		MaxBackups: 3,
		MaxAge:     30, // days
	}
	api, err = t.NewThreads(ctx, h, lite.BlockStore(), lite, tstore, t.Options{
		LogWriter: writer,
		Debug:     debug,
	})
	if err != nil {
		panic(err)
	}

	// Bootstrap to textile peers
	if err = logging.SetLogLevel("ipfslite", "debug"); err != nil {
		panic(err)
	}
	boots, err := ParseBootstrapPeers(bootstrapPeers)
	if err != nil {
		panic(err)
	}
	lite.Bootstrap(boots)

	return
}

// LoadKey at path.
func LoadKey(pth string) ic.PrivKey {
	var priv ic.PrivKey
	_, err := os.Stat(pth)
	if os.IsNotExist(err) {
		priv, _, err = ic.GenerateKeyPair(ic.Ed25519, 0)
		if err != nil {
			panic(err)
		}
		key, err := ic.MarshalPrivateKey(priv)
		if err != nil {
			panic(err)
		}
		if err = ioutil.WriteFile(pth, key, 0400); err != nil {
			panic(err)
		}
	} else if err != nil {
		panic(err)
	} else {
		key, err := ioutil.ReadFile(pth)
		if err != nil {
			panic(err)
		}
		priv, err = ic.UnmarshalPrivateKey(key)
		if err != nil {
			panic(err)
		}
	}
	return priv
}

// ParseBootstrapPeers returns address info from a list of string addresses.
func ParseBootstrapPeers(addrs []string) ([]peer.AddrInfo, error) {
	maddrs := make([]ma.Multiaddr, len(addrs))
	for i, addr := range addrs {
		var err error
		maddrs[i], err = ma.NewMultiaddr(addr)
		if err != nil {
			return nil, err
		}
	}
	return peer.AddrInfosFromP2pAddrs(maddrs...)
}
