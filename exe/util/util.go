package util

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	ipfslite "github.com/hsanjuan/ipfs-lite"
	"github.com/ipfs/go-datastore"
	logging "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p"
	connmgr "github.com/libp2p/go-libp2p-connmgr"
	"github.com/libp2p/go-libp2p-core/host"
	kaddht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p-peerstore/pstoreds"
	"github.com/mitchellh/go-homedir"
	ma "github.com/multiformats/go-multiaddr"
	tserv "github.com/textileio/go-textile-core/threadservice"
	t "github.com/textileio/go-textile-threads"
	"github.com/textileio/go-textile-threads/tstoreds"
	"github.com/textileio/go-textile-threads/util"
)

// BootstrapPeers to bootstrap from.
var bootstrapPeers = []string{
	"/ip4/104.210.43.77/tcp/4001/ipfs/12D3KooWSdGmRz5JQidqrtmiPGVHkStXpbSAMnbCcW8abq6zuiDP", // us-west
	"/ip4/20.39.232.27/tcp/4001/ipfs/12D3KooWLnUv9MWuRM6uHirRPBM4NwRj54n4gNNnBtiFiwPiv3Up",  // eu-west
	"/ip4/34.87.103.105/tcp/4001/ipfs/12D3KooWA5z2C3z1PNKi36Bw1MxZhBD8nv7UbB7YQP6WcSWYNwRQ", // as-southeast
}

// Build an instance of threads.
func Build(repo string, port int, proxyAddr string, debug bool) (
	ctx context.Context,
	cancel context.CancelFunc,
	ds datastore.Batching,
	h host.Host,
	dht *kaddht.IpfsDHT,
	api tserv.Threadservice) {

	repop, err := homedir.Expand(repo)
	if err != nil {
		panic(err)
	}
	if err = os.MkdirAll(repop, os.ModePerm); err != nil {
		panic(err)
	}

	ctx, cancel = context.WithCancel(context.Background())

	// Build an IPFS-Lite peer
	priv := util.LoadKey(filepath.Join(repop, "key"))

	ds, err = ipfslite.BadgerDatastore(repop)
	if err != nil {
		panic(err)
	}
	pstore, err := pstoreds.NewPeerstore(ctx, ds, pstoreds.DefaultOpts())
	if err != nil {
		panic(err)
	}

	listen, err := ma.NewMultiaddr(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", port))
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
	util.SetupDefaultLoggingConfig(repop)
	api, err = t.NewThreads(ctx, h, lite.BlockStore(), lite, tstore, t.Config{
		ProxyAddr: proxyAddr,
		Debug:     debug,
	})
	if err != nil {
		panic(err)
	}

	// Bootstrap to textile peers
	if err = logging.SetLogLevel("ipfslite", "debug"); err != nil {
		panic(err)
	}
	boots, err := util.ParseBootstrapPeers(bootstrapPeers)
	if err != nil {
		panic(err)
	}
	lite.Bootstrap(boots)

	return
}
