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

func (m *memoryThreadMetadata) PutInt64(t thread.ID, key string, val int64) error {
	m.putValue(t, key, val)
	return nil
}

func (m *memoryThreadMetadata) GetInt64(t thread.ID, key string) (*int64, error) {
	val, ok := m.getValue(t, key).(int64)
	if !ok {
		return nil, nil
	}
	return &val, nil
}

func (m *memoryThreadMetadata) PutString(t thread.ID, key string, val string) error {
	m.putValue(t, key, val)
	return nil
}

func (m *memoryThreadMetadata) GetString(t thread.ID, key string) (*string, error) {
	val, ok := m.getValue(t, key).(string)
	if !ok {
		return nil, nil
	}
	return &val, nil
}

func (m *memoryThreadMetadata) PutBool(t thread.ID, key string, val bool) error {
	m.putValue(t, key, val)
	return nil
}

func (m *memoryThreadMetadata) GetBool(t thread.ID, key string) (*bool, error) {
	val, ok := m.getValue(t, key).(bool)
	if !ok {
		return nil, nil
	}
	return &val, nil
}

func (m *memoryThreadMetadata) PutBytes(t thread.ID, key string, val []byte) error {
	b := make([]byte, len(val))
	copy(b, val)
	m.putValue(t, key, b)
	return nil
}

func (m *memoryThreadMetadata) GetBytes(t thread.ID, key string) (*[]byte, error) {
	val, ok := m.getValue(t, key).([]byte)
	if !ok {
		return nil, nil
	}
	return &val, nil
}

func (m *memoryThreadMetadata) putValue(t thread.ID, key string, val interface{}) {
	m.dslock.Lock()
	defer m.dslock.Unlock()
	if vals, ok := val.(string); ok && internKeys[key] {
		if interned, ok := m.interned[vals]; ok {
			val = interned
		} else {
			m.interned[vals] = val
		}
	}
	m.ds[metakey{t, key}] = val
}

func (m *memoryThreadMetadata) getValue(t thread.ID, key string) interface{} {
	m.dslock.RLock()
	defer m.dslock.RUnlock()
	if v, ok := m.ds[metakey{t, key}]; ok {
		return v
	}
	return nil
}
