package foldersync

import (
	"context"
	"strings"

	ipfslite "github.com/hsanjuan/ipfs-lite"
	datastore "github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	"github.com/libp2p/go-libp2p-core/crypto"
	host "github.com/libp2p/go-libp2p-host"
	"github.com/mr-tron/base58"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/textileio/go-threads/core/thread"
)

func parseInviteLink(inviteLink string) (thread.ID, ma.Multiaddr, thread.Key) {
	addrRest := strings.Split(inviteLink, "?")
	addr, err := ma.NewMultiaddr(addrRest[0])
	if err != nil {
		panic("invalid invite link")
	}
	keyThreadID := strings.Split(addrRest[1], "&")
	keyBytes, err := base58.Decode(keyThreadID[0])
	if err != nil {
		panic("invalid key")
	}
	key, err := thread.KeyFromBytes(keyBytes)
	if err != nil {
		panic("invalid key")
	}
	threadID, err := thread.Decode(keyThreadID[1])
	if err != nil {
		panic("invalid thread id")
	}
	return threadID, addr, key
}

func createIPFSLite(h host.Host) (*ipfslite.Peer, func() error, error) {
	ds := dssync.MutexWrap(datastore.NewMapDatastore())
	priv, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 0)
	if err != nil {
		return nil, nil, err
	}
	listen, _ := ma.NewMultiaddr("/ip4/0.0.0.0/tcp/0")
	if err != nil {
		return nil, nil, err
	}
	ctx := context.Background()
	h1, dht1, err := ipfslite.SetupLibp2p(ctx, priv, nil, []ma.Multiaddr{listen})
	if err != nil {
		return nil, nil, err
	}
	p1, err := ipfslite.New(ctx, ds, h1, dht1, nil)
	if err != nil {
		return nil, nil, err
	}

	close := func() error {
		ctx.Done()
		if err := dht1.Close(); err != nil {
			return err
		}
		if err := h1.Close(); err != nil {
			return err
		}
		return ds.Close()
	}
	return p1, close, nil
}
