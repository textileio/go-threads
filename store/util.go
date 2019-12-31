package store

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
	coreservice "github.com/textileio/go-threads/core/service"
	"github.com/textileio/go-threads/logstore/lstoreds"
	"github.com/textileio/go-threads/service"
	util "github.com/textileio/go-threads/util"
)

const (
	defaultIpfsLitePath = "ipfslite"
)

// DefaultService is a boostrapable default Service with
// sane defaults.
type ServiceBoostrapper interface {
	coreservice.Service
	GetIpfsLite() *ipfslite.Peer
	Bootstrap(addrs []peer.AddrInfo)
}

func DefaultService(repoPath string, opts ...ServiceOption) (ServiceBoostrapper, error) {
	config := &ServiceConfig{}
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

	// Build a logstore
	tstore, err := lstoreds.NewLogstore(ctx, ds, lstoreds.DefaultOpts())
	if err != nil {
		cancel()
		ds.Close()
		return nil, err
	}

	// Build a service
	api, err := service.NewService(ctx, h, lite.BlockStore(), lite, tstore, service.Config{
		Debug:     config.Debug,
		ProxyAddr: config.HostProxyAddr,
	})
	if err != nil {
		cancel()
		ds.Close()
		return nil, err
	}

	return &servBoostrapper{
		cancel:   cancel,
		Service:  api,
		litepeer: lite,
		pstore:   pstore,
		ds:       ds,
		host:     h,
		dht:      d,
	}, nil
}

type ServiceConfig struct {
	HostAddr      ma.Multiaddr
	HostProxyAddr ma.Multiaddr
	Debug         bool
}

type ServiceOption func(c *ServiceConfig) error

func WithServiceHostAddr(addr ma.Multiaddr) ServiceOption {
	return func(c *ServiceConfig) error {
		c.HostAddr = addr
		return nil
	}
}

func WithServiceHostProxyAddr(addr ma.Multiaddr) ServiceOption {
	return func(c *ServiceConfig) error {
		c.HostProxyAddr = addr
		return nil
	}
}

func WithServiceDebug(enabled bool) ServiceOption {
	return func(c *ServiceConfig) error {
		c.Debug = enabled
		return nil
	}
}

type servBoostrapper struct {
	cancel context.CancelFunc
	coreservice.Service
	litepeer *ipfslite.Peer
	pstore   peerstore.Peerstore
	ds       datastore.Datastore
	host     host.Host
	dht      *dht.IpfsDHT
}

var _ ServiceBoostrapper = (*servBoostrapper)(nil)

func (tsb *servBoostrapper) Bootstrap(addrs []peer.AddrInfo) {
	tsb.litepeer.Bootstrap(addrs)
}

func (tsb *servBoostrapper) GetIpfsLite() *ipfslite.Peer {
	return tsb.litepeer
}

func (tsb *servBoostrapper) Close() error {
	if err := tsb.Service.Close(); err != nil {
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
	// Logstore closed by service
}
