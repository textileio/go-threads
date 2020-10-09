package lstoremem

import (
	"context"
	"sort"
	"sync"
	"time"

	logging "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-peerstore/addr"
	ma "github.com/multiformats/go-multiaddr"
	core "github.com/textileio/go-threads/core/logstore"
	"github.com/textileio/go-threads/core/thread"
)

var log = logging.Logger("logstore")

type expiringAddr struct {
	Addr    ma.Multiaddr
	TTL     time.Duration
	Expires time.Time
}

func (e *expiringAddr) ExpiredBy(t time.Time) bool {
	return t.After(e.Expires)
}

type addrSegments [256]*addrSegment

type addrSegment struct {
	sync.RWMutex

	// Use pointers to save memory. Maps always leave some fraction of their
	// space unused. storing the *values* directly in the map will
	// drastically increase the space waste. In our case, by 6x.
	addrs map[thread.ID]map[peer.ID]map[string]*expiringAddr
}

func (s *addrSegment) getAddrs(t thread.ID, p peer.ID) (map[string]*expiringAddr, bool) {
	lmap, found := s.addrs[t]
	if lmap == nil {
		return nil, found
	}
	amap, found := lmap[p]
	return amap, found
}

func (s *addrSegments) get(p peer.ID) *addrSegment {
	return s[p[len(p)-1]]
}

// memoryAddrBook manages addresses.
type memoryAddrBook struct {
	segments addrSegments

	ctx    context.Context
	cancel func()
	gcLock sync.Mutex

	subManager *AddrSubManager
}

var _ core.AddrBook = (*memoryAddrBook)(nil)

func NewAddrBook() core.AddrBook {
	ctx, cancel := context.WithCancel(context.Background())

	ab := &memoryAddrBook{
		segments: func() (ret addrSegments) {
			for i := range ret {
				ret[i] = &addrSegment{addrs: make(map[thread.ID]map[peer.ID]map[string]*expiringAddr)}
			}
			return ret
		}(),
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
			mab.gc()

		case <-mab.ctx.Done():
			return
		}
	}
}

func (mab *memoryAddrBook) Close() error {
	mab.cancel()
	return nil
}

// gc garbage collects the in-memory address book.
func (mab *memoryAddrBook) gc() {
	mab.gcLock.Lock()
	defer mab.gcLock.Unlock()

	var now = time.Now()
	for _, s := range mab.segments {
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
		}
		s.Unlock()
	}
}

func (mab *memoryAddrBook) LogsWithAddrs(t thread.ID) (peer.IDSlice, error) {
	var pids peer.IDSlice
	for _, s := range mab.segments {
		s.RLock()
		for pid := range s.addrs[t] {
			pids = append(pids, pid)
		}
		s.RUnlock()
	}
	return pids, nil
}

