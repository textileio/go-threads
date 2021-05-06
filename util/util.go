package util

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"sort"

	"github.com/alecthomas/jsonschema"
	"github.com/dgraph-io/badger/options"
	ipfslite "github.com/hsanjuan/ipfs-lite"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"
	mbase "github.com/multiformats/go-multibase"
	"github.com/phayes/freeport"
	badger "github.com/textileio/go-ds-badger"
	core "github.com/textileio/go-threads/core/db"
	kt "github.com/textileio/go-threads/db/keytransform"
	"github.com/tidwall/sjson"
	"go.uber.org/zap/zapcore"
)

var (
	bootstrapPeers = []string{
		// hub-production ipfs-hub-0 host
		"/ip4/40.76.153.74/tcp/4001/p2p/QmR69wtWUMm1TWnmuD4JqC1TWLZcc8iR2KrTenfZZbiztd",
		// hub-production hub-0 threads host
		"/ip4/52.186.99.239/tcp/4006/p2p/12D3KooWQEtCBXMKjVas6Ph1pUHG2T4Lc9j1KvnAipojP2xcKU7n",
		// go-ipfs 0.7.0 default bootstrap peers
		"/dnsaddr/bootstrap.libp2p.io/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN",
		"/dnsaddr/bootstrap.libp2p.io/p2p/QmQCU2EcMqAqQPR2i9bChDtGNJchTbq5TbXJJ16u19uLTa",
		"/dnsaddr/bootstrap.libp2p.io/p2p/QmbLHAnMoJPWSCR5Zhtx6BHJX9KiKNN6tpvbUcqanj75Nb",
		"/dnsaddr/bootstrap.libp2p.io/p2p/QmcZf59bWwK5XFi76CZX8cbJ4BhTzzA3gU1ZjYZcYW3dwt",
		"/ip4/104.131.131.82/tcp/4001/p2p/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ",
		"/ip4/104.131.131.82/udp/4001/quic/p2p/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ",
	}
)

// NewBadgerDatastore returns a badger based datastore.
func NewBadgerDatastore(dirPath, name string, lowMem bool) (kt.TxnDatastoreExtended, error) {
	path := filepath.Join(dirPath, name)
	if err := os.MkdirAll(path, os.ModePerm); err != nil {
		return nil, err
	}
	opts := badger.DefaultOptions
	if lowMem {
		opts.TableLoadingMode = options.FileIO
	}
	return badger.NewDatastore(path, &opts)
}

// SetupDefaultLoggingConfig sets up a standard logging configuration.
func SetupDefaultLoggingConfig(repoPath string) error {
	folder := filepath.Join(repoPath, "log")
	if err := os.MkdirAll(folder, os.ModePerm); err != nil {
		return err
	}
	c := logging.Config{
		Format: logging.ColorizedOutput,
		Stderr: true,
		File:   filepath.Join(folder, "threads.log"),
		Level:  logging.LevelError,
	}
	logging.SetupLogging(c)

	return nil
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

func GenerateRandomBytes(n int) []byte {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}
	return b
}

func MakeToken(n int) string {
	bs := GenerateRandomBytes(n)
	encoded, err := mbase.Encode(mbase.Base32, bs)
	if err != nil {
		panic(err)
	}
	return encoded
}

type LogHead struct {
	LogID peer.ID
	Head  cid.Cid
}

func ComputeHeadsEdge(hs []LogHead) uint64 {
	// sort heads for deterministic edge computation
	sort.Slice(hs, func(i, j int) bool {
		if hs[i].LogID == hs[j].LogID {
			return hs[i].Head.KeyString() < hs[j].Head.KeyString()
		}
		return hs[i].LogID < hs[j].LogID
	})
	hasher := fnv.New64a()
	for i := 0; i < len(hs); i++ {
		_, _ = hasher.Write([]byte(hs[i].LogID))
		_, _ = hasher.Write(hs[i].Head.Bytes())
	}
	return hasher.Sum64()
}

type PeerAddr struct {
	PeerID peer.ID
	Addr   ma.Multiaddr
}

func ComputeAddrsEdge(as []PeerAddr) uint64 {
	// sort peer addresses for deterministic edge computation
	sort.Slice(as, func(i, j int) bool {
		if as[i].PeerID == as[j].PeerID {
			return bytes.Compare(as[i].Addr.Bytes(), as[j].Addr.Bytes()) < 0
		}
		return as[i].PeerID < as[j].PeerID
	})
	hasher := fnv.New64a()
	for i := 0; i < len(as); i++ {
		_, _ = hasher.Write([]byte(as[i].PeerID))
		_, _ = hasher.Write(as[i].Addr.Bytes())
	}
	return hasher.Sum64()
}
