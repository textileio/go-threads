package logstore

import (
	"context"
	"fmt"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/textileio/go-threads/core/thread"
	sym "github.com/textileio/go-threads/crypto/symmetric"
)

// ErrThreadExists indicates a thread already exists.
var ErrThreadExists = fmt.Errorf("thread already exists")

// ErrThreadNotFound indicates a requested thread was not found.
var ErrThreadNotFound = fmt.Errorf("thread not found")

// ErrLogNotFound indicates a requested log was not found.
var ErrLogNotFound = fmt.Errorf("log not found")

// ErrLogExists indicates a requested log already exists.
var ErrLogExists = fmt.Errorf("log already exists")

// Logstore stores log keys, addresses, heads and thread meta data.
type Logstore interface {
	Close() error

	ThreadMetadata
	KeyBook
	AddrBook
	HeadBook

	// Threads returns all threads in the store.
	Threads() (thread.IDSlice, error)

	// AddThread adds a thread.
	AddThread(thread.Info) error

	// GetThread returns info about a thread.
	GetThread(thread.ID) (thread.Info, error)

	// DeleteThread deletes a thread.
	DeleteThread(thread.ID) error

	// AddLog adds a log to a thread.
	AddLog(thread.ID, thread.LogInfo) error

	// GetLog returns info about a log.
	GetLog(thread.ID, peer.ID) (thread.LogInfo, error)

	// GetManagedLogs returns info about locally managed logs.
	GetManagedLogs(thread.ID) ([]thread.LogInfo, error)

	// DeleteLog deletes a log.
	DeleteLog(thread.ID, peer.ID) error
}

// ThreadMetadata stores local thread metadata like name.
type ThreadMetadata interface {
	// GetInt64 retrieves a string value under key.
	GetInt64(t thread.ID, key string) (*int64, error)

	// PutInt64 stores an int value under key.
	PutInt64(t thread.ID, key string, val int64) error

	// GetString retrieves a string value under key.
	GetString(t thread.ID, key string) (*string, error)

	// PutString stores a string value under key.
	PutString(t thread.ID, key string, val string) error

	// GetBool retrieves a boolean value under key.
	GetBool(t thread.ID, key string) (*bool, error)

	// PutBool stores a boolean value under key.
	PutBool(t thread.ID, key string, val bool) error

	// GetBytes retrieves a byte value under key.
	GetBytes(t thread.ID, key string) (*[]byte, error)

	// PutBytes stores a byte value under key.
	PutBytes(t thread.ID, key string, val []byte) error

	// ClearMetadata clears all metadata under a thread.
	ClearMetadata(t thread.ID) error
}

// KeyBook stores log keys.
type KeyBook interface {
	// PubKey retrieves the public key of a log.
	PubKey(thread.ID, peer.ID) (crypto.PubKey, error)

	// AddPubKey adds a public key under a log.
	AddPubKey(thread.ID, peer.ID, crypto.PubKey) error

	// PrivKey retrieves the private key of a log.
	PrivKey(thread.ID, peer.ID) (crypto.PrivKey, error)

	// AddPrivKey adds a private key under a log.
	AddPrivKey(thread.ID, peer.ID, crypto.PrivKey) error

	// ReadKey retrieves the read key of a thread.
	ReadKey(thread.ID) (*sym.Key, error)

	// AddReadKey adds a read key under a thread.
	AddReadKey(thread.ID, *sym.Key) error

	// ServiceKey retrieves the service key of a thread.
	ServiceKey(thread.ID) (*sym.Key, error)

	// AddServiceKey adds a service key under a thread.
	AddServiceKey(thread.ID, *sym.Key) error

	// ClearKeys deletes all keys under a thread.
	ClearKeys(thread.ID) error

	// ClearLogKeys deletes all keys under a log.
	ClearLogKeys(thread.ID, peer.ID) error

	// LogsWithKeys returns a list of log IDs for a service.
	LogsWithKeys(thread.ID) (peer.IDSlice, error)

	// ThreadsFromKeys returns a list of threads referenced in the book.
	ThreadsFromKeys() (thread.IDSlice, error)
}

// AddrBook stores log addresses.
type AddrBook interface {
	// AddAddr adds an address under a log with a given TTL.
	AddAddr(thread.ID, peer.ID, ma.Multiaddr, time.Duration) error

	// AddAddrs adds addresses under a log with a given TTL.
	AddAddrs(thread.ID, peer.ID, []ma.Multiaddr, time.Duration) error

	// SetAddr sets a log's address with a given TTL.
	SetAddr(thread.ID, peer.ID, ma.Multiaddr, time.Duration) error

	// SetAddrs sets a log's addresses with a given TTL.
	SetAddrs(thread.ID, peer.ID, []ma.Multiaddr, time.Duration) error

	// UpdateAddrs updates the TTL of a log address.
	UpdateAddrs(t thread.ID, id peer.ID, oldTTL time.Duration, newTTL time.Duration) error

	// Addrs returns all addresses for a log.
	Addrs(thread.ID, peer.ID) ([]ma.Multiaddr, error)

	// AddrStream returns a channel that delivers address changes for a log.
	AddrStream(context.Context, thread.ID, peer.ID) (<-chan ma.Multiaddr, error)

	// ClearAddrs deletes all addresses for a log.
	ClearAddrs(thread.ID, peer.ID) error

	// LogsWithAddrs returns a list of log IDs for a thread.
	LogsWithAddrs(thread.ID) (peer.IDSlice, error)

	// ThreadsFromAddrs returns a list of threads referenced in the book.
	ThreadsFromAddrs() (thread.IDSlice, error)
}

// HeadBook stores log heads.
type HeadBook interface {
	// AddHead stores cid in a log's head.
	AddHead(thread.ID, peer.ID, cid.Cid) error

	// AddHeads stores cids in a log's head.
	AddHeads(thread.ID, peer.ID, []cid.Cid) error

	// SetHead sets a log's head as cid.
	SetHead(thread.ID, peer.ID, cid.Cid) error

	// SetHeads sets a log's head as cids.
	SetHeads(thread.ID, peer.ID, []cid.Cid) error

	// Heads retrieves head values for a log.
	Heads(thread.ID, peer.ID) ([]cid.Cid, error)

	// ClearHeads deletes the head entry for a log.
	ClearHeads(thread.ID, peer.ID) error
}
