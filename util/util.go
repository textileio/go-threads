package util

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/alecthomas/jsonschema"
	ipfslite "github.com/hsanjuan/ipfs-lite"
	logging "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	swarm "github.com/libp2p/go-libp2p-swarm"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/phayes/freeport"
	core "github.com/textileio/go-threads/core/db"
	"github.com/tidwall/sjson"
	"go.uber.org/zap/zapcore"
)

var (
	bootstrapPeers = []string{
		"/ip4/104.210.43.77/tcp/4001/p2p/QmR69wtWUMm1TWnmuD4JqC1TWLZcc8iR2KrTenfZZbiztd",       // us-west-1 hub ipfs host
		"/ip4/104.210.43.77/tcp/4006/p2p/12D3KooWQEtCBXMKjVas6Ph1pUHG2T4Lc9j1KvnAipojP2xcKU7n", // us-west-1 hub threads host
	}
)

// CanDial returns whether or not the address is dialable.
func CanDial(addr ma.Multiaddr, s *swarm.Swarm) bool {
	parts := strings.Split(addr.String(), "/"+ma.ProtocolWithCode(ma.P_P2P).Name)
	addr, _ = ma.NewMultiaddr(parts[0])
	tr := s.TransportForDialing(addr)
	return tr != nil && tr.CanDial(addr)
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
	ipfspeers := ipfslite.DefaultBootstrapPeers()
	textilepeers, err := ParseBootstrapPeers(bootstrapPeers)
	if err != nil {
		panic("coudn't parse default bootstrap peers")
	}
	return append(textilepeers, ipfspeers...)
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

func FreeLocalAddr() ma.Multiaddr {
	hostPort, err := freeport.GetFreePort()
	if err != nil {
		return nil
	}
	return MustParseAddr(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", hostPort))
}

func SchemaFromInstance(i interface{}, expandedStruct bool) *jsonschema.Schema {
	reflector := jsonschema.Reflector{ExpandedStruct: expandedStruct}
	return reflector.Reflect(i)
}

func SchemaFromSchemaString(s string) *jsonschema.Schema {
	schemaBytes := []byte(s)
	schema := &jsonschema.Schema{}
	if err := json.Unmarshal(schemaBytes, schema); err != nil {
		panic(err)
	}
	return schema
}

func JSONFromInstance(i interface{}) []byte {
	JSON, err := json.Marshal(i)
	if err != nil {
		panic(err)
	}
	return JSON
}

func InstanceFromJSON(b []byte, i interface{}) {
	if err := json.Unmarshal(b, i); err != nil {
		panic(err)
	}
}

func SetJSONProperty(name string, value interface{}, json []byte) []byte {
	updated, err := sjson.SetBytes(json, name, value)
	if err != nil {
		panic(err)
	}
	return updated
}

func SetJSONID(id core.InstanceID, json []byte) []byte {
	return SetJSONProperty("_id", id.String(), json)
}
