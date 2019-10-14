package tstoremem

import (
	"sync"

	"github.com/textileio/go-textile-core/thread"
	tstore "github.com/textileio/go-textile-core/threadstore"
)

var internKeys = map[string]bool{
	"Name": true,
}

type metakey struct {
	id  thread.ID
	key string
}

type memoryThreadMetadata struct {
	ds       map[metakey]interface{}
	dslock   sync.RWMutex
	interned map[string]interface{}
}

func NewThreadMetadata() tstore.ThreadMetadata {
	return &memoryThreadMetadata{
		ds:       make(map[metakey]interface{}),
		interned: make(map[string]interface{}),
	}
}

func (ts *memoryThreadMetadata) PutInt64(t thread.ID, key string, val int64) error {
	ts.putValue(t, key, val)
	return nil
}

func (ts *memoryThreadMetadata) GetInt64(t thread.ID, key string) (*int64, error) {
	val := ts.getValue(t, key).(*int64)
	return val, nil
}

func (ts *memoryThreadMetadata) PutString(t thread.ID, key string, val string) error {
	ts.putValue(t, key, val)
	return nil
}

func (ts *memoryThreadMetadata) GetString(t thread.ID, key string) (*string, error) {
	val := ts.getValue(t, key).(*string)
	return val, nil
}

func (ts *memoryThreadMetadata) PutBytes(t thread.ID, key string, val []byte) error {
	ts.putValue(t, key, val)
	return nil
}

func (ts *memoryThreadMetadata) GetBytes(t thread.ID, key string) (*[]byte, error) {
	val := ts.getValue(t, key).(*[]byte)
	return val, nil
}

func (ts *memoryThreadMetadata) putValue(t thread.ID, key string, val interface{}) {
	ts.dslock.Lock()
	defer ts.dslock.Unlock()
	if vals, ok := val.(string); ok && internKeys[key] {
		if interned, ok := ts.interned[vals]; ok {
			val = interned
		} else {
			ts.interned[vals] = val
		}
	}
	ts.ds[metakey{t, key}] = val
}

func (ts *memoryThreadMetadata) getValue(t thread.ID, key string) interface{} {
	ts.dslock.RLock()
	defer ts.dslock.RUnlock()
	if v, ok := ts.ds[metakey{t, key}]; ok {
		return v
	}
	return nil
}
