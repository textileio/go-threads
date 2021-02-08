package lstoremem

import (
	"context"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dgtony/collections/bitset"
	logging "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-peerstore/addr"
	ma "github.com/multiformats/go-multiaddr"
	core "github.com/textileio/go-threads/core/logstore"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/go-threads/util"
)

var log = logging.Logger("logstore")

const numSegments = 256

type expiringAddr struct {
	Addr    ma.Multiaddr
	TTL     time.Duration
	Expires time.Time
}

func (e *expiringAddr) ExpiredBy(t time.Time) bool {
	return t.After(e.Expires)
}

type threadInfo struct {
	segments *bitset.FixedBitSet
	// edge value is eventually consistent
	edge uint64
	// avoid concurrent updates
	ongoing bool
}

type addrSegments struct {
	segments [numSegments]*addrSegment
	threads  map[thread.ID]*threadInfo
	mx       sync.Mutex
}

func newAddrSegments() *addrSegments {
	var as addrSegments
	for i := 0; i < len(as.segments); i++ {
		as.segments[i] = newAddrSegment()
	}
	as.threads = make(map[thread.ID]*threadInfo)
	return &as
}

func (s *addrSegments) processAddrs(
	t thread.ID,
	p peer.ID,
	create, write bool,
	operation func(m map[string]*expiringAddr),
) {
	var (
		si  = int(s.sIdx(p))
		seg = s.segments[si]
	)

	if write {
		seg.Lock()
		defer seg.Unlock()
	} else {
		seg.RLock()
		defer seg.RUnlock()
	}

	amap, found, threadCreated := seg.get(t, p, create)
	if threadCreated {
		// When addrSegment returns positive flag threadCreated it is implied that given thread
		// was touched for the first time in current segment (i.e new peer was added).
		// But thread may have addresses in other segments as well.
		s.mx.Lock()
		info, ok := s.threads[t]
		if !ok {
			info = &threadInfo{segments: bitset.NewFixed(numSegments)}
			s.threads[t] = info
		}
		info.segments.Set(si)
		s.mx.Unlock()
	}
	if found {
		// operations are performed on existing address maps only
		operation(amap)
	}
}

func (s *addrSegments) removeAddrs(t thread.ID, p peer.ID) bool {
	var seg = s.segments[s.sIdx(p)]
	seg.Lock()
	defer seg.Unlock()
	return seg.remove(t, p)
}

func (s *addrSegments) sIdx(p peer.ID) uint8 {
	return p[len(p)-1]
}

func (s *addrSegments) updateEdge(t thread.ID) {
	s.mx.Lock()
	var info, found = s.threads[t]
	if !found || info.ongoing {
		s.mx.Unlock()
		return
	}
	// mark thread to avoid concurrent updates
	info.ongoing = true
	sIdxs := info.segments.Ones()
	s.mx.Unlock()

	var (
		addrs []util.PeerAddr
		now   = time.Now()
		ts    int32
		lk    sync.Mutex
		wg    sync.WaitGroup
	)

	// collect thread addresses from all involved segments in parallel
	for _, sIdx := range sIdxs {
		wg.Add(1)
		go func(seg *addrSegment) {
			seg.RLock()
			for p, amap := range seg.addrs[t] {
				atomic.AddInt32(&ts, 1)
				for _, e := range amap {
					if !e.ExpiredBy(now) {
						lk.Lock()
						addrs = append(addrs, util.PeerAddr{
							PeerID: p,
							Addr:   e.Addr,
						})
						lk.Unlock()
					}
				}
			}
			seg.RUnlock()
			wg.Done()
		}(s.segments[sIdx])
	}

	wg.Wait()
	s.mx.Lock()
	info.ongoing = false
	if ts > 0 {
		info.edge = util.ComputeAddrsEdge(addrs)
	} else {
		// no thread addresses left, remove thread info
		delete(s.threads, t)
	}
	s.mx.Unlock()
}

type addrSegment struct {
	sync.RWMutex

	// Use pointers to save memory. Maps always leave some fraction of their
	// space unused. storing the *values* directly in the map will
	// drastically increase the space waste. In our case, by 6x.
	addrs map[thread.ID]map[peer.ID]map[string]*expiringAddr
}

func newAddrSegment() *addrSegment {
	return &addrSegment{addrs: make(map[thread.ID]map[peer.ID]map[string]*expiringAddr)}
}

