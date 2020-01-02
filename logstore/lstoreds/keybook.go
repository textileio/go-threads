package lstoreds

import (
	"fmt"

	ds "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	core "github.com/textileio/go-threads/core/logstore"
	"github.com/textileio/go-threads/core/thread"
	sym "github.com/textileio/go-threads/crypto/symmetric"
	"github.com/whyrusleeping/base32"
)

type dsKeyBook struct {
	ds ds.Datastore
}

// Public and private keys are stored under the following db key pattern:
// /threads/keys/<b32 thread id no padding>/<b32 log id no padding>/(pub|priv)
// Follow and read keys are stored under the following db key pattern:
// /threads/keys/<b32 thread id no padding>/(follow|read)
var (
	kbBase       = ds.NewKey("/thread/keys")
	pubSuffix    = ds.NewKey("/pub")
	privSuffix   = ds.NewKey("/priv")
	readSuffix   = ds.NewKey("/read")
	followSuffix = ds.NewKey("/follow")
)

var _ core.KeyBook = (*dsKeyBook)(nil)

// NewKeyBook returns a new key book for storing public and private keys
// of (thread.ID, peer.ID) pairs with durable guarantees by store.
func NewKeyBook(store ds.Datastore) (core.KeyBook, error) {
	return &dsKeyBook{ds: store}, nil
}

// PubKey returns the public key of (thread.ID, peer.ID). The implementation
// assumes the key is in the store with the exception that peer.ID is an
// Identity multihash. If the public key can't be resolved, nil is returned.
func (kb *dsKeyBook) PubKey(t thread.ID, p peer.ID) (crypto.PubKey, error) {
	key := dsLogKey(t, p, kbBase).Child(pubSuffix)

	v, err := kb.ds.Get(key)
	if err == ds.ErrNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("error when getting key %s from store: %v", key, err)
	}
	pk, err := crypto.UnmarshalPublicKey(v)
	if err != nil {
		return nil, fmt.Errorf("store backed public key %s can't be unmarshaled: %w", key, err)
	}

	return pk, nil
}

// AddPubKey adds the public key of peer.ID which should match accordingly.
func (kb *dsKeyBook) AddPubKey(t thread.ID, p peer.ID, pk crypto.PubKey) error {
	if pk == nil {
		return fmt.Errorf("public key is nil")
	}

	if !p.MatchesPublicKey(pk) {
		return fmt.Errorf("log ID doesn't provided match public key")
	}
	val, err := pk.Bytes()
	if err != nil {
		return fmt.Errorf("error when getting bytes from public key: %w", err)
	}
	key := dsLogKey(t, p, kbBase).Child(pubSuffix)
	if kb.ds.Put(key, val) != nil {
		return fmt.Errorf("error when putting public key in store: %w", err)
	}
	return nil
}

// PrivKey returns the private key of (thread.ID, peer.ID). If not private key
// is stored, returns nil.
func (kb *dsKeyBook) PrivKey(t thread.ID, p peer.ID) (crypto.PrivKey, error) {
	key := dsLogKey(t, p, kbBase).Child(privSuffix)
	v, err := kb.ds.Get(key)
	if err == ds.ErrNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("error when getting private key for %s", key)
	}
	sk, err := crypto.UnmarshalPrivateKey(v)
	if err != nil {
		return nil, fmt.Errorf("error when unmarshaling private key of %v", key)
	}
	return sk, nil
}

// AddPrivKey adds the private key of peer.ID which should match accordingly.
func (kb *dsKeyBook) AddPrivKey(t thread.ID, p peer.ID, sk crypto.PrivKey) error {
	if sk == nil {
		return fmt.Errorf("private key is nil")
	}
	if !p.MatchesPrivateKey(sk) {
		return fmt.Errorf("peer ID doesn't match with private key")
	}
	skb, err := sk.Bytes()
	if err != nil {
		return fmt.Errorf("error when getting private key bytes: %w", err)
	}
	key := dsLogKey(t, p, kbBase).Child(privSuffix)
	if err = kb.ds.Put(key, skb); err != nil {
		return fmt.Errorf("error when putting key %v in datastore: %w", key, err)
	}
	return nil
}

// ReadKey returns the read-key associated with thread.ID thread.
// In case it doesn't exist, it will return nil.
func (kb *dsKeyBook) ReadKey(t thread.ID) (*sym.Key, error) {
	key := dsThreadKey(t, kbBase).Child(readSuffix)
	v, err := kb.ds.Get(key)
	if err == ds.ErrNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("error when getting read-key from datastore: %v", err)
	}
	return sym.NewKey(v)
}

// AddReadKey adds a read-key for a peer.ID
func (kb *dsKeyBook) AddReadKey(t thread.ID, rk *sym.Key) error {
	if rk == nil {
		return fmt.Errorf("read-key is nil")
	}
	key := dsThreadKey(t, kbBase).Child(readSuffix)
	if err := kb.ds.Put(key, rk.Bytes()); err != nil {
		return fmt.Errorf("error when adding read-key to datastore: %w", err)
	}
	return nil
}

// FollowKey returns the follow-key associated with thread.ID service.
// In case it doesn't exist, it will return nil.
func (kb *dsKeyBook) FollowKey(t thread.ID) (*sym.Key, error) {
	key := dsThreadKey(t, kbBase).Child(followSuffix)

	v, err := kb.ds.Get(key)
	if err == ds.ErrNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("error when getting follow-key from datastore: %v", err)
	}
	return sym.NewKey(v)
}

// AddFollowKey adds a follow-key for a peer.ID
func (kb *dsKeyBook) AddFollowKey(t thread.ID, fk *sym.Key) error {
	if fk == nil {
		return fmt.Errorf("follow-key is nil")
	}
	key := dsThreadKey(t, kbBase).Child(followSuffix)
	if err := kb.ds.Put(key, fk.Bytes()); err != nil {
		return fmt.Errorf("error when adding follow-key to datastore: %w", err)
	}
	return nil
}

// LogsWithKeys returns a list of log IDs for a thread.
func (kb *dsKeyBook) LogsWithKeys(t thread.ID) (peer.IDSlice, error) {
	ids, err := uniqueLogIds(kb.ds, kbBase.ChildString(base32.RawStdEncoding.EncodeToString(t.Bytes())),
		func(result query.Result) string {
			return ds.RawKey(result.Key).Parent().Name()
		})
	if err != nil {
		return nil, fmt.Errorf("error while retrieving logs with addresses: %v", err)
	}
	return ids, nil
}

// ThreadsFromKeys returns a list of threads referenced in the book.
func (kb *dsKeyBook) ThreadsFromKeys() (thread.IDSlice, error) {
	ids, err := uniqueThreadIds(kb.ds, kbBase, func(result query.Result) string {
		return ds.RawKey(result.Key).Parent().Parent().Name()
	})
	if err != nil {
		return nil, fmt.Errorf("error while retrieving threads from keys: %v", err)
	}
	return ids, nil
}
