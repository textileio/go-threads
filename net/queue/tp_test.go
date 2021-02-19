package queue

import (
	"context"
	"testing"
	"time"

	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/go-threads/test"
)

func TestThreadPacker(t *testing.T) {
	var (
		maxPack     = 3
		timeout     = 1 * time.Second
		ctx, cancel = context.WithCancel(context.Background())
		tp          = NewThreadPacker(ctx, maxPack, timeout)

		pid  = test.GeneratePeerIDs(1)[0]
		tids = make([]thread.ID, 2*maxPack+1)
	)

	for i := 0; i < 2*maxPack+1; i++ {
		tids[i] = thread.NewIDV1(thread.Raw, 32)
	}

	go func() {
		// add: entire pack + another incomplete one
		for i := 0; i < 2*maxPack-1; i++ {
			tp.Add(pid, tids[i])
		}

		// wait until incomplete pack will be flushed
		time.Sleep(timeout + 50*time.Millisecond)

		// add remaining threads
		tp.Add(pid, tids[2*maxPack-1])
		tp.Add(pid, tids[2*maxPack])

		// let last request propagate and stop the packer
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	var packs []ThreadPack
	for p := range tp.Run() {
		packs = append(packs, p)
	}

	var equal = func(p1, p2 ThreadPack) bool {
		if p1.pid != p2.pid || len(p1.tids) != len(p2.tids) {
			return false
		}
		for i := 0; i < len(p1.tids); i++ {
			if p1.tids[i] != p2.tids[i] {
				return false
			}
		}
		return true
	}

	if numPacks := len(packs); numPacks != 3 {
		t.Errorf("wrong number of packs: %d, expected: 3", numPacks)
	}

	if !equal(packs[0], ThreadPack{pid: pid, tids: tids[:3]}) {
		t.Error("unexpected first pack")
	}
	if !equal(packs[1], ThreadPack{pid: pid, tids: tids[3:5]}) {
		t.Error("unexpected second pack")
	}
	if !equal(packs[2], ThreadPack{pid: pid, tids: tids[5:]}) {
		t.Error("unexpected final pack")
	}
}
