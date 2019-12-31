package broadcast

import (
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/go-multierror"
)

const (
	N       = 3
	testStr = "Test"
	timeout = time.Second
)

type ListenFunc func(int, *Broadcaster, *sync.WaitGroup)

func setupN(f ListenFunc) (*Broadcaster, *sync.WaitGroup) {
	var b Broadcaster
	var wg sync.WaitGroup
	wg.Add(N)
	for i := 0; i < N; i++ {
		go f(i, &b, &wg)
	}
	wg.Wait()
	return &b, &wg
}

func TestSend(t *testing.T) {
	b, wg := setupN(func(i int, b *Broadcaster, wg *sync.WaitGroup) {
		l := b.Listen()
		wg.Done()
		select {
		case v := <-l.Channel():
			if v.(string) != testStr {
				t.Error("bad value received")
			}
		case <-time.After(timeout):
			t.Error("receive timed out")
		}
		wg.Done()
	})
	wg.Add(N)
	_ = b.Send(testStr)
	wg.Wait()
}

func TestSendError(t *testing.T) {
	var b Broadcaster
	// Register listeners, but do not consume
	b.Listen()
	b.Listen()
	b.Listen()
	if err := b.Send(testStr); err == nil {
		t.Error("should error when no consumers")
		if multi, ok := err.(*multierror.Error); ok {
			if len(multi.Errors) != 3 {
				t.Error("expected 3 errors")
			}
		} else {
			t.Error("expected a multi-error")
		}
	}
	if err := b.Send(testStr); err == nil {
		t.Error("should error when no consumers")
	}
}

func TestListenAndSendOnClosed(t *testing.T) {
	var b = NewBroadcaster(5)
	b.Discard()
	b.Listen()
	err := b.Send(testStr)
	if err != ErrClosedChannel {
		if err != nil {
			t.Errorf("Test should raise closed channel error: %s", err.Error())
		}
	}
	if err == nil || err.Error() != "send after close" {
		t.Error("Test should raise `send after close`")
	}
}

func TestListenAndSendOnCloseWithTimeout(t *testing.T) {
	var b = NewBroadcaster(5)
	b.Discard()
	b.Listen()
	err := b.SendWithTimeout(testStr, 0)
	if err != ErrClosedChannel {
		if err != nil {
			t.Errorf("Test should raise closed channel error: %s", err.Error())
		}
	}
	if err == nil || err.Error() != "send after close" {
		t.Error("Test should raise `send after close`")
	}
}

func TestSendWithTimeout(t *testing.T) {
	var b Broadcaster
	var wg sync.WaitGroup
	wg.Add(1)
	go func(i int, b *Broadcaster, wg *sync.WaitGroup) {
		l := b.Listen()
		wg.Done()
		time.Sleep(time.Second)
		select {
		case v := <-l.Channel():
			if v.(string) != testStr {
				t.Error("bad value received")
			}
		case <-time.After(timeout):
			t.Error("receive timed out")
		}
		wg.Done()
	}(1, &b, &wg)
	wg.Wait()
	wg.Add(1)
	if err := b.Send(testStr); err == nil {
		t.Error("should error")
	}
	if err := b.SendWithTimeout(testStr, 0); err == nil {
		t.Error("should error within 1 second")
	}
	if err := b.SendWithTimeout(testStr, 2*time.Second); err != nil {
		t.Error("should not error within 2 seconds")
	}
	wg.Wait()
}

func TestBroadcasterClose(t *testing.T) {
	b, wg := setupN(func(i int, b *Broadcaster, wg *sync.WaitGroup) {
		l := b.Listen()
		wg.Done()
		select {
		case _, ok := <-l.Channel():
			if ok {
				t.Error("receive after close")
			}
		case <-time.After(timeout):
			t.Error("receive timed out")
		}
		wg.Done()
	})
	wg.Add(N)
	b.Discard()
	wg.Wait()
}

func TestListenerClose(t *testing.T) {
	b, wg := setupN(func(i int, b *Broadcaster, wg *sync.WaitGroup) {
		l := b.Listen()
		if i == 0 {
			l.Discard()
		}
		wg.Done()
		select {
		case <-l.Channel():
			if i == 0 {
				t.Error("receive after close")
			}
		case <-time.After(timeout):
			if i != 0 {
				t.Error("receive timed out")
			}
		}
		wg.Done()
	})
	wg.Add(N)
	_ = b.Send(testStr)
	wg.Wait()
}
