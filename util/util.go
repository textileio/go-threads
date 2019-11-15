package util

import (
	"crypto/rand"
	"strings"

	ic "github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"
	swarm "github.com/libp2p/go-libp2p-swarm"
	"github.com/mr-tron/base58"
	ma "github.com/multiformats/go-multiaddr"
	sym "github.com/textileio/go-textile-core/crypto/symmetric"
	"github.com/textileio/go-textile-core/thread"
	tserv "github.com/textileio/go-textile-core/threadservice"
)

// CreateThread creates a new set of keys.
func CreateThread(t tserv.Threadservice, id thread.ID) (info thread.Info, err error) {
	info.ID = id
	info.FollowKey, err = sym.CreateKey()
	if err != nil {
		return
	}
	info.ReadKey, err = sym.CreateKey()
	if err != nil {
		return
	}
	err = t.Store().AddThread(info)
	return
}

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
	pro := ma.ProtocolWithCode(ma.P_P2P).Name
	addr, err := ma.NewMultiaddr("/" + pro + "/" + host.String())
	if err != nil {
		return
	}
	return thread.LogInfo{
		ID:      id,
		PubKey:  pk,
		PrivKey: sk,
		Addrs:   []ma.Multiaddr{addr},
	}, nil
}

// GetOrCreateLog returns the log with the given thread and log id.
// If no log exists, a new one is created under the given thread.
func GetOrCreateLog(t tserv.Threadservice, id thread.ID, lid peer.ID) (info thread.LogInfo, err error) {
	info, err = t.Store().LogInfo(id, lid)
	if err != nil {
		return
	}
	if info.PubKey != nil {
		return
	}
	info, err = CreateLog(t.Host().ID())
	if err != nil {
		return
	}
	err = t.Store().AddLog(id, info)
	return
}

// GetOwnLoad returns the log owned by the host under the given thread.
func GetOwnLog(t tserv.Threadservice, id thread.ID) (info thread.LogInfo, err error) {
	logs, err := t.Store().LogsWithKeys(id)
	if err != nil {
		return
	}
	for _, lid := range logs {
		sk, err := t.Store().PrivKey(id, lid)
		if err != nil {
			return info, err
		}
		if sk != nil {
			return t.Store().LogInfo(id, lid)
		}
	}
	return info, nil
}

// GetOrCreateOwnLoad returns the log owned by the host under the given thread.
// If no log exists, a new one is created under the given thread.
func GetOrCreateOwnLog(t tserv.Threadservice, id thread.ID) (info thread.LogInfo, err error) {
	info, err = GetOwnLog(t, id)
	if err != nil {
		return info, err
	}
	if info.PubKey != nil {
		return
	}
	info, err = CreateLog(t.Host().ID())
	if err != nil {
		return
	}
	err = t.Store().AddLog(id, info)
	return info, err
}

// AddPeerFromAddress parses the given address and adds the dialable component
// to the peerstore.
// If a dht is provided and the address does not contain a dialable component,
// it will be queried for peer info.
func AddPeerFromAddress(addrStr string, pstore peerstore.Peerstore) (pid peer.ID, err error) {
	addr, err := ma.NewMultiaddr(addrStr)
	if err != nil {
		return
	}
	p2p, err := addr.ValueForProtocol(ma.P_P2P)
	if err != nil {
		return
	}
	pid, err = peer.IDB58Decode(p2p)
	if err != nil {
		return
	}
	dialable, err := GetDialable(addr)
	if err == nil {
		pstore.AddAddr(pid, dialable, peerstore.PermanentAddrTTL)
	}

	return pid, nil
}

// GetDialable returns the portion of an address suitable for storage in a peerstore.
func GetDialable(addr ma.Multiaddr) (ma.Multiaddr, error) {
	parts := strings.Split(addr.String(), "/"+ma.ProtocolWithCode(ma.P_P2P).Name)
	return ma.NewMultiaddr(parts[0])
}

// CanDial returns whether or not the address is dialable.
func CanDial(addr ma.Multiaddr, s *swarm.Swarm) bool {
	parts := strings.Split(addr.String(), "/"+ma.ProtocolWithCode(ma.P_P2P).Name)
	addr, _ = ma.NewMultiaddr(parts[0])
	tr := s.TransportForDialing(addr)
	return tr != nil && tr.CanDial(addr)
}

// DecodeKey from a string into a symmetric key.
func DecodeKey(k string) (*sym.Key, error) {
	b, err := base58.Decode(k)
	if err != nil {
		return nil, err
	}
	return sym.NewKey(b)
}

// PadArgs returns args with new length pad.
func PadArgs(args []string, len int) []string {
	padded := make([]string, len)
	copy(padded, args)
	return padded
}
