package test

import (
	"context"
	"fmt"
	"sort"
	"testing"

	pstore "github.com/libp2p/go-libp2p-core/peerstore"
	"github.com/textileio/go-textile-core/thread"
	tstore "github.com/textileio/go-textile-core/threadstore"
)

var threadstoreBenchmarks = map[string]func(tstore.Threadstore, chan *logpair) func(*testing.B){
	"AddAddrs": benchmarkAddAddrs,
	"SetAddrs": benchmarkSetAddrs,
	"GetAddrs": benchmarkGetAddrs,
	// The in-between get allows us to benchmark the read-through cache.
	"AddGetAndClearAddrs": benchmarkAddGetAndClearAddrs,
	// Calls LogsWithAddr on a threadstore with 1000 logs.
	"Get1000LogsWithAddrs": benchmarkGet1000LogsWithAddrs,
}

func BenchmarkThreadstore(b *testing.B, factory ThreadstoreFactory, variant string) {
	// Parameterises benchmarks to tackle logs with 1, 10, 100 multiaddrs.
	params := []struct {
		n  int
		ch chan *logpair
	}{
		{1, make(chan *logpair, 100)},
		{10, make(chan *logpair, 100)},
		{100, make(chan *logpair, 100)},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start all test log producing goroutines, where each produces logs with as many
	// multiaddrs as the n field in the param struct.
	for _, p := range params {
		go AddressProducer(ctx, b, p.ch, p.n)
	}

	// So tests are always run in the same order.
	ordernames := make([]string, 0, len(threadstoreBenchmarks))
	for name := range threadstoreBenchmarks {
		ordernames = append(ordernames, name)
	}
	sort.Strings(ordernames)

	for _, name := range ordernames {
		bench := threadstoreBenchmarks[name]
		for _, p := range params {
			// Create a new threadstore.
			ts, closeFunc := factory()

			// Run the test.
			b.Run(fmt.Sprintf("%s-%dAddrs-%s", name, p.n, variant), bench(ts, p.ch))

			// Cleanup.
			if closeFunc != nil {
				closeFunc()
			}
		}
	}
}

func benchmarkAddAddrs(ts tstore.Threadstore, addrs chan *logpair) func(*testing.B) {
	return func(b *testing.B) {
		tid := thread.NewIDV1(thread.Raw, 24)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			pp := <-addrs
			ts.AddAddrs(tid, pp.ID, pp.Addr, pstore.PermanentAddrTTL)
		}
	}
}

func benchmarkSetAddrs(ts tstore.Threadstore, addrs chan *logpair) func(*testing.B) {
	return func(b *testing.B) {
		tid := thread.NewIDV1(thread.Raw, 24)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			pp := <-addrs
			ts.SetAddrs(tid, pp.ID, pp.Addr, pstore.PermanentAddrTTL)
		}
	}
}

func benchmarkGetAddrs(ts tstore.Threadstore, addrs chan *logpair) func(*testing.B) {
	return func(b *testing.B) {
		tid := thread.NewIDV1(thread.Raw, 24)
		pp := <-addrs
		ts.SetAddrs(tid, pp.ID, pp.Addr, pstore.PermanentAddrTTL)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = ts.Addrs(tid, pp.ID)
		}
	}
}

func benchmarkAddGetAndClearAddrs(ts tstore.Threadstore, addrs chan *logpair) func(*testing.B) {
	return func(b *testing.B) {
		tid := thread.NewIDV1(thread.Raw, 24)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			pp := <-addrs
			ts.AddAddrs(tid, pp.ID, pp.Addr, pstore.PermanentAddrTTL)
			ts.Addrs(tid, pp.ID)
			ts.ClearAddrs(tid, pp.ID)
		}
	}
}

func benchmarkGet1000LogsWithAddrs(ts tstore.Threadstore, addrs chan *logpair) func(*testing.B) {
	return func(b *testing.B) {
		tid := thread.NewIDV1(thread.Raw, 24)
		var logs = make([]*logpair, 1000)
		for i := range logs {
			pp := <-addrs
			ts.AddAddrs(tid, pp.ID, pp.Addr, pstore.PermanentAddrTTL)
			logs[i] = pp
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = ts.LogsWithAddrs(tid)
		}
	}
}
