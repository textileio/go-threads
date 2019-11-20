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
	"github.com/libp2p/go-libp2p-core/peer"
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

func DefaultThreadservice(listenPort int, repoPath string, debug bool) (ThreadserviceBoostrapper, error) {
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
	listen, err := ma.NewMultiaddr(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", listenPort))
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
		ds.Close()
		cancel()
		return nil, err
	}

	lite, err := ipfslite.New(ctx, ds, h, dht, nil)
	if err != nil {
		ds.Close()
		cancel()
		return nil, err
	}

	// Build a threadstore
	tstore, err := tstoreds.NewThreadstore(ctx, ds, tstoreds.DefaultOpts())
	if err != nil {
		ds.Close()
		cancel()
		return nil, err
	}

	// Build a threadservice
	api, err := t.NewThreads(ctx, h, lite.BlockStore(), lite, tstore, t.Options{
		Debug: debug,
	})
	if err != nil {
		ds.Close()
		cancel()
		return nil, err
	}

	return &tservBoostraper{
		cancel:        cancel,
		Threadservice: api,
		litepeer:      lite,
		ds:            ds,
	}, nil
}

type tservBoostraper struct {
	cancel context.CancelFunc
	tserv.Threadservice
	litepeer *ipfslite.Peer
	ds       datastore.Datastore
}

var _ ThreadserviceBoostrapper = (*tservBoostraper)(nil)

func (tsb *tservBoostraper) Bootstrap(addrs []peer.AddrInfo) {
	tsb.litepeer.Bootstrap(addrs)
}

func (tsb *tservBoostraper) Close() error {
	tsb.cancel()
	if err := tsb.ds.Close(); err != nil {
		return err
	}
	return tsb.Threadservice.Close()
}
