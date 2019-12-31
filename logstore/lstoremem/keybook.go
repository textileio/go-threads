package lstoremem

import (
	"errors"
	"sync"

	ic "github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	core "github.com/textileio/go-threads/core/logstore"
	"github.com/textileio/go-threads/core/service"
	sym "github.com/textileio/go-threads/crypto/symmetric"
)

type memoryKeyBook struct {
	sync.RWMutex

	pks map[service.ID]map[peer.ID]ic.PubKey
	sks map[service.ID]map[peer.ID]ic.PrivKey
	rks map[service.ID][]byte
	fks map[service.ID][]byte
}

func (mkb *memoryKeyBook) getPubKey(t service.ID, p peer.ID) (ic.PubKey, bool) {
	lmap, found := mkb.pks[t]
	if lmap == nil {
		return nil, found
	}
	hmap, found := lmap[p]
	return hmap, found
}

func (mkb *memoryKeyBook) getPrivKey(t service.ID, p peer.ID) (ic.PrivKey, bool) {
	lmap, found := mkb.sks[t]
	if lmap == nil {
		return nil, found
	}
	hmap, found := lmap[p]
	return hmap, found
}

var _ core.KeyBook = (*memoryKeyBook)(nil)

func NewKeyBook() core.KeyBook {
	return &memoryKeyBook{
		pks: map[service.ID]map[peer.ID]ic.PubKey{},
		sks: map[service.ID]map[peer.ID]ic.PrivKey{},
		rks: map[service.ID][]byte{},
		fks: map[service.ID][]byte{},
	}
}

func (mkb *memoryKeyBook) LogsWithKeys(t service.ID) (peer.IDSlice, error) {
	mkb.RLock()
	ps := make(map[peer.ID]struct{})
	if mkb.pks[t] != nil {
		for p := range mkb.pks[t] {
			ps[p] = struct{}{}
		}
	}
	if mkb.sks[t] != nil {
		for p := range mkb.sks[t] {
			ps[p] = struct{}{}
		}
	}
	mkb.RUnlock()
	var pids peer.IDSlice
	for p := range ps {
		pids = append(pids, p)
	}
	return pids, nil
}

func (mkb *memoryKeyBook) ThreadsFromKeys() (service.IDSlice, error) {
	mkb.RLock()
	ts := make(map[service.ID]struct{})
	for t := range mkb.pks {
		ts[t] = struct{}{}
	}
	for t := range mkb.sks {
		ts[t] = struct{}{}
	}
	mkb.RUnlock()
	var tids service.IDSlice
	for t := range ts {
		tids = append(tids, t)
	}
	return tids, nil
}

func (mkb *memoryKeyBook) PubKey(t service.ID, p peer.ID) (ic.PubKey, error) {
	mkb.RLock()
	pk, _ := mkb.getPubKey(t, p)
	mkb.RUnlock()
	return pk, nil
}

func (mkb *memoryKeyBook) AddPubKey(t service.ID, p peer.ID, pk ic.PubKey) error {
	// check it's correct first
	if !p.MatchesPublicKey(pk) {
		return errors.New("ID does not match PublicKey")
	}

	mkb.Lock()
	if mkb.pks[t] == nil {
		mkb.pks[t] = make(map[peer.ID]ic.PubKey, 1)
	}
	mkb.pks[t][p] = pk
	mkb.Unlock()
	return nil
}

func (mkb *memoryKeyBook) PrivKey(t service.ID, p peer.ID) (ic.PrivKey, error) {
	mkb.RLock()
	sk, _ := mkb.getPrivKey(t, p)
	mkb.RUnlock()
	return sk, nil
}

func (mkb *memoryKeyBook) AddPrivKey(t service.ID, p peer.ID, sk ic.PrivKey) error {
	if sk == nil {
		return errors.New("sk is nil (PrivKey)")
	}

	// check it's correct first
	if !p.MatchesPrivateKey(sk) {
		return errors.New("ID does not match PrivateKey")
	}

	mkb.Lock()
	if mkb.sks[t] == nil {
		mkb.sks[t] = make(map[peer.ID]ic.PrivKey, 1)
	}
	mkb.sks[t][p] = sk
	mkb.Unlock()
	return nil
}

func (mkb *memoryKeyBook) ReadKey(t service.ID) (key *sym.Key, err error) {
	mkb.RLock()
	b := mkb.rks[t]
	if b != nil {
		key, err = sym.NewKey(b)
	}
	mkb.RUnlock()
	return key, err
}

func (mkb *memoryKeyBook) AddReadKey(t service.ID, key *sym.Key) error {
	if key == nil {
		return errors.New("key is nil (ReadKey)")
	}

	mkb.Lock()
	mkb.rks[t] = key.Bytes()
	mkb.Unlock()
	return nil
}

func (mkb *memoryKeyBook) FollowKey(t service.ID) (key *sym.Key, err error) {
	mkb.RLock()
	b := mkb.fks[t]
	if b != nil {
		key, err = sym.NewKey(b)
	}
	mkb.RUnlock()
	return
}

func (mkb *memoryKeyBook) AddFollowKey(t service.ID, key *sym.Key) error {
	if key == nil {
		return errors.New("key is nil (FollowKey)")
	}

	mkb.Lock()
	mkb.fks[t] = key.Bytes()
	mkb.Unlock()
	return nil
}
