package lstoremem

import (
	"sync"

	core "github.com/textileio/go-threads/core/logstore"
	"github.com/textileio/go-threads/core/thread"
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

var _ core.ThreadMetadata = (*memoryThreadMetadata)(nil)

func NewThreadMetadata() core.ThreadMetadata {
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
	val, ok := ts.getValue(t, key).(int64)
	if !ok {
		return nil, nil
	}
	return &val, nil
}

func (ts *memoryThreadMetadata) PutString(t thread.ID, key string, val string) error {
	ts.putValue(t, key, val)
	return nil
}

func (ts *memoryThreadMetadata) GetString(t thread.ID, key string) (*string, error) {
	val, ok := ts.getValue(t, key).(string)
	if !ok {
		return nil, nil
	}
	return &val, nil
}

func (ts *memoryThreadMetadata) PutBytes(t thread.ID, key string, val []byte) error {
	b := make([]byte, len(val))
	copy(b, val)
	ts.putValue(t, key, b)
	return nil
}

func (ts *memoryThreadMetadata) GetBytes(t thread.ID, key string) (*[]byte, error) {
	val, ok := ts.getValue(t, key).([]byte)
	if !ok {
		return nil, nil
	}
	return &val, nil
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