func (s *addrSegment) get(
	t thread.ID,
	p peer.ID,
	create bool,
) (amap map[string]*expiringAddr, found, threadCreated bool) {
	pmap, found := s.addrs[t]
	if !found {
		if !create {
			return nil, false, false
		}
		pmap = make(map[peer.ID]map[string]*expiringAddr, 1)
		s.addrs[t] = pmap
		threadCreated = true
	}

	amap, found = pmap[p]
	if !found {
		if !create {
			return nil, false, false
		}
		amap = make(map[string]*expiringAddr, 1)
		s.addrs[t][p] = amap
		found = true
	}
	return
}

func (s *addrSegment) remove(t thread.ID, p peer.ID) bool {
	pmap, found := s.addrs[t]
	if !found {
		return false
	}
	_, found = pmap[p]
	if found {
		delete(pmap, p)
		if len(pmap) == 0 {
			delete(s.addrs, t)
		}
	}
	return found
}

// memoryAddrBook manages addresses.
type memoryAddrBook struct {
	segments *addrSegments

	ctx    context.Context
	cancel func()
	gcLock sync.Mutex

	subManager *AddrSubManager
}

var _ core.AddrBook = (*memoryAddrBook)(nil)

func NewAddrBook() core.AddrBook {
	ctx, cancel := context.WithCancel(context.Background())

	ab := &memoryAddrBook{
		segments:   newAddrSegments(),
		subManager: NewAddrSubManager(),
		ctx:        ctx,
		cancel:     cancel,
	}

	go ab.background()
	return ab
}

// background periodically schedules a gc
func (mab *memoryAddrBook) background() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ts := mab.gc()
			mab.recomputeEdges(ts)

		case <-mab.ctx.Done():
			return
		}
	}
}

func (mab *memoryAddrBook) Close() error {
	mab.cancel()
	return nil
}

// gc garbage collects the in-memory address book
// and returns IDs for all  traversed threads.
func (mab *memoryAddrBook) gc() map[thread.ID]struct{} {
	mab.gcLock.Lock()
	defer mab.gcLock.Unlock()

	var (
		now = time.Now()
		tc  = make(map[thread.ID]struct{})
	)

	for _, s := range mab.segments.segments {
		s.Lock()
		for t, pmap := range s.addrs {
			for p, amap := range pmap {
				for k, a := range amap {
					if a.ExpiredBy(now) {
						delete(amap, k)
					}
				}
				if len(amap) == 0 {
					delete(s.addrs[t], p)
				}
			}
			if len(pmap) == 0 {
				delete(s.addrs, t)
			}
			tc[t] = struct{}{}
		}
		s.Unlock()
	}
	return tc
}

// recomputeEdges walks through all the threads and updates edge values.
func (mab *memoryAddrBook) recomputeEdges(ts map[thread.ID]struct{}) {
	for tid := range ts {
		mab.segments.updateEdge(tid)
	}
}

func (mab *memoryAddrBook) LogsWithAddrs(t thread.ID) (peer.IDSlice, error) {
	var pids peer.IDSlice
	for _, s := range mab.segments.segments {
		s.RLock()
		for pid := range s.addrs[t] {
			pids = append(pids, pid)
		}
		s.RUnlock()
	}
	return pids, nil
}

func (mab *memoryAddrBook) ThreadsFromAddrs() (thread.IDSlice, error) {
	mab.segments.mx.Lock()
	defer mab.segments.mx.Unlock()
	var tids thread.IDSlice
	for t := range mab.segments.threads {
		tids = append(tids, t)
	}
	return tids, nil
}

// AddAddr calls AddAddrs(t, p, []ma.Multiaddr{addr}, ttl)
func (mab *memoryAddrBook) AddAddr(t thread.ID, p peer.ID, addr ma.Multiaddr, ttl time.Duration) error {
	return mab.AddAddrs(t, p, []ma.Multiaddr{addr}, ttl)
}

// AddAddrs gives memoryAddrBook addresses to use, with a given ttl
// (time-to-live), after which the address is no longer valid.
// This function never reduces the TTL or expiration of an address.
func (mab *memoryAddrBook) AddAddrs(t thread.ID, p peer.ID, addrs []ma.Multiaddr, ttl time.Duration) error {
	// if ttl is zero, exit. nothing to do.
	if ttl <= 0 {
		return nil
	}

	var addrChanged bool
	mab.segments.processAddrs(t, p, true, true, func(amap map[string]*expiringAddr) {
		exp := time.Now().Add(ttl)
		for _, a := range addrs {
			if a == nil {
				log.Warnf("was passed nil multiaddr for %s", p)
				continue
			}
			asBytes := a.Bytes()
			x, found := amap[string(asBytes)] // won't allocate.
			if !found {
				// not found, save and announce it.
				amap[string(asBytes)] = &expiringAddr{Addr: a, Expires: exp, TTL: ttl}
				mab.subManager.BroadcastAddr(p, a)
				addrChanged = true
			} else {
				// Update expiration/TTL independently.
				// We never want to reduce either.
				if ttl > x.TTL {
					x.TTL = ttl
				}
				if exp.After(x.Expires) {
					x.Expires = exp
				}
			}
		}
	})
	if addrChanged {
		go mab.segments.updateEdge(t)
	}
	return nil
}