func (mab *memoryAddrBook) ThreadsFromAddrs() (thread.IDSlice, error) {
	ts := make(map[thread.ID]struct{})
	for _, s := range mab.segments {
		s.RLock()
		for tid := range s.addrs {
			ts[tid] = struct{}{}
		}
		s.RUnlock()
	}

	var tids thread.IDSlice
	for t := range ts {
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

	s := mab.segments.get(p)
	s.Lock()
	defer s.Unlock()

	amap, _ := s.getAddrs(t, p)
	if amap == nil {
		if s.addrs[t] == nil {
			s.addrs[t] = make(map[peer.ID]map[string]*expiringAddr, 1)
		}
		amap = make(map[string]*expiringAddr, len(addrs))
		s.addrs[t][p] = amap
	}
	exp := time.Now().Add(ttl)
	for _, a := range addrs {
		if a == nil {
			log.Warnf("was passed nil multiaddr for %s", p)
			continue
		}
		// It's cheaper than using String(), but unfortunately it will
		// copy bytes on conversion anyway just to ensure that string
		// contents are immutable and independent from the original
		// buffer. Zero-copy conversion could be done with unsafe but
		// in this case it's not worth bothering with.
		asStr := string(a.Bytes())
		x, found := amap[asStr]
		if !found {
			// not found, save and announce it.
			amap[asStr] = &expiringAddr{Addr: a, Expires: exp, TTL: ttl}
			mab.subManager.BroadcastAddr(p, a)
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
	return nil
}

// SetAddr calls mgr.SetAddrs(t, p, addr, ttl)
func (mab *memoryAddrBook) SetAddr(t thread.ID, p peer.ID, addr ma.Multiaddr, ttl time.Duration) error {
	return mab.SetAddrs(t, p, []ma.Multiaddr{addr}, ttl)
}

// SetAddrs sets the ttl on addresses. This clears any TTL there previously.
// This is used when we receive the best estimate of the validity of an address.
func (mab *memoryAddrBook) SetAddrs(t thread.ID, p peer.ID, addrs []ma.Multiaddr, ttl time.Duration) error {
	s := mab.segments.get(p)
	s.Lock()
	defer s.Unlock()

	amap, _ := s.getAddrs(t, p)
	if amap == nil {
		if s.addrs[t] == nil {
			s.addrs[t] = make(map[peer.ID]map[string]*expiringAddr, 1)
		}
		amap = make(map[string]*expiringAddr, len(addrs))
		s.addrs[t][p] = amap
	}

	exp := time.Now().Add(ttl)
	for _, a := range addrs {
		if a == nil {
			log.Warnf("was passed nil multiaddr for %s", p)
			continue
		}

		// re-set all of them for new ttl.
		aBytes := a.Bytes()
		if ttl > 0 {
			amap[string(aBytes)] = &expiringAddr{Addr: a, Expires: exp, TTL: ttl}
			mab.subManager.BroadcastAddr(p, a)
		} else {
			delete(amap, string(aBytes))
		}
	}
	return nil
}

// UpdateAddrs updates the addresses associated with the given peer that have
// the given oldTTL to have the given newTTL.
func (mab *memoryAddrBook) UpdateAddrs(t thread.ID, p peer.ID, oldTTL time.Duration, newTTL time.Duration) error {
	s := mab.segments.get(p)
	s.Lock()
	defer s.Unlock()

	amap, found := s.getAddrs(t, p)
	if !found {
		return nil
	}

	exp := time.Now().Add(newTTL)
	for k, a := range amap {
		if oldTTL == a.TTL {
			a.TTL = newTTL
			a.Expires = exp
			amap[k] = a
		}
	}
	return nil
}

// Addrs returns all known (and valid) addresses for a given log
func (mab *memoryAddrBook) Addrs(t thread.ID, p peer.ID) ([]ma.Multiaddr, error) {
	s := mab.segments.get(p)
	s.RLock()
	defer s.RUnlock()

	amap, found := s.getAddrs(t, p)
	if !found {
		return nil, nil
	}

	now := time.Now()
	good := make([]ma.Multiaddr, 0, len(amap))
	for _, m := range amap {
		if !m.ExpiredBy(now) {
			good = append(good, m.Addr)
		}
	}

	return good, nil
}

// ClearAddrs removes all previously stored addresses
func (mab *memoryAddrBook) ClearAddrs(t thread.ID, p peer.ID) error {
	s := mab.segments.get(p)
	s.Lock()
	defer s.Unlock()

	lmap := s.addrs[t]
	if lmap != nil {
		delete(lmap, p)
		if len(lmap) == 0 {
			delete(s.addrs, t)
		}
	}
	return nil
}

// AddrStream returns a channel on which all new addresses discovered for a
// given peer ID will be published.
func (mab *memoryAddrBook) AddrStream(ctx context.Context, t thread.ID, p peer.ID) (<-chan ma.Multiaddr, error) {
	s := mab.segments.get(p)
	s.RLock()
	defer s.RUnlock()

	baseaddrslice, _ := s.getAddrs(t, p)
	initial := make([]ma.Multiaddr, 0, len(baseaddrslice))
	for _, a := range baseaddrslice {
		initial = append(initial, a.Addr)
	}

	return mab.subManager.AddrStream(ctx, p, initial)
}

func (mab *memoryAddrBook) DumpAddrs() (core.DumpAddrBook, error) {
	var dump = core.DumpAddrBook{
		Data: make(map[thread.ID]map[peer.ID][]core.ExpiredAddress, 256),
	}

	mab.gcLock.Lock()
	var now = time.Now()

	for _, segment := range mab.segments {
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
	mab.gcLock.Lock()
	defer mab.gcLock.Unlock()

	// reset segments
	for i := range mab.segments {
		mab.segments[i] = &addrSegment{
			addrs: make(map[thread.ID]map[peer.ID]map[string]*expiringAddr, len(mab.segments[i].addrs)),
		}
	}

	var now = time.Now()
	for tid, logs := range dump.Data {
		for lid, addrs := range logs {
			s := mab.segments.get(lid)
			am, _ := s.getAddrs(tid, lid)
			if am == nil {
				if s.addrs[tid] == nil {
					s.addrs[tid] = make(map[peer.ID]map[string]*expiringAddr, 1)
				}
				am = make(map[string]*expiringAddr, len(addrs))
				s.addrs[tid][lid] = am
			}

			for _, rec := range addrs {
				if rec.Expires.After(now) {
					am[string(rec.Addr.Bytes())] = &expiringAddr{
						Addr:    rec.Addr,
						TTL:     rec.Expires.Sub(now),
						Expires: rec.Expires,
					}
				}
			}
		}
	}
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
