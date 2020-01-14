package util

import (
	"crypto/rand"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	logging "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"
	swarm "github.com/libp2p/go-libp2p-swarm"
	"github.com/mr-tron/base58"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/textileio/go-threads/core/service"
	"github.com/textileio/go-threads/core/thread"
	sym "github.com/textileio/go-threads/crypto/symmetric"
	"go.uber.org/zap/zapcore"
)

var (
	bootstrapPeers = []string{
		"/ip4/104.210.43.77/tcp/4001/ipfs/12D3KooWSdGmRz5JQidqrtmiPGVHkStXpbSAMnbCcW8abq6zuiDP", // us-west
		"/ip4/20.39.232.27/tcp/4001/ipfs/12D3KooWLnUv9MWuRM6uHirRPBM4NwRj54n4gNNnBtiFiwPiv3Up",  // eu-west
		"/ip4/34.87.103.105/tcp/4001/ipfs/12D3KooWA5z2C3z1PNKi36Bw1MxZhBD8nv7UbB7YQP6WcSWYNwRQ", // as-southeast
	}
)

// CreateThread creates a new set of keys.
func CreateThread(s service.Service, id thread.ID) (info thread.Info, err error) {
	info.ID = id
	info.FollowKey, err = sym.CreateKey()
	if err != nil {
		return
	}
	info.ReadKey, err = sym.CreateKey()
	if err != nil {
		return
	}
	err = s.Store().AddThread(info)
	return
}

// CreateLog creates a new log with the given peer as host.
func CreateLog(host peer.ID) (info thread.LogInfo, err error) {
	sk, pk, err := crypto.GenerateEd25519Key(rand.Reader)
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

// GetLog returns the log with the given thread and log id.
func GetLog(s service.Service, id thread.ID, lid peer.ID) (info thread.LogInfo, err error) {
	info, err = s.Store().LogInfo(id, lid)
	if err != nil {
		return
	}
	if info.PubKey != nil {
		return
	}
	return info, fmt.Errorf("log %s doesn't exist for thread %s", lid, id)
}

// GetOwnLoad returns the log owned by the host under the given thread.
func GetOwnLog(s service.Service, id thread.ID) (info thread.LogInfo, err error) {
	logs, err := s.Store().LogsWithKeys(id)
	if err != nil {
		return
	}
	for _, lid := range logs {
		sk, err := s.Store().PrivKey(id, lid)
		if err != nil {
			return info, err
		}
		if sk != nil {
			return s.Store().LogInfo(id, lid)
		}
	}
	return info, nil
}

// GetOrCreateOwnLoad returns the log owned by the host under the given thread.
// If no log exists, a new one is created under the given thread.
func GetOrCreateOwnLog(s service.Service, id thread.ID) (info thread.LogInfo, err error) {
	info, err = GetOwnLog(s, id)
	if err != nil {
		return info, err
	}
	if info.PubKey != nil {
		return
	}
	info, err = CreateLog(s.Host().ID())
	if err != nil {
		return
	}
	err = s.Store().AddLog(id, info)
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
	pid, err = peer.Decode(p2p)
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

// SetupDefaultLoggingConfig sets up a standard logging configuration.
func SetupDefaultLoggingConfig(repoPath string) {
	_ = os.Setenv("GOLOG_LOG_FMT", "color")
	_ = os.Setenv("GOLOG_FILE", filepath.Join(repoPath, "log", "threads.log"))
	logging.SetupLogging()
	logging.SetAllLoggers(logging.LevelError)
}

// SetLogLevels sets levels for the given systems.
func SetLogLevels(systems map[string]logging.LogLevel) error {
	for sys, level := range systems {
		l := zapcore.Level(level)
		if sys == "*" {
			for _, s := range logging.GetSubsystems() {
				if err := logging.SetLogLevel(s, l.CapitalString()); err != nil {
					return err
				}
			}
		}
		if err := logging.SetLogLevel(sys, l.CapitalString()); err != nil {
			return err
		}
	}
	return nil
}

func LoadKey(pth string) crypto.PrivKey {
	var priv crypto.PrivKey
	_, err := os.Stat(pth)
	if os.IsNotExist(err) {
		priv, _, err = crypto.GenerateKeyPair(crypto.Ed25519, 0)
		if err != nil {
			panic(err)
		}
		key, err := crypto.MarshalPrivateKey(priv)
		if err != nil {
			panic(err)
		}
		if err = ioutil.WriteFile(pth, key, 0400); err != nil {
			panic(err)
		}
	} else if err != nil {
		panic(err)
	} else {
		key, err := ioutil.ReadFile(pth)
		if err != nil {
			panic(err)
		}
		priv, err = crypto.UnmarshalPrivateKey(key)
		if err != nil {
			panic(err)
		}
	}
	return priv
}

func DefaultBoostrapPeers() []peer.AddrInfo {
	ais, err := ParseBootstrapPeers(bootstrapPeers)
	if err != nil {
		panic("coudn't parse default bootstrap peers")
	}
	return ais
}

func ParseBootstrapPeers(addrs []string) ([]peer.AddrInfo, error) {
	maddrs := make([]ma.Multiaddr, len(addrs))
	for i, addr := range addrs {
		var err error
		maddrs[i], err = ma.NewMultiaddr(addr)
		if err != nil {
			return nil, err
		}
	}
	return peer.AddrInfosFromP2pAddrs(maddrs...)
}

func TCPAddrFromMultiAddr(maddr ma.Multiaddr) (addr string, err error) {
	if maddr == nil {
		err = fmt.Errorf("invalid address")
		return
	}
	ip4, err := maddr.ValueForProtocol(ma.P_IP4)
	if err != nil {
		return
	}
	tcp, err := maddr.ValueForProtocol(ma.P_TCP)
	if err != nil {
		return
	}
	return fmt.Sprintf("%s:%s", ip4, tcp), nil
}

func MustParseAddr(str string) ma.Multiaddr {
	addr, err := ma.NewMultiaddr(str)
	if err != nil {
		panic(err)
	}
	return addr
}