// SetAddr calls mgr.SetAddrs(t, p, addr, ttl)
func (mab *memoryAddrBook) SetAddr(t thread.ID, p peer.ID, addr ma.Multiaddr, ttl time.Duration) error {
	return mab.SetAddrs(t, p, []ma.Multiaddr{addr}, ttl)
}

// SetAddrs sets the ttl on addresses. This clears any TTL there previously.
// This is used when we receive the best estimate of the validity of an address.
func (mab *memoryAddrBook) SetAddrs(t thread.ID, p peer.ID, addrs []ma.Multiaddr, ttl time.Duration) error {
	var addrChanged bool
	mab.segments.processAddrs(t, p, true, true, func(amap map[string]*expiringAddr) {
		var exp = time.Now().Add(ttl)
		for _, a := range addrs {
			if a == nil {
				log.Warnf("was passed nil multiaddr for %s", p)
				continue
			}

			var (
				aBytes   = a.Bytes()
				_, exist = amap[string(aBytes)]
			)

			// inserted or removed address
			addrChanged = addrChanged || (ttl > 0 != exist)

			// re-set all of them for new ttl.
			if ttl > 0 {
				amap[string(aBytes)] = &expiringAddr{Addr: a, Expires: exp, TTL: ttl}
				mab.subManager.BroadcastAddr(p, a)
			} else {
				delete(amap, string(aBytes))
			}
		}
	})
	if addrChanged {
		go mab.segments.updateEdge(t)
	}
	return nil
}

// UpdateAddrs updates the addresses associated with the given peer that have
// the given oldTTL to have the given newTTL.
func (mab *memoryAddrBook) UpdateAddrs(t thread.ID, p peer.ID, oldTTL time.Duration, newTTL time.Duration) error {
	mab.segments.processAddrs(t, p, false, true, func(amap map[string]*expiringAddr) {
		exp := time.Now().Add(newTTL)
		for k, a := range amap {
			if oldTTL == a.TTL {
				a.TTL = newTTL
				a.Expires = exp
				amap[k] = a
			}
		}
	})
	return nil
}

// Addrs returns all known (and valid) addresses for a given log
func (mab *memoryAddrBook) Addrs(t thread.ID, p peer.ID) ([]ma.Multiaddr, error) {
	var (
		addrExpired bool
		good        = make([]ma.Multiaddr, 0)
	)
	mab.segments.processAddrs(t, p, false, false, func(amap map[string]*expiringAddr) {
		now := time.Now()
		for _, m := range amap {
			if !m.ExpiredBy(now) {
				good = append(good, m.Addr)
			} else {
				addrExpired = true
			}
		}
	})
	if addrExpired {
		go mab.segments.updateEdge(t)
	}
	return good, nil
}

// ClearAddrs removes all previously stored addresses
func (mab *memoryAddrBook) ClearAddrs(t thread.ID, p peer.ID) error {
	if mab.segments.removeAddrs(t, p) {
		go mab.segments.updateEdge(t)
	}
	return nil
}

// AddrStream returns a channel on which all new addresses discovered for a
// given peer ID will be published.
func (mab *memoryAddrBook) AddrStream(ctx context.Context, t thread.ID, p peer.ID) (<-chan ma.Multiaddr, error) {
	var initial = make([]ma.Multiaddr, 0)
	mab.segments.processAddrs(t, p, false, false, func(amap map[string]*expiringAddr) {
		for _, e := range amap {
			initial = append(initial, e.Addr)
		}
	})
	return mab.subManager.AddrStream(ctx, p, initial)
}

func (mab *memoryAddrBook) AddrsEdge(t thread.ID) (uint64, error) {
	mab.segments.mx.Lock()
	defer mab.segments.mx.Unlock()
	info, found := mab.segments.threads[t]
	if !found {
		return 0, core.ErrThreadNotFound
	}
	return info.edge, nil
}

func (mab *memoryAddrBook) DumpAddrs() (core.DumpAddrBook, error) {
	var dump = core.DumpAddrBook{
		Data: make(map[thread.ID]map[peer.ID][]core.ExpiredAddress, numSegments),
	}

	mab.gcLock.Lock()
	var now = time.Now()

	for _, segment := range mab.segments.segments {
		for tid, logs := range segment.addrs {
			lm, exist := dump.Data[tid]
			if !exist {
				lm = make(map[peer.ID][]core.ExpiredAddress, len(logs))
				dump.Data[tid] = lm
			}

			for lid, addrMap := range logs {
				for _, ap := range addrMap {
					if ap != nil && !ap.ExpiredBy(now) {
						lm[lid] = append(lm[lid], core.ExpiredAddress{
							Addr:    ap.Addr,
							Expires: ap.Expires,
						})
					}
				}
			}
		}
	}
	mab.gcLock.Unlock()
	return dump, nil
}

