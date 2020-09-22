package util

import (
	"sync"

	apipb "github.com/textileio/go-threads/net/api/pb"
	netpb "github.com/textileio/go-threads/net/pb"
)

func RecFromServiceRec(r *netpb.Log_Record) *apipb.Record {
	return &apipb.Record{
		RecordNode: r.RecordNode,
		EventNode:  r.EventNode,
		HeaderNode: r.HeaderNode,
		BodyNode:   r.BodyNode,
	}
}

func RecToServiceRec(r *apipb.Record) *netpb.Log_Record {
	return &netpb.Log_Record{
		RecordNode: r.RecordNode,
		EventNode:  r.EventNode,
		HeaderNode: r.HeaderNode,
		BodyNode:   r.BodyNode,
	}
}

func NewSemaphore(capacity int) *Semaphore {
	return &Semaphore{inner: make(chan struct{}, capacity)}
}

type Semaphore struct {
	inner chan struct{}
}

// Blocking acquire
func (s *Semaphore) Acquire() {
	s.inner <- struct{}{}
}

// Non-blocking acquire
func (s *Semaphore) TryAcquire() bool {
	select {
	case s.inner <- struct{}{}:
		return true
	default:
		return false
	}
}

func (s *Semaphore) Release() {
	select {
	case <-s.inner:
	default:
		panic("thread semaphore inconsistency: release before acquire!")
	}
}

type SemaphoreKey interface {
	Key() string
}

func NewSemaphorePool(semaCap int) *SemaphorePool {
	return &SemaphorePool{ss: make(map[string]*Semaphore), semaCap: semaCap}
}

type SemaphorePool struct {
	ss      map[string]*Semaphore
	semaCap int
	mu      sync.Mutex
}

func (p *SemaphorePool) Get(k SemaphoreKey) *Semaphore {
	var (
		s     *Semaphore
		exist bool
		key   = k.Key()
	)

	p.mu.Lock()
	if s, exist = p.ss[key]; !exist {
		s = NewSemaphore(p.semaCap)
		p.ss[key] = s
	}
	p.mu.Unlock()

	return s
}

func (p *SemaphorePool) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()

	// grab all semaphores and hold
	for _, s := range p.ss {
		s.Acquire()
	}
}
