package eventstore

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	ipfslite "github.com/hsanjuan/ipfs-lite"
	"github.com/ipfs/go-datastore"
	"github.com/libp2p/go-libp2p"
	connmgr "github.com/libp2p/go-libp2p-connmgr"
	host "github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	peerstore "github.com/libp2p/go-libp2p-peerstore"
	"github.com/libp2p/go-libp2p-peerstore/pstoreds"
	ma "github.com/multiformats/go-multiaddr"
	tserv "github.com/textileio/go-textile-core/threadservice"
	t "github.com/textileio/go-textile-threads"
	"github.com/textileio/go-textile-threads/tstoreds"
	util "github.com/textileio/go-textile-threads/util"
)

const (
	defaultIpfsLitePath  = "ipfslite"
	defaultListeningPort = 5002
)

// DefaultThreadService is a boostrapable default Threadservice with
// sane defaults.
type ThreadserviceBoostrapper interface {
	tserv.Threadservice
	Bootstrap(addrs []peer.AddrInfo)
}

func DefaultThreadservice(repoPath string, opts ...Option) (ThreadserviceBoostrapper, error) {
	config := &Config{ProxyPort: 5050}
	for _, opt := range opts {
		if err := opt(config); err != nil {
			return nil, err
		}
	}

	ipfsLitePath := filepath.Join(repoPath, defaultIpfsLitePath)
	if err := os.MkdirAll(ipfsLitePath, os.ModePerm); err != nil {
		return nil, err
	}
	ds, err := ipfslite.BadgerDatastore(ipfsLitePath)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	pstore, err := pstoreds.NewPeerstore(ctx, ds, pstoreds.DefaultOpts())
	if err != nil {
		ds.Close()
		cancel()
		return nil, err
	}
	listen, err := ma.NewMultiaddr(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", config.ListenPort))
	if err != nil {
		ds.Close()
		cancel()
		return nil, err
	}
	priv := util.LoadKey(filepath.Join(ipfsLitePath, "key"))
	h, dht, err := ipfslite.SetupLibp2p(
		ctx,
		priv,
		nil,
		[]ma.Multiaddr{listen},
		libp2p.ConnectionManager(connmgr.NewConnManager(100, 400, time.Minute)),
		libp2p.Peerstore(pstore),
	)
	if err != nil {
		cancel()
		ds.Close()
		return nil, err
	}

	lite, err := ipfslite.New(ctx, ds, h, dht, nil)
	if err != nil {
		cancel()
		ds.Close()
		return nil, err
	}

	// Build a threadstore
	tstore, err := tstoreds.NewThreadstore(ctx, ds, tstoreds.DefaultOpts())
	if err != nil {
		cancel()
		ds.Close()
		return nil, err
	}

	// Build a threadservice
	api, err := t.NewThreads(ctx, h, lite.BlockStore(), lite, tstore, t.Config{
		Debug:     config.Debug,
		ProxyAddr: fmt.Sprintf("0.0.0.0:%d", config.ProxyPort),
	})
	if err != nil {
		cancel()
		ds.Close()
		return nil, err
	}

	return &tservBoostrapper{
		cancel:        cancel,
		Threadservice: api,
		litepeer:      lite,
		pstore:        pstore,
		ds:            ds,
		host:          h,
		dht:           dht,
	}, nil
}

type Config struct {
	ListenPort int
	ProxyPort  int
	Debug      bool
}

type Option func(c *Config) error

func ListenPort(port int) Option {
	return func(c *Config) error {
		c.ListenPort = port
		return nil
	}
}

func ProxyPort(port int) Option {
	return func(c *Config) error {
		c.ProxyPort = port
		return nil
	}
}

func Debug(enabled bool) Option {
	return func(c *Config) error {
		c.Debug = enabled
		return nil
	}
}

type tservBoostrapper struct {
	cancel context.CancelFunc
	tserv.Threadservice
	litepeer *ipfslite.Peer
	pstore   peerstore.Peerstore
	ds       datastore.Datastore
	host     host.Host
	dht      *dht.IpfsDHT
}

var _ ThreadserviceBoostrapper = (*tservBoostrapper)(nil)

func (tsb *tservBoostrapper) Bootstrap(addrs []peer.AddrInfo) {
	tsb.litepeer.Bootstrap(addrs)
}

func (tsb *tservBoostrapper) Close() error {
	if err := tsb.Threadservice.Close(); err != nil {
		return err
	}
	tsb.cancel()
	if err := tsb.dht.Close(); err != nil {
		return err
	}
	if err := tsb.host.Close(); err != nil {
		return err
	}
	if err := tsb.pstore.Close(); err != nil {
		return err
	}
	return tsb.ds.Close()
	// Threadstore closed by threadservice
}
