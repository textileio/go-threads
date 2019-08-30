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

func (ts *memoryThreadMetadata) PutMeta(t thread.ID, key string, val interface{}) error {
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
	return nil
}

func (ts *memoryThreadMetadata) GetMeta(t thread.ID, key string) (interface{}, error) {
	ts.dslock.RLock()
	defer ts.dslock.RUnlock()
	i, ok := ts.ds[metakey{t, key}]
	if !ok {
		return nil, tstore.ErrNotFound
	}
	return i, nil
}
