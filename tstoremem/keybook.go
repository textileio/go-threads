package tstoremem

import (
	"errors"
	"sync"

	ic "github.com/libp2p/go-libp2p-core/crypto"
	peer "github.com/libp2p/go-libp2p-core/peer"
	"github.com/textileio/go-textile-core/thread"
	tstore "github.com/textileio/go-textile-core/threadstore"
)

type memoryLogKeyBook struct {
	sync.RWMutex

	pks map[peer.ID]ic.PubKey
	sks map[peer.ID]ic.PrivKey
	rks map[peer.ID][]byte
	fks map[peer.ID][]byte
}

func NewLogKeyBook() tstore.LogKeyBook {
	return &memoryLogKeyBook{
		pks: map[peer.ID]ic.PubKey{},
		sks: map[peer.ID]ic.PrivKey{},
		rks: map[peer.ID][]byte{},
		fks: map[peer.ID][]byte{},
	}
}

func (mkb *memoryLogKeyBook) LogsWithKeys(t thread.ID) peer.IDSlice {
	mkb.RLock()
	ps := make(peer.IDSlice, 0, len(mkb.pks)+len(mkb.sks))
	for p := range mkb.pks {
		ps = append(ps, p)
	}
	for p := range mkb.sks {
		if _, found := mkb.pks[p]; !found {
			ps = append(ps, p)
		}
	}
	mkb.RUnlock()
	return ps
}

func (mkb *memoryLogKeyBook) ThreadsFromKeys() thread.IDSlice {
	panic("implement me")
}

func (mkb *memoryLogKeyBook) PubKey(t thread.ID, l peer.ID) ic.PubKey {
	mkb.RLock()
	pk := mkb.pks[l]
	mkb.RUnlock()
	if pk != nil {
		return pk
	}
	pk, err := l.ExtractPublicKey()
	if err == nil {
		mkb.Lock()
		mkb.pks[l] = pk
		mkb.Unlock()
	}
	return pk
}

func (mkb *memoryLogKeyBook) AddPubKey(t thread.ID, l peer.ID, pk ic.PubKey) error {
	// check it's correct first
	if !l.MatchesPublicKey(pk) {
		return errors.New("ID does not match PublicKey")
	}

	mkb.Lock()
	mkb.pks[l] = pk
	mkb.Unlock()
	return nil
}

func (mkb *memoryLogKeyBook) PrivKey(t thread.ID, l peer.ID) ic.PrivKey {
	mkb.RLock()
	sk := mkb.sks[l]
	mkb.RUnlock()
	return sk
}

func (mkb *memoryLogKeyBook) AddPrivKey(t thread.ID, l peer.ID, sk ic.PrivKey) error {
	if sk == nil {
		return errors.New("sk is nil (PrivKey)")
	}

	// check it's correct first
	if !l.MatchesPrivateKey(sk) {
		return errors.New("ID does not match PrivateKey")
	}

	mkb.Lock()
	mkb.sks[l] = sk
	mkb.Unlock()
	return nil
}

func (mkb *memoryLogKeyBook) ReadKey(thread.ID, peer.ID) []byte {
	panic("implement me")
}

func (mkb *memoryLogKeyBook) AddReadKey(thread.ID, peer.ID, []byte) error {
	panic("implement me")
}

func (mkb *memoryLogKeyBook) FollowKey(thread.ID, peer.ID) []byte {
	panic("implement me")
}

func (mkb *memoryLogKeyBook) AddFollowKey(thread.ID, peer.ID, []byte) error {
	panic("implement me")
}
