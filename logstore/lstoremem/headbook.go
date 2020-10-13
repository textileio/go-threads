package lstoremem

import (
	"sync"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/peer"
	core "github.com/textileio/go-threads/core/logstore"
	"github.com/textileio/go-threads/core/thread"
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

var _ core.HeadBook = (*memoryHeadBook)(nil)

func NewHeadBook() core.HeadBook {
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
			log.Warnf("was passed nil head for %s", p)
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
			log.Warnf("was passed nil head for %s", p)
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
		return heads, nil
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

func (mhb *memoryHeadBook) DumpHeads() (core.DumpHeadBook, error) {
	var dump = core.DumpHeadBook{
		Data: make(map[thread.ID]map[peer.ID][]cid.Cid, len(mhb.heads)),
	}

	for tid, logs := range mhb.heads {
		lm := make(map[peer.ID][]cid.Cid, len(logs))
		for lid, hs := range logs {
			heads := make([]cid.Cid, 0, len(hs))
			for head := range hs {
				heads = append(heads, head)
			}
			lm[lid] = heads
		}
		dump.Data[tid] = lm
	}

	return dump, nil
}

func (mhb *memoryHeadBook) RestoreHeads(dump core.DumpHeadBook) error {
	if !AllowEmptyRestore && len(dump.Data) == 0 {
		return core.ErrEmptyDump
	}

	var restored = make(map[thread.ID]map[peer.ID]map[cid.Cid]struct{}, len(dump.Data))
	for tid, logs := range dump.Data {
		lm := make(map[peer.ID]map[cid.Cid]struct{}, len(logs))
		for lid, hs := range logs {
			hm := make(map[cid.Cid]struct{}, len(hs))
			for _, head := range hs {
				hm[head] = struct{}{}
			}
			lm[lid] = hm
		}
		restored[tid] = lm
	}

	mhb.heads = restored
	return nil
}
