package lstoremem

import (
	"errors"
	"sync"

	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	core "github.com/textileio/go-threads/core/logstore"
	"github.com/textileio/go-threads/core/thread"
	sym "github.com/textileio/go-threads/crypto/symmetric"
)

type memoryKeyBook struct {
	sync.RWMutex

	pks map[thread.ID]map[peer.ID]crypto.PubKey
	sks map[thread.ID]map[peer.ID]crypto.PrivKey
	rks map[thread.ID][]byte
	fks map[thread.ID][]byte
}

func (mkb *memoryKeyBook) getPubKey(t thread.ID, p peer.ID) (crypto.PubKey, bool) {
	lmap, found := mkb.pks[t]
	if lmap == nil {
		return nil, found
	}
	hmap, found := lmap[p]
	return hmap, found
}

func (mkb *memoryKeyBook) getPrivKey(t thread.ID, p peer.ID) (crypto.PrivKey, bool) {
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
		pks: map[thread.ID]map[peer.ID]crypto.PubKey{},
		sks: map[thread.ID]map[peer.ID]crypto.PrivKey{},
		rks: map[thread.ID][]byte{},
		fks: map[thread.ID][]byte{},
	}
}

func (mkb *memoryKeyBook) PubKey(t thread.ID, p peer.ID) (crypto.PubKey, error) {
	mkb.RLock()
	pk, _ := mkb.getPubKey(t, p)
	mkb.RUnlock()
	return pk, nil
}

func (mkb *memoryKeyBook) AddPubKey(t thread.ID, p peer.ID, pk crypto.PubKey) error {
	// check it's correct first
	if !p.MatchesPublicKey(pk) {
		return errors.New("ID does not match PublicKey")
	}

	mkb.Lock()
	if mkb.pks[t] == nil {
		mkb.pks[t] = make(map[peer.ID]crypto.PubKey, 1)
	}
	mkb.pks[t][p] = pk
	mkb.Unlock()
	return nil
}

func (mkb *memoryKeyBook) PrivKey(t thread.ID, p peer.ID) (crypto.PrivKey, error) {
	mkb.RLock()
	sk, _ := mkb.getPrivKey(t, p)
	mkb.RUnlock()
	return sk, nil
}

func (mkb *memoryKeyBook) AddPrivKey(t thread.ID, p peer.ID, sk crypto.PrivKey) error {
	if sk == nil {
		return errors.New("sk is nil (PrivKey)")
	}

	// check it's correct first
	if !p.MatchesPrivateKey(sk) {
		return errors.New("ID does not match PrivateKey")
	}

	mkb.Lock()
	if mkb.sks[t] == nil {
		mkb.sks[t] = make(map[peer.ID]crypto.PrivKey, 1)
	}
	mkb.sks[t][p] = sk
	mkb.Unlock()
	return nil
}

func (mkb *memoryKeyBook) ReadKey(t thread.ID) (key *sym.Key, err error) {
	mkb.RLock()
	b := mkb.rks[t]
	if b != nil {
		key, err = sym.FromBytes(b)
	}
	mkb.RUnlock()
	return key, err
}

func (mkb *memoryKeyBook) AddReadKey(t thread.ID, key *sym.Key) error {
	if key == nil {
		return errors.New("key is nil (ReadKey)")
	}

	mkb.Lock()
	mkb.rks[t] = key.Bytes()
	mkb.Unlock()
	return nil
}

func (mkb *memoryKeyBook) ServiceKey(t thread.ID) (key *sym.Key, err error) {
	mkb.RLock()
	b := mkb.fks[t]
	if b != nil {
		key, err = sym.FromBytes(b)
	}
	mkb.RUnlock()
	return
}

func (mkb *memoryKeyBook) AddServiceKey(t thread.ID, key *sym.Key) error {
	if key == nil {
		return errors.New("key is nil (ServiceKey)")
	}

	mkb.Lock()
	mkb.fks[t] = key.Bytes()
	mkb.Unlock()
	return nil
}

func (mkb *memoryKeyBook) ClearKeys(t thread.ID) error {
	mkb.Lock()
	delete(mkb.pks, t)
	delete(mkb.sks, t)
	delete(mkb.rks, t)
	delete(mkb.fks, t)
	mkb.Unlock()
	return nil
}

func (mkb *memoryKeyBook) ClearLogKeys(t thread.ID, _ peer.ID) error {
	mkb.Lock()
	delete(mkb.pks, t)
	delete(mkb.sks, t)
	mkb.Unlock()
	return nil
}

func (mkb *memoryKeyBook) LogsWithKeys(t thread.ID) (peer.IDSlice, error) {
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

func (mkb *memoryKeyBook) ThreadsFromKeys() (thread.IDSlice, error) {
	mkb.RLock()
	ts := make(map[thread.ID]struct{})
	for t := range mkb.pks {
		ts[t] = struct{}{}
	}
	for t := range mkb.sks {
		ts[t] = struct{}{}
	}
	mkb.RUnlock()
	var tids thread.IDSlice
	for t := range ts {
		tids = append(tids, t)
	}
	return tids, nil
}

func (mkb *memoryKeyBook) DumpKeys() (core.DumpKeyBook, error) {
	mkb.RLock()
	defer mkb.RUnlock()

	var (
		dump    core.DumpKeyBook
		public  = make(map[thread.ID]map[peer.ID]crypto.PubKey, len(mkb.pks))
		private = make(map[thread.ID]map[peer.ID]crypto.PrivKey, len(mkb.sks))
		read    = make(map[thread.ID][]byte, len(mkb.rks))
		service = make(map[thread.ID][]byte, len(mkb.fks))
	)

	for tid, logs := range mkb.pks {
		lm := make(map[peer.ID]crypto.PubKey, len(logs))
		for lid, key := range logs {
			lm[lid] = key
		}
		public[tid] = lm
	}

	for tid, logs := range mkb.sks {
		lm := make(map[peer.ID]crypto.PrivKey, len(logs))
		for lid, key := range logs {
			lm[lid] = key
		}
		private[tid] = lm
	}

	for tid, key := range mkb.rks {
		read[tid] = key
	}

	for tid, key := range mkb.fks {
		service[tid] = key
	}

	dump.Data.Public = public
	dump.Data.Private = private
	dump.Data.Read = read
	dump.Data.Service = service

	return dump, nil
}

func (mkb *memoryKeyBook) RestoreKeys(dump core.DumpKeyBook) error {
	if len(dump.Data.Public) == 0 &&
		len(dump.Data.Private) == 0 &&
		len(dump.Data.Read) == 0 &&
		len(dump.Data.Service) == 0 {
		return core.ErrEmptyDump
	}

	mkb.Lock()
	defer mkb.Unlock()

	mkb.pks = dump.Data.Public
	mkb.sks = dump.Data.Private
	mkb.rks = dump.Data.Read
	mkb.fks = dump.Data.Service
	return nil
}
