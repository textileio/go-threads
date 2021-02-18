package queue

import (
	"context"
	"testing"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/textileio/go-threads/core/thread"
)

func TestOperationQueue(t *testing.T) {
	var (
		q  = newThreadQueue()
		t1 = thread.NewIDV1(thread.Raw, 32)
		t2 = thread.NewIDV1(thread.Raw, 32)
		t3 = thread.NewIDV1(thread.Raw, 32)

		// auxiliary
		eval = func(op PeerCall) { _ = op(nil, "", thread.Undef) }
		val  int
	)

	if _, tid, _, ok := q.Pop(); ok || tid != thread.Undef {
		t.Error("unexpected element in the empty queue")
	}
	if size := q.Size(); size != 0 {
		t.Errorf("size of the empty queue is %d", size)
	}

	operations := []struct {
		tid      thread.ID
		call     PeerCall
		priority int
	}{
		{
			tid:      t1,
			call:     func(context.Context, peer.ID, thread.ID) error { val = 1; return nil },
			priority: 1,
		},
		{
			tid:      t2,
			call:     func(context.Context, peer.ID, thread.ID) error { val = 2; return nil },
			priority: 1,
		},
		{
			tid:      t3,
			call:     func(context.Context, peer.ID, thread.ID) error { val = 3; return nil },
			priority: 1,
		},
	}

	for _, e := range operations {
		if !q.Add(e.tid, e.call, e.priority) {
			t.Error("no indication of adding new thread operation")
		}
	}

	// sequence: t1 -> t2 -> t3
	if size := q.Size(); size != len(operations) {
		t.Errorf("bad queue size, expected: %d, got: %d", len(operations), size)
	}

	// remove t2, sequence: t1 -> t3
	if !q.Remove(t2) || q.Size() != len(operations)-1 {
		t.Error("bad operation removal")
	}

	// evaluate call for t1
	if op, tid, _, ok := q.Pop(); !ok || tid != t1 {
		t.Error("cannot get expected operation")
	} else {
		eval(op)
		expected := 1
		if val != expected {
			t.Errorf("wrong call, expected value: %d, got: %d", expected, val)
		}
	}

	// add all the operations again, but last one should still be in the queue
	for i, e := range operations {
		if q.Add(e.tid, e.call, e.priority) == (i == 2) {
			t.Error("wrong indication of adding thread operation")
		}
	}

	// sequence: t3 -> t1 -> t2, evaluate t3
	if op, tid, _, ok := q.Pop(); !ok || tid != t3 {
		t.Error("cannot get expected operation")
	} else {
		eval(op)
		expected := 3
		if val != expected {
			t.Errorf("wrong call, expected value: %d, got: %d", expected, val)
		}
	}

	// replace t1 call
	if q.Add(t1, func(context.Context, peer.ID, thread.ID) error { val = 123; return nil }, 5) {
		t.Error("replacing lower-priority call should not return true")
	}

	// sequence: t1 (replaced) -> t2
	if op, tid, _, ok := q.Pop(); !ok || tid != t1 {
		t.Error("cannot get expected operation")
	} else {
		eval(op)
		expected := 123
		if val != expected {
			t.Errorf("wrong call, expected value: %d, got: %d", expected, val)
		}
	}

	// sequence: t2
	if op, tid, _, ok := q.Pop(); !ok || tid != t2 {
		t.Error("cannot get expected operation")
	} else {
		eval(op)
		expected := 2
		if val != expected {
			t.Errorf("wrong call, expected value: %d, got: %d", expected, val)
		}
	}

	// sequence: empty
	if _, tid, _, ok := q.Pop(); ok || q.Size() != 0 || tid != thread.Undef {
		t.Error("unexpected operations in the queue")
	}
}
