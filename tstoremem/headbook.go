package tstoremem

import (
	"sync"

	"github.com/ipfs/go-cid"
	peer "github.com/libp2p/go-libp2p-core/peer"
	"github.com/textileio/go-textile-core/thread"
	tstore "github.com/textileio/go-textile-core/threadstore"
)

type memoryHeadBook struct {
	sync.RWMutex

	heads map[thread.ID]map[peer.ID]map[cid.Cid]struct{}
}

func (mhb *memoryHeadBook) getHeads(t thread.ID, p peer.ID) (map[cid.Cid]struct{}, bool) {
	lmap, found := mhb.heads[t]
	if lmap == nil {
		return nil, found
	}
	hmap, found := lmap[p]
	return hmap, found
}

func NewHeadBook() tstore.HeadBook {
	return &memoryHeadBook{
		heads: map[thread.ID]map[peer.ID]map[cid.Cid]struct{}{},
	}
}

func (mhb *memoryHeadBook) AddHead(t thread.ID, p peer.ID, head cid.Cid) error {
	return mhb.AddHeads(t, p, []cid.Cid{head})
}

func (mhb *memoryHeadBook) AddHeads(t thread.ID, p peer.ID, heads []cid.Cid) error {
	mhb.Lock()
	defer mhb.Unlock()

	hmap, _ := mhb.getHeads(t, p)
	if hmap == nil {
		if mhb.heads[t] == nil {
			mhb.heads[t] = make(map[peer.ID]map[cid.Cid]struct{}, 1)
		}
		hmap = make(map[cid.Cid]struct{}, len(heads))
		mhb.heads[t][p] = hmap
	}

	for _, h := range heads {
		if !h.Defined() {
			log.Warningf("was passed nil head for %s", p)
			continue
		}
		hmap[h] = struct{}{}
	}
	return nil
}

func (mhb *memoryHeadBook) SetHead(t thread.ID, p peer.ID, head cid.Cid) error {
	return mhb.SetHeads(t, p, []cid.Cid{head})
}

func (mhb *memoryHeadBook) SetHeads(t thread.ID, p peer.ID, heads []cid.Cid) error {
	mhb.Lock()
	defer mhb.Unlock()

	hmap, _ := mhb.getHeads(t, p)
	if hmap == nil {
		if mhb.heads[t] == nil {
			mhb.heads[t] = make(map[peer.ID]map[cid.Cid]struct{}, 1)
		}
	}
	hmap = make(map[cid.Cid]struct{}, len(heads))
	mhb.heads[t][p] = hmap

	for _, h := range heads {
		if !h.Defined() {
			log.Warningf("was passed nil head for %s", p)
			continue
		}
		hmap[h] = struct{}{}
	}
	return nil
}

func (mhb *memoryHeadBook) Heads(t thread.ID, p peer.ID) ([]cid.Cid, error) {
	mhb.RLock()
	defer mhb.RUnlock()

	var heads []cid.Cid
	hmap, _ := mhb.getHeads(t, p)
	if hmap == nil {
		return heads
	}
	for h := range hmap {
		heads = append(heads, h)
	}
	return heads, nil
}

func (mhb *memoryHeadBook) ClearHeads(t thread.ID, p peer.ID) error {
	mhb.Lock()
	defer mhb.Unlock()

	lmap := mhb.heads[t]
	if lmap != nil {
		delete(lmap, p)
		if len(lmap) == 0 {
			delete(mhb.heads, t)
		}
	}
	return nil
}
