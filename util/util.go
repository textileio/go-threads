package util

import (
	"crypto/rand"
	"fmt"

	ic "github.com/libp2p/go-libp2p-core/crypto"
	peer "github.com/libp2p/go-libp2p-peer"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/textileio/go-textile-core/crypto/symmetric"
	"github.com/textileio/go-textile-core/thread"
)

// CreateLog creates a new log with the given peer as host.
func CreateLog(host peer.ID) (info thread.LogInfo, err error) {
	sk, pk, err := ic.GenerateEd25519Key(rand.Reader)
	if err != nil {
		return
	}
	id, err := peer.IDFromPublicKey(pk)
	if err != nil {
		return
	}
	rk, err := symmetric.CreateKey()
	if err != nil {
		return
	}
	fk, err := symmetric.CreateKey()
	if err != nil {
		return
	}
	addr, err := ma.NewMultiaddr(fmt.Sprintf("/p2p/%s", host.String()))
	if err != nil {
		return
	}
	return thread.LogInfo{
		ID:        id,
		PubKey:    pk,
		PrivKey:   sk,
		ReadKey:   rk.Bytes(),
		FollowKey: fk.Bytes(),
		Addrs:     []ma.Multiaddr{addr},
	}, nil
}
