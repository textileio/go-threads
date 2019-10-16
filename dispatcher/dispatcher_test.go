package dispatcher

import (
	"bytes"
	"encoding/binary"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
	dispatch "github.com/textileio/go-textile-core/dispatcher"
)

type nullReducer struct{}

func (n *nullReducer) Reduce(event dispatch.Event) error {
	return nil
}

type errorReducer struct{}

func (n *errorReducer) Reduce(event dispatch.Event) error {
	return errors.New("error")
}

type slowReducer struct{}

func (n *slowReducer) Reduce(event dispatch.Event) error {
	time.Sleep(2 * time.Second)
	return nil
}

type nullEvent struct {
	Timestamp time.Time
}

func (n *nullEvent) Body() []byte {
	return nil
}

func (n *nullEvent) Time() []byte {
	t := n.Timestamp.UnixNano()
	buf := new(bytes.Buffer)
	// Use big endian to preserve lexicographic sorting
	binary.Write(buf, binary.BigEndian, t)
	return buf.Bytes()
}

func (n *nullEvent) EntityID() string {
	return "null"
}

func (n *nullEvent) Type() string {
	return "null"
}

type ErrorMapDatastore struct {
	*datastore.MapDatastore
}

func (m *ErrorMapDatastore) Put(key datastore.Key, value []byte) error {
	return errors.New("error")
}

// Sanity check
var _ dispatch.Event = (*nullEvent)(nil)

func setup() *Dispatcher {
	return NewDispatcher(datastore.NewMapDatastore())
}

func TestNewDispatcher(t *testing.T) {
	invalid := &Dispatcher{}
	if invalid.store != nil {
		t.Error("nil store expected")
	}
	if invalid.reducers != nil {
		t.Error("nil map expected")
	}
	dispatcher := setup()
	event := &nullEvent{Timestamp: time.Now()}
	dispatcher.Dispatch(event)
	if dispatcher.store == nil {
		t.Error("expected valid store")
	}
	if dispatcher.reducers == nil {
		t.Error("expected valid map")
	}
}

func TestRegister(t *testing.T) {
	dispatcher := setup()
	token := dispatcher.Register(&nullReducer{})
	if token != 1 {
		t.Error("callback registration failed")
	}
	if len(dispatcher.reducers) < 1 {
		t.Error("expected callbacks map to have non-zero length")
	}
}

func TestDeregister(t *testing.T) {
	dispatcher := setup()
	err := dispatcher.Deregister(99)
	if err == nil {
		t.Error("expected to throw error")
	}
	if err.Error() != "token not found" {
		t.Error("expected token now found error")
	}
	token := dispatcher.Register(&nullReducer{})
	dispatcher.Deregister(token)
	if len(dispatcher.reducers) > 0 {
		t.Error("expected reducers map to have zero length")
	}
}

func TestDispatch(t *testing.T) {
	dispatcher := setup()
	event := &nullEvent{Timestamp: time.Now()}
	if err := dispatcher.Dispatch(event); err != nil {
		t.Error("unexpected error in dispatch call")
	}
	dispatcher.Register(&errorReducer{})
	err := dispatcher.Dispatch(event)
	if errs, ok := err.(*multierror.Error); ok {
		if len(errs.Errors) != 1 {
			t.Error("should be one error")
		}
		if errs.Errors[0].Error() != "warning error" {
			t.Errorf("`%s` should be `warning error`", err)
		}
	} else {
		t.Error("expected error in dispatch call")
	}
}

func TestPersistError(t *testing.T) {
	dispatcher := NewDispatcher(&ErrorMapDatastore{
		datastore.NewMapDatastore(),
	})
	event := &nullEvent{Timestamp: time.Now()}
	dispatcher.Register(&errorReducer{})
	dispatcher.Register(&errorReducer{})
	err := dispatcher.Dispatch(event)
	if err != ErrPersistence {
		t.Error("expected persistence error")
	}
}

func TestLock(t *testing.T) {
	dispatcher := setup()
	dispatcher.Register(&slowReducer{})
	event := &nullEvent{Timestamp: time.Now()}
	t1 := time.Now()
	wg := &sync.WaitGroup{}
	go func() {
		wg.Add(1)
		defer wg.Done()
		if err := dispatcher.Dispatch(event); err != nil {
			t.Error("unexpected error in dispatch call")
		}
	}()
	if err := dispatcher.Dispatch(event); err != nil {
		t.Error("unexpected error in dispatch call")
	}
	wg.Wait()
	t2 := time.Now()
	if t2.Sub(t1) < (4 * time.Second) {
		t.Error("reached this point too soon")
	}
}

func TestQuery(t *testing.T) {
	dispatcher := setup()
	var events []dispatch.Event
	n := 100
	for i := 1; i <= n; i++ {
		events = append(events, &nullEvent{Timestamp: time.Now()})
		time.Sleep(time.Millisecond)
	}
	for _, event := range events {
		if err := dispatcher.Dispatch(event); err != nil {
			t.Error("unexpected error in dispatch call")
		}
	}
	results, err := dispatcher.Query(query.Query{
		Orders: []query.Order{query.OrderByKey{}},
	})
	if err != nil {
		t.Errorf("unexpected error: %s", err.Error())
	}
	if rest, _ := results.Rest(); len(rest) != n {
		t.Errorf("expected %d result, got %d", n, len(rest))
	}
}
