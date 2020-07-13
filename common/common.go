package common

import (
	"context"
	"os"
	"path/filepath"
	"time"

	ipfslite "github.com/hsanjuan/ipfs-lite"
	"github.com/ipfs/go-datastore"
	badger "github.com/ipfs/go-ds-badger"
	"github.com/libp2p/go-libp2p"
	connmgr "github.com/libp2p/go-libp2p-connmgr"
	cconnmgr "github.com/libp2p/go-libp2p-core/connmgr"
	host "github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	peerstore "github.com/libp2p/go-libp2p-core/peerstore"
	"github.com/libp2p/go-libp2p-kad-dht/dual"
	"github.com/libp2p/go-libp2p-peerstore/pstoreds"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/textileio/go-threads/core/app"
	"github.com/textileio/go-threads/logstore/lstoreds"
	"github.com/textileio/go-threads/net"
	util "github.com/textileio/go-threads/util"
	"google.golang.org/grpc"
)

const (
	defaultIpfsLitePath = "ipfslite"
	defaultLogstorePath = "logstore"
)

// DefaultNetwork is a boostrapable default Net with sane defaults.
type NetBoostrapper interface {
	app.Net
	GetIpfsLite() *ipfslite.Peer
	Bootstrap(addrs []peer.AddrInfo)
}

func DefaultNetwork(repoPath string, opts ...NetOption) (NetBoostrapper, error) {
	config := &NetConfig{}
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

	if config.ConnManager == nil {
		config.ConnManager = connmgr.NewConnManager(100, 400, time.Second*20)
	}

	ipfsLitePath := filepath.Join(repoPath, defaultIpfsLitePath)
	if err := os.MkdirAll(ipfsLitePath, os.ModePerm); err != nil {
		return nil, err
	}
	litestore, err := ipfslite.BadgerDatastore(ipfsLitePath)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	pstore, err := pstoreds.NewPeerstore(ctx, litestore, pstoreds.DefaultOpts())
	if err != nil {
		litestore.Close()
		cancel()
		return nil, err
	}
	priv := util.LoadKey(filepath.Join(ipfsLitePath, "key"))
	h, d, err := ipfslite.SetupLibp2p(
		ctx,
		priv,
		nil,
		[]ma.Multiaddr{config.HostAddr},
		litestore,
		libp2p.Peerstore(pstore),
		libp2p.ConnectionManager(config.ConnManager),
		libp2p.DisableRelay(),
	)
	if err != nil {
		cancel()
		litestore.Close()
		return nil, err
	}
	lite, err := ipfslite.New(ctx, litestore, h, d, nil)
	if err != nil {
		cancel()
		litestore.Close()
		return nil, err
	}

	// Build a logstore
	logstorePath := filepath.Join(repoPath, defaultLogstorePath)
	if err := os.MkdirAll(logstorePath, os.ModePerm); err != nil {
		cancel()
		return nil, err
	}
	logstore, err := badger.NewDatastore(logstorePath, &badger.DefaultOptions)
	if err != nil {
		cancel()
		litestore.Close()
		return nil, err
	}
	tstore, err := lstoreds.NewLogstore(ctx, logstore, lstoreds.DefaultOpts())
	if err != nil {
		cancel()
		if err := logstore.Close(); err != nil {
			return nil, err
		}
		litestore.Close()
		return nil, err
	}

	// Build a network
	api, err := net.NewNetwork(ctx, h, lite.BlockStore(), lite, tstore, net.Config{
		Debug:  config.Debug,
		PubSub: !config.PubSubDisabled,
	}, config.GRPCOptions...)
	if err != nil {
		cancel()
		if err := logstore.Close(); err != nil {
			return nil, err
		}
		litestore.Close()
		return nil, err
	}

	return &netBoostrapper{
		cancel:    cancel,
		Net:       api,
		litepeer:  lite,
		pstore:    pstore,
		logstore:  logstore,
		litestore: litestore,
		host:      h,
		dht:       d,
	}, nil
}

type NetConfig struct {
	HostAddr       ma.Multiaddr
	ConnManager    cconnmgr.ConnManager
	Debug          bool
	GRPCOptions    []grpc.ServerOption
	PubSubDisabled bool
}

type NetOption func(c *NetConfig) error

func WithNetHostAddr(addr ma.Multiaddr) NetOption {
	return func(c *NetConfig) error {
		c.HostAddr = addr
		return nil
	}
}

func WithConnectionManager(cm cconnmgr.ConnManager) NetOption {
	return func(c *NetConfig) error {
		c.ConnManager = cm
		return nil
	}
}

func WithNetDebug(enabled bool) NetOption {
	return func(c *NetConfig) error {
		c.Debug = enabled
		return nil
	}
}

func WithNetGRPCOptions(opts ...grpc.ServerOption) NetOption {
	return func(c *NetConfig) error {
		c.GRPCOptions = opts
		return nil
	}
}

func WithPubSubDisabled() NetOption {
	return func(c *NetConfig) error {
		c.PubSubDisabled = true
		return nil
	}
}

type netBoostrapper struct {
	cancel context.CancelFunc
	app.Net
	litepeer  *ipfslite.Peer
	pstore    peerstore.Peerstore
	logstore  datastore.Datastore
	litestore datastore.Datastore
	host      host.Host
	dht       *dual.DHT
}

var _ NetBoostrapper = (*netBoostrapper)(nil)

func (tsb *netBoostrapper) Bootstrap(addrs []peer.AddrInfo) {
	tsb.litepeer.Bootstrap(addrs)
}

func (tsb *netBoostrapper) GetIpfsLite() *ipfslite.Peer {
	return tsb.litepeer
}

func (tsb *netBoostrapper) Close() error {
	if err := tsb.Net.Close(); err != nil {
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
	if err := tsb.litestore.Close(); err != nil {
		return err
	}
	return tsb.logstore.Close()
	// Logstore closed by network
}