func (mab *memoryAddrBook) RestoreAddrs(dump core.DumpAddrBook) error {
	if !AllowEmptyRestore && len(dump.Data) == 0 {
		return core.ErrEmptyDump
	}

	mab.gcLock.Lock()
	defer mab.gcLock.Unlock()

	// reset segments
	mab.segments = newAddrSegments()

	var ts = make(map[thread.ID]struct{}, len(dump.Data))
	for tid, peers := range dump.Data {
		for pid, addrs := range peers {
			mab.segments.processAddrs(tid, pid, true, true, func(am map[string]*expiringAddr) {
				var now = time.Now()
				for _, rec := range addrs {
					if rec.Expires.After(now) {
						am[string(rec.Addr.Bytes())] = &expiringAddr{
							Addr:    rec.Addr,
							TTL:     rec.Expires.Sub(now),
							Expires: rec.Expires,
						}
					}
				}
			})
		}
	}

	mab.recomputeEdges(ts)
	return nil
}

type addrSub struct {
	pubch chan ma.Multiaddr
	ctx   context.Context
}

func (s *addrSub) pubAddr(a ma.Multiaddr) {
	select {
	case s.pubch <- a:
	case <-s.ctx.Done():
	}
}

// An abstracted, pub-sub manager for address streams. Extracted from
// memoryAddrBook in order to support additional implementations.
type AddrSubManager struct {
	mu   sync.RWMutex
	subs map[peer.ID][]*addrSub
}

// NewAddrSubManager initializes an AddrSubManager.
func NewAddrSubManager() *AddrSubManager {
	return &AddrSubManager{
		subs: make(map[peer.ID][]*addrSub),
	}
}

// Used internally by the address stream coroutine to remove a subscription
// from the manager.
func (mgr *AddrSubManager) removeSub(p peer.ID, s *addrSub) {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	subs := mgr.subs[p]
	if len(subs) == 1 {
		if subs[0] != s {
			return
		}
		delete(mgr.subs, p)
		return
	}

	for i, v := range subs {
		if v == s {
			subs[i] = subs[len(subs)-1]
			subs[len(subs)-1] = nil
			mgr.subs[p] = subs[:len(subs)-1]
			return
		}
	}
}

// BroadcastAddr broadcasts a new address to all subscribed streams.
func (mgr *AddrSubManager) BroadcastAddr(p peer.ID, addr ma.Multiaddr) {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	if subs, ok := mgr.subs[p]; ok {
		for _, sub := range subs {
			sub.pubAddr(addr)
		}
	}
}

// AddrStream creates a new subscription for a given peer ID, pre-populating the
// channel with any addresses we might already have on file.
func (mgr *AddrSubManager) AddrStream(ctx context.Context, p peer.ID, initial []ma.Multiaddr) (<-chan ma.Multiaddr, error) {
	sub := &addrSub{pubch: make(chan ma.Multiaddr), ctx: ctx}
	out := make(chan ma.Multiaddr)

	mgr.mu.Lock()
	if _, ok := mgr.subs[p]; ok {
		mgr.subs[p] = append(mgr.subs[p], sub)
	} else {
		mgr.subs[p] = []*addrSub{sub}
	}
	mgr.mu.Unlock()

	sort.Sort(addr.AddrList(initial))

	go func(buffer []ma.Multiaddr) {
		defer close(out)

		sent := make(map[string]bool, len(buffer))
		var outch chan ma.Multiaddr

		for _, a := range buffer {
			sent[string(a.Bytes())] = true
		}

		var next ma.Multiaddr
		if len(buffer) > 0 {
			next = buffer[0]
			buffer = buffer[1:]
			outch = out
		}

		for {
			select {
			case outch <- next:
				if len(buffer) > 0 {
					next = buffer[0]
					buffer = buffer[1:]
				} else {
					outch = nil
					next = nil
				}
			case naddr := <-sub.pubch:
				if sent[string(naddr.Bytes())] {
					continue
				}

				sent[string(naddr.Bytes())] = true
				if next == nil {
					next = naddr
					outch = out
				} else {
					buffer = append(buffer, naddr)
				}
			case <-ctx.Done():
				mgr.removeSub(p, sub)
				return
			}
		}

	}(initial)

	return out, nil
}
