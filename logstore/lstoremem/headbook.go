package lstoremem

import (
	"sync"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/peer"
	core "github.com/textileio/go-threads/core/logstore"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/go-threads/util"
)

type memoryHeadBook struct {
	sync.RWMutex
	threads map[thread.ID]struct {
		heads map[peer.ID]map[cid.Cid]struct{}
		edge  uint64
	}
}

func (mhb *memoryHeadBook) getHeads(t thread.ID, p peer.ID, createEmpty bool) map[cid.Cid]struct{} {
	lmap := mhb.threads[t]
	if lmap.heads == nil {
		if !createEmpty {
			return nil
		}
		lmap.heads = make(map[peer.ID]map[cid.Cid]struct{}, 1)
		mhb.threads[t] = lmap
	}
	hmap := lmap.heads[p]
	if hmap == nil && createEmpty {
		hmap = make(map[cid.Cid]struct{})
		lmap.heads[p] = hmap
	}
	return hmap
}

var _ core.HeadBook = (*memoryHeadBook)(nil)

func NewHeadBook() core.HeadBook {
	return &memoryHeadBook{
		threads: make(map[thread.ID]struct {
			heads map[peer.ID]map[cid.Cid]struct{}
			edge  uint64
		}),
	}
}

func (mhb *memoryHeadBook) AddHead(t thread.ID, p peer.ID, head cid.Cid) error {
	return mhb.AddHeads(t, p, []cid.Cid{head})
}

func (mhb *memoryHeadBook) AddHeads(t thread.ID, p peer.ID, heads []cid.Cid) error {
	mhb.Lock()
	defer mhb.Unlock()
	defer mhb.updateEdge(t)

	hmap := mhb.getHeads(t, p, true)
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
	defer mhb.updateEdge(t)

	var hset = make(map[cid.Cid]struct{}, len(heads))
	for _, h := range heads {
		if !h.Defined() {
			log.Warnf("was passed nil head for %s", p)
			continue
		}
		hset[h] = struct{}{}
	}
	// replace heads
	if mhb.threads[t].heads == nil {
		mhb.threads[t] = struct {
			heads map[peer.ID]map[cid.Cid]struct{}
			edge  uint64
		}{heads: make(map[peer.ID]map[cid.Cid]struct{})}
	}
	mhb.threads[t].heads[p] = hset
	return nil
}

func (mhb *memoryHeadBook) Heads(t thread.ID, p peer.ID) ([]cid.Cid, error) {
	mhb.RLock()
	defer mhb.RUnlock()

	hset := mhb.getHeads(t, p, false)
	if hset == nil {
		return nil, nil
	}

	var heads = make([]cid.Cid, 0, len(hset))
	for h := range hset {
		heads = append(heads, h)
	}
	return heads, nil
}

func (mhb *memoryHeadBook) ClearHeads(t thread.ID, p peer.ID) error {
	mhb.Lock()
	defer mhb.Unlock()

	var lset = mhb.threads[t].heads
	if lset == nil {
		return nil
	}
	delete(lset, p)
	if len(lset) == 0 {
		delete(mhb.threads, t)
	} else {
		mhb.updateEdge(t)
	}
	return nil
}

func (mhb *memoryHeadBook) HeadsEdge(t thread.ID) (uint64, error) {
	mhb.RLock()
	defer mhb.RUnlock()

	lset, found := mhb.threads[t]
	if !found {
		return 0, core.ErrThreadNotFound
	}
	// invariant: edge always precomputed for existing thread
	return lset.edge, nil
}

func (mhb *memoryHeadBook) updateEdge(t thread.ID) {
	// invariant: requested thread exist
	var (
		lset  = mhb.threads[t]
		heads = make([]util.LogHead, 0, len(lset.heads))
	)
	for lid, hs := range lset.heads {
		for head := range hs {
			heads = append(heads, util.LogHead{
				LogID: lid,
				Head:  head,
			})
		}
	}
	lset.edge = util.ComputeHeadsEdge(heads)
	mhb.threads[t] = lset
}

func (mhb *memoryHeadBook) DumpHeads() (core.DumpHeadBook, error) {
	var dump = core.DumpHeadBook{
		Data: make(map[thread.ID]map[peer.ID][]cid.Cid, len(mhb.threads)),
	}

	for tid, lset := range mhb.threads {
		lm := make(map[peer.ID][]cid.Cid, len(lset.heads))
		for lid, hs := range lset.heads {
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

	// reset stored
	mhb.threads = make(map[thread.ID]struct {
		heads map[peer.ID]map[cid.Cid]struct{}
		edge  uint64
	}, len(dump.Data))

	for tid, logs := range dump.Data {
		lm := make(map[peer.ID]map[cid.Cid]struct{}, len(logs))
		for lid, hs := range logs {
			hm := make(map[cid.Cid]struct{}, len(hs))
			for _, head := range hs {
				hm[head] = struct{}{}
			}
			lm[lid] = hm
		}
		mhb.threads[tid] = struct {
			heads map[peer.ID]map[cid.Cid]struct{}
			edge  uint64
		}{heads: lm}
		mhb.updateEdge(tid)
	}

	return nil
}
