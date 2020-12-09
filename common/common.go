package common

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	ipfslite "github.com/hsanjuan/ipfs-lite"
	ds "github.com/ipfs/go-datastore"
	"github.com/libp2p/go-libp2p"
	connmgr "github.com/libp2p/go-libp2p-connmgr"
	cconnmgr "github.com/libp2p/go-libp2p-core/connmgr"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-peerstore/pstoreds"
	ma "github.com/multiformats/go-multiaddr"
	mongods "github.com/textileio/go-ds-mongo"
	"github.com/textileio/go-threads/core/app"
	core "github.com/textileio/go-threads/core/logstore"
	"github.com/textileio/go-threads/logstore/lstoreds"
	"github.com/textileio/go-threads/logstore/lstorehybrid"
	"github.com/textileio/go-threads/logstore/lstoremem"
	"github.com/textileio/go-threads/net"
	"github.com/textileio/go-threads/util"
	"google.golang.org/grpc"
)

// DefaultNetwork is a boostrapable default Net with sane defaults.
type NetBoostrapper interface {
	app.Net
	GetIpfsLite() *ipfslite.Peer
	Bootstrap(addrs []peer.AddrInfo)
}

func DefaultNetwork(opts ...NetOption) (NetBoostrapper, error) {
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

	ctx, cancel := context.WithCancel(context.Background())
	fin.Add(util.NewContextCloser(cancel))

	litestore, err := persistentStore(ctx, config, "ipfslite", fin)
	if err != nil {
		return nil, fin.Cleanup(err)
	}
	fin.Add(litestore)

	pstore, err := pstoreds.NewPeerstore(ctx, litestore, pstoreds.DefaultOpts())
	if err != nil {
		return nil, fin.Cleanup(err)
	}
	fin.Add(pstore)

	hostKey, err := getIPFSHostKey(config, litestore)
	if err != nil {
		return nil, fin.Cleanup(err)
	}

	h, d, err := ipfslite.SetupLibp2p(
		ctx,
		hostKey,
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

	tstore, err := buildLogstore(ctx, config, fin)
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

func buildLogstore(ctx context.Context, config NetConfig, fin *util.Finalizer) (core.Logstore, error) {
	switch config.LSType {
	case LogstoreInMemory:
		return lstoremem.NewLogstore(), nil

	case LogstoreHybrid:
		pls, err := persistentLogstore(ctx, config, fin)
		if err != nil {
			return nil, err
		}
		mls := lstoremem.NewLogstore()
		return lstorehybrid.NewLogstore(pls, mls)

	case LogstorePersistent:
		return persistentLogstore(ctx, config, fin)

	default:
		return nil, fmt.Errorf("unsupported logstore type: %s", config.LSType)
	}
}

func persistentLogstore(ctx context.Context, config NetConfig, fin *util.Finalizer) (core.Logstore, error) {
	pds, err := persistentStore(ctx, config, "logstore", fin)
	if err != nil {
		return nil, err
	}
	return lstoreds.NewLogstore(ctx, pds, lstoreds.DefaultOpts())
}

func persistentStore(ctx context.Context, config NetConfig, name string, fin *util.Finalizer) (ds.Batching, error) {
	if len(config.MongoUri) != 0 {
		return mongoStore(ctx, config.MongoUri, config.MongoDB, name, fin)
	} else {
		return badgerStore(filepath.Join(config.BadgerRepoPath, name), fin)
	}
}

func badgerStore(repoPath string, fin *util.Finalizer) (ds.Batching, error) {
	if err := os.MkdirAll(repoPath, os.ModePerm); err != nil {
		return nil, err
	}

	dstore, err := ipfslite.BadgerDatastore(repoPath)
	if err != nil {
		return nil, err
	}
	fin.Add(dstore)

	return dstore, nil
}

func mongoStore(ctx context.Context, uri, db, collection string, fin *util.Finalizer) (ds.Batching, error) {
	dstore, err := mongods.New(ctx, uri, db, mongods.WithCollName(collection))
	if err != nil {
		return nil, err
	}
	fin.Add(dstore)

	return dstore, nil
}

func getIPFSHostKey(config NetConfig, store ds.Datastore) (crypto.PrivKey, error) {
	if len(config.MongoUri) != 0 {
		k := ds.NewKey("key")
		bytes, err := store.Get(k)
		if errors.Is(err, ds.ErrNotFound) {
			key, bytes, err := newIPFSHostKey()
			if err != nil {
				return nil, err
			}
			if err = store.Put(k, bytes); err != nil {
				return nil, err
			}
			return key, nil
		} else if err != nil {
			return nil, err
		}
		return crypto.UnmarshalPrivateKey(bytes)
	} else {
		// If a local datastore is used, the key is written to a file
		dir := filepath.Join(config.BadgerRepoPath, "ipfslite")
		pth := filepath.Join(dir, "key")
		_, err := os.Stat(pth)
		if os.IsNotExist(err) {
			key, bytes, err := newIPFSHostKey()
			if err != nil {
				return nil, err
			}
			if err := os.MkdirAll(dir, os.ModePerm); err != nil {
				return nil, err
			}
			if err = ioutil.WriteFile(pth, bytes, 0400); err != nil {
				return nil, err
			}
			return key, nil
		} else if err != nil {
			return nil, err
		} else {
			bytes, err := ioutil.ReadFile(pth)
			if err != nil {
				return nil, err
			}
			return crypto.UnmarshalPrivateKey(bytes)
		}
	}
}

func newIPFSHostKey() (crypto.PrivKey, []byte, error) {
	priv, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 0)
	if err != nil {
		return nil, nil, err
	}
	key, err := crypto.MarshalPrivateKey(priv)
	if err != nil {
		return nil, nil, err
	}
	return priv, key, nil
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

	if len(config.LSType) == 0 {
		config.LSType = LogstorePersistent
	}

	if len(config.MongoDB) == 0 {
		config.MongoDB = "threadnet"
	}

	return nil
}

type LogstoreType string

const (
	LogstoreInMemory   LogstoreType = "in-memory"
	LogstorePersistent LogstoreType = "persistent"
	LogstoreHybrid     LogstoreType = "hybrid"
)

type NetConfig struct {
	HostAddr          ma.Multiaddr
	ConnManager       cconnmgr.ConnManager
	GRPCServerOptions []grpc.ServerOption
	GRPCDialOptions   []grpc.DialOption
	LSType            LogstoreType
	BadgerRepoPath    string
	MongoUri          string
	MongoDB           string
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

func WithNetLogstore(lt LogstoreType) NetOption {
	return func(c *NetConfig) error {
		c.LSType = lt
		return nil
	}
}

func WithNetBadgerPersistence(repoPath string) NetOption {
	return func(c *NetConfig) error {
		c.BadgerRepoPath = repoPath
		return nil
	}
}

func WithNetMongoPersistence(uri, db string) NetOption {
	return func(c *NetConfig) error {
		c.MongoUri = uri
		c.MongoDB = db
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
