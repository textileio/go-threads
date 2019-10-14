package tstoreds

import (
	"fmt"

	ds "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
	ic "github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/textileio/go-textile-core/thread"
	tstore "github.com/textileio/go-textile-core/threadstore"
	"github.com/whyrusleeping/base32"
)

type dsKeyBook struct {
	ds ds.Datastore
}

// Public and private keys are stored under the following db key pattern:
// /threads/keys/<b32 thread id no padding>/<b32 log id no padding>/(pub|priv|read|follow)
var (
	kbBase       = ds.NewKey("/thread/keys")
	pubSuffix    = ds.NewKey("/pub")
	privSuffix   = ds.NewKey("/priv")
	readSuffix   = ds.NewKey("/read")
	followSuffix = ds.NewKey("/follow")
)

var _ tstore.KeyBook = (*dsKeyBook)(nil)

// NewKeyBook returns a new key book for storing public and private keys
// of (thread.ID, peer.ID) pairs with durable guarantees by store.
func NewKeyBook(store ds.Datastore) (tstore.KeyBook, error) {
	return &dsKeyBook{ds: store}, nil
}

// PubKey returns the public key of (thread.ID, peer.ID). The implementation
// assumes the key is in the store with the exception that peer.ID is an
// Identity multihash. If the public key can't be resolved, nil is returned.
func (kb *dsKeyBook) PubKey(t thread.ID, p peer.ID) (ic.PubKey, error) {
	key := dsKey(t, p, kbBase).Child(pubSuffix)

	var pk ic.PubKey
	if v, err := kb.ds.Get(key); err == nil {
		if pk, err = ic.UnmarshalPublicKey(v); err != nil {
			return nil, fmt.Errorf("store backed public key %v can't be unmarshaled: %w", key, err)
		}
	} else if err == ds.ErrNotFound {
		pk, err = p.ExtractPublicKey()
		switch err {
		case nil:
			pkb, err := pk.Bytes()
			if err != nil {
				return nil, fmt.Errorf("error when getting bytes from identity multihashed public key %v: %w", key, err)
			}
			if kb.ds.Put(key, pkb) != nil {
				return nil, fmt.Errorf("error when putting identity multihashed public key %v in store: %w", key, err)
			}
		case peer.ErrNoPublicKey:
			return nil, fmt.Errorf("missing stored public key %v and isn't an identity multihash", key)
		default:
			return nil, fmt.Errorf("missing stored public key %v errored while extracting public key: %w", key, err)
		}
	} else {
		return nil, fmt.Errorf("error when getting key %v from store", key)
	}
	return pk, nil
}

// AddPubKey adds the public key of peer.ID which should match accordingly.
func (kb *dsKeyBook) AddPubKey(t thread.ID, p peer.ID, pk ic.PubKey) error {
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
	key := dsKey(t, p, kbBase).Child(pubSuffix)
	if kb.ds.Put(key, val) != nil {
		return fmt.Errorf("error when putting public key in store: %w", err)
	}
	return nil
}

// PrivKey returns the private key of (thread.ID, peer.ID). If not private key
// is stored, returns nil.
func (kb *dsKeyBook) PrivKey(t thread.ID, p peer.ID) (ic.PrivKey, error) {
	key := dsKey(t, p, kbBase).Child(privSuffix)
	v, err := kb.ds.Get(key)
	if err == ds.ErrNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("error when getting private key for %s", key)
	}
	sk, err := ic.UnmarshalPrivateKey(v)
	if err != nil {
		return nil, fmt.Errorf("error when unmarshaling private key of %v", key)
	}
	return sk, nil
}

// AddPrivKey adds the private key of peer.ID which should match accordingly.
func (kb *dsKeyBook) AddPrivKey(t thread.ID, p peer.ID, sk ic.PrivKey) error {
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
	key := dsKey(t, p, kbBase).Child(privSuffix)
	if err = kb.ds.Put(key, skb); err != nil {
		return fmt.Errorf("error when putting key %v in datastore: %w", key, err)
	}
	return nil
}

// ReadKey returns the read key associated with peer.ID for thread.ID thread.
// In case it doesn't exist, it will return nil.
func (kb *dsKeyBook) ReadKey(t thread.ID, p peer.ID) ([]byte, error) {
	key := dsKey(t, p, kbBase).Child(readSuffix)
	v, err := kb.ds.Get(key)
	if err != nil {
		return nil, fmt.Errorf("error when getting read key from store for peer ID %v", key)
	}
	return v, nil
}

// AddReadKey adds a read key for a peer.ID
func (kb *dsKeyBook) AddReadKey(t thread.ID, p peer.ID, rk []byte) error {
	if rk == nil {
		return fmt.Errorf("read-key is nil")
	}
	key := dsKey(t, p, kbBase).Child(readSuffix)
	if err := kb.ds.Put(key, rk); err != nil {
		return fmt.Errorf("error when adding read-key to datastore: %w", err)
	}
	return nil
}

func (kb *dsKeyBook) FollowKey(t thread.ID, p peer.ID) ([]byte, error) {
	key := dsKey(t, p, kbBase).Child(followSuffix)

	v, err := kb.ds.Get(key)
	if err != nil {
		return nil, fmt.Errorf("error when getting follow-key from datastore: %v", err)
	}
	return v, nil
}

func (kb *dsKeyBook) AddFollowKey(t thread.ID, p peer.ID, fk []byte) error {
	if fk == nil {
		return fmt.Errorf("follow-key is nil")
	}
	key := dsKey(t, p, kbBase).Child(followSuffix)
	if err := kb.ds.Put(key, fk); err != nil {
		return fmt.Errorf("error when adding follow-key to datastore: %w", err)
	}
	return nil
}

func (kb *dsKeyBook) LogsWithKeys(t thread.ID) (peer.IDSlice, error) {
	ids, err := uniqueLogIds(kb.ds, kbBase.ChildString(base32.RawStdEncoding.EncodeToString(t.Bytes())), func(result query.Result) string {
		return ds.RawKey(result.Key).Parent().Name()
	})
	if err != nil {
		return nil, fmt.Errorf("error while retrieving logs with addresses: %v", err)
	}
	return ids, nil
}

func (kb *dsKeyBook) ThreadsFromKeys() (thread.IDSlice, error) {
	ids, err := uniqueThreadIds(kb.ds, kbBase, func(result query.Result) string {
		return ds.RawKey(result.Key).Parent().Parent().Name()
	})
	if err != nil {
		return nil, fmt.Errorf("error while retrieving threads from keys: %v", err)
	}
	return ids, nil
}
