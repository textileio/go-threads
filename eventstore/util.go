package eventstore

import (
	"context"
	"os"
	"path/filepath"
	"time"

	ipfslite "github.com/hsanjuan/ipfs-lite"
	"github.com/ipfs/go-datastore"
	"github.com/libp2p/go-libp2p"
	connmgr "github.com/libp2p/go-libp2p-connmgr"
	host "github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p-peerstore/pstoreds"
	ma "github.com/multiformats/go-multiaddr"
	tserv "github.com/textileio/go-textile-core/threadservice"
	t "github.com/textileio/go-threads"
	"github.com/textileio/go-threads/tstoreds"
	util "github.com/textileio/go-threads/util"
)

const (
	defaultIpfsLitePath = "ipfslite"
)

// DefaultThreadService is a boostrapable default Threadservice with
// sane defaults.
type ThreadserviceBoostrapper interface {
	tserv.Threadservice
	GetIpfsLite() *ipfslite.Peer
	Bootstrap(addrs []peer.AddrInfo)
}

func DefaultThreadservice(repoPath string, opts ...Option) (ThreadserviceBoostrapper, error) {
	config := &Config{}
	for _, opt := range opts {
		if err := opt(config); err != nil {
			return nil, err
		}
	}

	if config.HostAddr == nil {
		addr, err := ma.NewMultiaddr("/ip4/0.0.0.0/tcp/0")
		if err != nil {
			return nil, err
		}
		config.HostAddr = addr
	}
	if config.HostProxyAddr == nil {
		addr, err := ma.NewMultiaddr("/ip4/0.0.0.0/tcp/0")
		if err != nil {
			return nil, err
		}
		config.HostProxyAddr = addr
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
	priv := util.LoadKey(filepath.Join(ipfsLitePath, "key"))
	h, d, err := ipfslite.SetupLibp2p(
		ctx,
		priv,
		nil,
		[]ma.Multiaddr{config.HostAddr},
		libp2p.ConnectionManager(connmgr.NewConnManager(100, 400, time.Minute)),
		libp2p.Peerstore(pstore),
	)
	if err != nil {
		cancel()
		ds.Close()
		return nil, err
	}

	lite, err := ipfslite.New(ctx, ds, h, d, nil)
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
		ProxyAddr: config.HostProxyAddr,
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
		dht:           d,
	}, nil
}

type Config struct {
	HostAddr      ma.Multiaddr
	HostProxyAddr ma.Multiaddr
	Debug         bool
}

type Option func(c *Config) error

func HostAddr(addr ma.Multiaddr) Option {
	return func(c *Config) error {
		c.HostAddr = addr
		return nil
	}
}

func HostProxyAddr(addr ma.Multiaddr) Option {
	return func(c *Config) error {
		c.HostProxyAddr = addr
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

func (tsb *tservBoostrapper) GetIpfsLite() *ipfslite.Peer {
	return tsb.litepeer
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
