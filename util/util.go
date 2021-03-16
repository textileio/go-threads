package util

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dgraph-io/badger/options"
	ipfslite "github.com/hsanjuan/ipfs-lite"
	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/oklog/ulid/v2"
	"github.com/phayes/freeport"
	badger "github.com/textileio/go-ds-badger"
	kt "github.com/textileio/go-threads/db/keytransform"
	"go.uber.org/zap/zapcore"
)

var (
	bootstrapPeers = []string{
		// hub-production ipfs-hub-0 host
		"/ip4/40.76.153.74/tcp/4001/p2p/QmR69wtWUMm1TWnmuD4JqC1TWLZcc8iR2KrTenfZZbiztd",
		// hub-production hub-0 threads host
		"/ip4/52.186.99.239/tcp/4006/p2p/12D3KooWQEtCBXMKjVas6Ph1pUHG2T4Lc9j1KvnAipojP2xcKU7n",
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
func SetupDefaultLoggingConfig(file string) error {
	if file != "" {
		if err := os.MkdirAll(filepath.Dir(file), os.ModePerm); err != nil {
			return err
		}
	}
	c := logging.Config{
		Format: logging.ColorizedOutput,
		Stderr: true,
		File:   file,
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

func GenerateRandomBytes(n int) []byte {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}
	return b
}

func MakeToken() string {
	return strings.ToLower(ulid.MustNew(ulid.Now(), rand.Reader).String())
}
