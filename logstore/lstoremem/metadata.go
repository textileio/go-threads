package lstoremem

import (
	"fmt"
	"sync"

	core "github.com/textileio/go-threads/core/logstore"
	"github.com/textileio/go-threads/core/thread"
)

type memoryThreadMetadata struct {
	ds     map[core.MetadataKey]interface{}
	dslock sync.RWMutex
}

var _ core.ThreadMetadata = (*memoryThreadMetadata)(nil)

func NewThreadMetadata() core.ThreadMetadata {
	return &memoryThreadMetadata{
		ds: make(map[core.MetadataKey]interface{}),
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
	m.ds[core.MetadataKey{T: t, K: key}] = val
}

func (m *memoryThreadMetadata) getValue(t thread.ID, key string) interface{} {
	m.dslock.RLock()
	defer m.dslock.RUnlock()
	if v, ok := m.ds[core.MetadataKey{T: t, K: key}]; ok {
		return v
	}
	return nil
}

func (m *memoryThreadMetadata) ClearMetadata(t thread.ID) error {
	m.dslock.Lock()
	defer m.dslock.Unlock()
	for k := range m.ds {
		if k.T.Equals(t) {
			delete(m.ds, k)
		}
	}
	return nil
}

func (m *memoryThreadMetadata) DumpMeta() (core.DumpMetadata, error) {
	m.dslock.RLock()
	defer m.dslock.RUnlock()

	var (
		dump    core.DumpMetadata
		vInt64  = make(map[core.MetadataKey]int64)
		vBool   = make(map[core.MetadataKey]bool)
		vString = make(map[core.MetadataKey]string)
		vBytes  = make(map[core.MetadataKey][]byte)
	)

	for mk, value := range m.ds {
		switch v := value.(type) {
		case bool:
			vBool[mk] = v
		case int64:
			vInt64[mk] = v
		case string:
			vString[mk] = v
		case []byte:
			vBytes[mk] = v
		default:
			return dump, fmt.Errorf("unsupported value type %T, key: %v, value: %v", value, mk, value)
		}
	}

	dump.Data.Bool = vBool
	dump.Data.Int64 = vInt64
	dump.Data.String = vString
	dump.Data.Bytes = vBytes
	return dump, nil
}

func (m *memoryThreadMetadata) RestoreMeta(dump core.DumpMetadata) error {
	var dataLen = len(dump.Data.Bool) +
		len(dump.Data.Int64) +
		len(dump.Data.String) +
		len(dump.Data.Bytes)
	if dataLen == 0 {
		return core.ErrEmptyDump
	}

	m.dslock.Lock()
	defer m.dslock.Unlock()

	// clear local data
	m.ds = make(map[core.MetadataKey]interface{}, dataLen)

	// replace with dump
	for mk, val := range dump.Data.Bool {
		m.ds[mk] = val
	}
	for mk, val := range dump.Data.Int64 {
		m.ds[mk] = val
	}
	for mk, val := range dump.Data.String {
		m.ds[mk] = val
	}
	for mk, val := range dump.Data.Bytes {
		m.ds[mk] = val
	}

	return nil
}
