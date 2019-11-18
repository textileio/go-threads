package eventstore

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	ipfslite "github.com/hsanjuan/ipfs-lite"
	ds "github.com/ipfs/go-datastore"
	badger "github.com/ipfs/go-ds-badger"
	"github.com/libp2p/go-libp2p"
	connmgr "github.com/libp2p/go-libp2p-connmgr"
	ic "github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-peerstore/pstoreds"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/textileio/go-textile-core/threadservice"
	t "github.com/textileio/go-textile-threads"
	core "github.com/textileio/go-textile-core/store"
	"github.com/textileio/go-textile-threads/jsonpatcher"
	"github.com/textileio/go-textile-threads/tstoreds"
)

const (
	defaultDatastorePath = "/datastore"
	defaultIpfsLitePath  = "/ipfslite"
	defaultListeningPort = 5002
)

var (
	bootstrapPeers = []string{
		"/ip4/104.210.43.77/tcp/4001/ipfs/12D3KooWSdGmRz5JQidqrtmiPGVHkStXpbSAMnbCcW8abq6zuiDP", // us-west
		"/ip4/20.39.232.27/tcp/4001/ipfs/12D3KooWLnUv9MWuRM6uHirRPBM4NwRj54n4gNNnBtiFiwPiv3Up",  // eu-west
		"/ip4/34.87.103.105/tcp/4001/ipfs/12D3KooWA5z2C3z1PNKi36Bw1MxZhBD8nv7UbB7YQP6WcSWYNwRQ", // as-southeast
	}
)

// StoreOption takes a StoreConfig and modifies it
type StoreOption func(*StoreConfig) error

// StoreConfig has configuration parameters for a store
type StoreConfig struct {
	ListenPort    int
	RepoPath      string
	Datastore     ds.Datastore
	Threadservice threadservice.Threadservice
	EventCodec    core.EventCodec
	Debug         bool
}

func newDefaultEventCodec() core.EventCodec {
	return jsonpatcher.New()
}

func newDefaultDatastore(repoPath string) (ds.Datastore, error) {
	path := filepath.Join(repoPath, defaultDatastorePath)
	if err := os.MkdirAll(path, os.ModePerm); err != nil {
		return nil, err
	}
	return badger.NewDatastore(path, &badger.DefaultOptions)
}

func newDefaultThreadservice(ctx context.Context, listenPort int, repoPath string, debug bool) (threadservice.Threadservice, error) {
	ipfsLitePath := filepath.Join(repoPath, defaultIpfsLitePath)
	if err := os.MkdirAll(ipfsLitePath, os.ModePerm); err != nil {
		return nil, err
	}
	ds, err := ipfslite.BadgerDatastore(ipfsLitePath)
	if err != nil {
		return nil, err
	}
	go func() {
		<-ctx.Done()
		ds.Close()
	}()

	pstore, err := pstoreds.NewPeerstore(ctx, ds, pstoreds.DefaultOpts())
	if err != nil {
		return nil, err
	}
	listen, err := ma.NewMultiaddr(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", listenPort))
	if err != nil {
		return nil, err
	}
	priv := loadKey(filepath.Join(ipfsLitePath, "key"))
	h, dht, err := ipfslite.SetupLibp2p(
		ctx,
		priv,
		nil,
		[]ma.Multiaddr{listen},
		libp2p.ConnectionManager(connmgr.NewConnManager(100, 400, time.Minute)),
		libp2p.Peerstore(pstore),
	)
	if err != nil {
		return nil, err
	}

	lite, err := ipfslite.New(ctx, ds, h, dht, nil)
	if err != nil {
		return nil, err
	}

	// Build a threadstore
	tstore, err := tstoreds.NewThreadstore(ctx, ds, tstoreds.DefaultOpts())
	if err != nil {
		return nil, err
	}

	// Build a threadservice
	api, err := t.NewThreads(ctx, h, lite.BlockStore(), lite, tstore, t.Options{
		Debug: debug,
	})
	if err != nil {
		return nil, err
	}
	boots, err := parseBootstrapPeers(bootstrapPeers)
	if err != nil {
		return nil, err
	}
	lite.Bootstrap(boots)
	return api, nil
}

func WithRepoPath(path string) StoreOption {
	return func(sc *StoreConfig) error {
		sc.RepoPath = path
		return nil
	}
}

// WithDebug indicate to output debug information
func WithDebug(enable bool) StoreOption {
	return func(sc *StoreConfig) error {
		sc.Debug = enable
		return nil
	}
}

// WithThreadservice indicate to use a particular Threadservice
// implementation for Thread syncing. This configuration takes
// priority over WithListenPort.
func WithThreadservice(ts threadservice.Threadservice) StoreOption {
	return func(sc *StoreConfig) error {
		sc.Threadservice = ts
		return nil
	}
}

// WithListenPort indicate the port which the automatically
// instantiated Threadservice IPFSLite instance will listen
func WithListenPort(port int) StoreOption {
	return func(sc *StoreConfig) error {
		sc.ListenPort = port
		return nil
	}
}

// WithEventCodec configure to use ec as the EventCodec
// manager for transforming actions in events, and viceversa
func WithEventCodec(ec core.EventCodec) StoreOption {
	return func(sc *StoreConfig) error {
		sc.EventCodec = ec
		return nil
	}
}

func loadKey(pth string) ic.PrivKey {
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

func parseBootstrapPeers(addrs []string) ([]peer.AddrInfo, error) {
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
