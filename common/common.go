package common

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	ipfslite "github.com/hsanjuan/ipfs-lite"
	"github.com/libp2p/go-libp2p"
	connmgr "github.com/libp2p/go-libp2p-connmgr"
	cconnmgr "github.com/libp2p/go-libp2p-core/connmgr"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-peerstore/pstoreds"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/textileio/go-threads/core/app"
	core "github.com/textileio/go-threads/core/logstore"
	"github.com/textileio/go-threads/logstore/lstoreds"
	"github.com/textileio/go-threads/logstore/lstorehybrid"
	"github.com/textileio/go-threads/logstore/lstoremem"
	"github.com/textileio/go-threads/net"
	"github.com/textileio/go-threads/util"
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
	var (
		config NetConfig
		fin    = util.NewFinalizer()
	)

	for _, opt := range opts {
		if err := opt(&config); err != nil {
			return nil, err
		}
	}

	if err := setDefaults(&config); err != nil {
		return nil, err
	}

	ipfsLitePath := filepath.Join(repoPath, defaultIpfsLitePath)
	if err := os.MkdirAll(ipfsLitePath, os.ModePerm); err != nil {
		return nil, err
	}

	litestore, err := ipfslite.BadgerDatastore(ipfsLitePath)
	if err != nil {
		return nil, err
	}
	fin.Add(litestore)

	ctx, cancel := context.WithCancel(context.Background())
	fin.Add(util.NewContextCloser(cancel))

	pstore, err := pstoreds.NewPeerstore(ctx, litestore, pstoreds.DefaultOpts())
	if err != nil {
		return nil, fin.Cleanup(err)
	}
	fin.Add(pstore)

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
		return nil, fin.Cleanup(err)
	}

	lite, err := ipfslite.New(ctx, litestore, h, d, nil)
	if err != nil {
		return nil, fin.Cleanup(err)
	}

	tstore, err := buildLogstore(ctx, config.LogstoreKind, repoPath, fin)
	if err != nil {
		return nil, fin.Cleanup(err)
	}

	// Build a network
	api, err := net.NewNetwork(ctx, h, lite.BlockStore(), lite, tstore, net.Config{
		Debug:  config.Debug,
		PubSub: config.PubSub,
	}, config.GRPCServerOptions, config.GRPCDialOptions)
	if err != nil {
		return nil, fin.Cleanup(err)
	}
	fin.Add(h, d, api)

	return &netBoostrapper{
		Net:       api,
		litepeer:  lite,
		finalizer: fin,
	}, nil
}

func buildLogstore(ctx context.Context, kind LogstoreKind, repoPath string, fin *util.Finalizer) (core.Logstore, error) {
	switch kind {
	case LogstoreInMemory:
		return lstoremem.NewLogstore(), nil

	case LogstoreHybrid:
		pls, err := persistentLogstore(ctx, repoPath, fin)
		if err != nil {
			return nil, err
		}
		mls := lstoremem.NewLogstore()
		return lstorehybrid.NewLogstore(pls, mls)

	case LogstorePersistent:
		return persistentLogstore(ctx, repoPath, fin)

	default:
		return nil, fmt.Errorf("unsupported kind of logstore: %s", kind)
	}
}

func persistentLogstore(ctx context.Context, repoPath string, fin *util.Finalizer) (core.Logstore, error) {
	logstorePath := filepath.Join(repoPath, defaultLogstorePath)
	if err := os.MkdirAll(logstorePath, os.ModePerm); err != nil {
		return nil, err
	}

	dstore, err := ipfslite.BadgerDatastore(logstorePath)
	if err != nil {
		return nil, err
	}
	fin.Add(dstore)

	return lstoreds.NewLogstore(ctx, dstore, lstoreds.DefaultOpts())
}

func setDefaults(config *NetConfig) error {
	if config.HostAddr == nil {
		addr, err := ma.NewMultiaddr("/ip4/0.0.0.0/tcp/0")
		if err != nil {
			return err
		}
		config.HostAddr = addr
	}

	if config.ConnManager == nil {
		config.ConnManager = connmgr.NewConnManager(100, 400, time.Second*20)
	}

	if len(config.LogstoreKind) == 0 {
		config.LogstoreKind = LogstorePersistent
	}

	return nil
}

type LogstoreKind string

const (
	LogstoreInMemory   LogstoreKind = "in-memory"
	LogstorePersistent LogstoreKind = "persistent"
	LogstoreHybrid     LogstoreKind = "hybrid"
)

type NetConfig struct {
	HostAddr          ma.Multiaddr
	ConnManager       cconnmgr.ConnManager
	GRPCServerOptions []grpc.ServerOption
	GRPCDialOptions   []grpc.DialOption
	LogstoreKind      LogstoreKind
	PubSub            bool
	Debug             bool
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

func WithNetGRPCServerOptions(opts ...grpc.ServerOption) NetOption {
	return func(c *NetConfig) error {
		c.GRPCServerOptions = opts
		return nil
	}
}

func WithNetGRPCDialOptions(opts ...grpc.DialOption) NetOption {
	return func(c *NetConfig) error {
		c.GRPCDialOptions = opts
		return nil
	}
}

func WithNetPubSub(enabled bool) NetOption {
	return func(c *NetConfig) error {
		c.PubSub = enabled
		return nil
	}
}

func WithNetLogstore(lk LogstoreKind) NetOption {
	return func(c *NetConfig) error {
		c.LogstoreKind = lk
		return nil
	}
}

type netBoostrapper struct {
	app.Net
	litepeer  *ipfslite.Peer
	finalizer *util.Finalizer
}

var _ NetBoostrapper = (*netBoostrapper)(nil)

func (tsb *netBoostrapper) Bootstrap(addrs []peer.AddrInfo) {
	tsb.litepeer.Bootstrap(addrs)
}

func (tsb *netBoostrapper) GetIpfsLite() *ipfslite.Peer {
	return tsb.litepeer
}

func (tsb *netBoostrapper) Close() error {
	return tsb.finalizer.Cleanup(nil)
}
