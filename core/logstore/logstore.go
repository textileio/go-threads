package logstore

import (
	"context"
	"fmt"
	"time"

	"github.com/ipfs/go-cid"
	ic "github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/textileio/go-threads/core/service"
	sym "github.com/textileio/go-threads/crypto/symmetric"
)

// ErrNotFound is and error used to indicate an item is not found.
var ErrNotFound = fmt.Errorf("item not found")

// Logstore stores log keys, addresses, heads and thread meta data.
type Logstore interface {
	Close() error

	ThreadMetadata
	KeyBook
	AddrBook
	HeadBook

	// Threads returns all threads in the store.
	Threads() (service.IDSlice, error)

	// AddThread adds a thread.
	AddThread(service.Info) error

	// ThreadInfo returns info about a thread.
	ThreadInfo(service.ID) (service.Info, error)

	// AddLog adds a log to a thread.
	AddLog(service.ID, service.LogInfo) error

	// LogInfo returns info about a log.
	LogInfo(service.ID, peer.ID) (service.LogInfo, error)
}

// ThreadMetadata stores local thread metadata like name.
type ThreadMetadata interface {
	// GetInt64 retrieves a string value under key.
	GetInt64(t service.ID, key string) (*int64, error)

	// PutInt64 stores an int value under key.
	PutInt64(t service.ID, key string, val int64) error

	// GetString retrieves an int value under key.
	GetString(t service.ID, key string) (*string, error)

	// PutString stores a string value under key.
	PutString(t service.ID, key string, val string) error

	// GetBytes retrieves a byte value under key.
	GetBytes(t service.ID, key string) (*[]byte, error)

	// PutBytes stores a byte value under key.
	PutBytes(t service.ID, key string, val []byte) error
}

// KeyBook stores log keys.
type KeyBook interface {
	// PubKey retrieves the public key of a log.
	PubKey(service.ID, peer.ID) (ic.PubKey, error)

	// AddPubKey adds a public key under a log.
	AddPubKey(service.ID, peer.ID, ic.PubKey) error

	// PrivKey retrieves the private key of a log.
	PrivKey(service.ID, peer.ID) (ic.PrivKey, error)

	// AddPrivKey adds a private key under a log.
	AddPrivKey(service.ID, peer.ID, ic.PrivKey) error

	// ReadKey retrieves the read key of a log.
	ReadKey(service.ID) (*sym.Key, error)

	// AddReadKey adds a read key under a log.
	AddReadKey(service.ID, *sym.Key) error

	// FollowKey retrieves the follow key of a log.
	FollowKey(service.ID) (*sym.Key, error)

	// AddFollowKey adds a follow key under a log.
	AddFollowKey(service.ID, *sym.Key) error

	// LogsWithKeys returns a list of log IDs for a service.
	LogsWithKeys(service.ID) (peer.IDSlice, error)

	// ThreadsFromKeys returns a list of threads referenced in the book.
	ThreadsFromKeys() (service.IDSlice, error)
}

// AddrBook stores log addresses.
type AddrBook interface {
	// AddAddr adds an address under a log with a given TTL.
	AddAddr(service.ID, peer.ID, ma.Multiaddr, time.Duration) error

	// AddAddrs adds addresses under a log with a given TTL.
	AddAddrs(service.ID, peer.ID, []ma.Multiaddr, time.Duration) error

	// SetAddr sets a log's address with a given TTL.
	SetAddr(service.ID, peer.ID, ma.Multiaddr, time.Duration) error

	// SetAddrs sets a log's addresses with a given TTL.
	SetAddrs(service.ID, peer.ID, []ma.Multiaddr, time.Duration) error

	// UpdateAddrs updates the TTL of a log address.
	UpdateAddrs(t service.ID, id peer.ID, oldTTL time.Duration, newTTL time.Duration) error

	// Addrs returns all addresses for a log.
	Addrs(service.ID, peer.ID) ([]ma.Multiaddr, error)

	// AddrStream returns a channel that delivers address changes for a log.
	AddrStream(context.Context, service.ID, peer.ID) (<-chan ma.Multiaddr, error)

	// ClearAddrs deletes all addresses for a log.
	ClearAddrs(service.ID, peer.ID) error

	// LogsWithAddrs returns a list of log IDs for a thread.
	LogsWithAddrs(service.ID) (peer.IDSlice, error)

	// ThreadsFromAddrs returns a list of threads referenced in the book.
	ThreadsFromAddrs() (service.IDSlice, error)
}

// HeadBook stores log heads.
type HeadBook interface {
	// AddHead stores cid in a log's head.
	AddHead(service.ID, peer.ID, cid.Cid) error

	// AddHeads stores cids in a log's head.
	AddHeads(service.ID, peer.ID, []cid.Cid) error

	// SetHead sets a log's head as cid.
	SetHead(service.ID, peer.ID, cid.Cid) error

	// SetHeads sets a log's head as cids.
	SetHeads(service.ID, peer.ID, []cid.Cid) error

	// Heads retrieves head values for a log.
	Heads(service.ID, peer.ID) ([]cid.Cid, error)

	// ClearHeads deletes the head entry for a log.
	ClearHeads(service.ID, peer.ID) error
}
