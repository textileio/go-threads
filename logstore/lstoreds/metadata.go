package lstoreds

import (
	"bytes"
	"encoding/gob"
	"fmt"

	ds "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
	core "github.com/textileio/go-threads/core/logstore"
	"github.com/textileio/go-threads/core/thread"
	"github.com/whyrusleeping/base32"
)

// Thread metadata is stored in db key pattern:
// /thread/meta/<base32 thread id no padding>
var (
	tmetaBase                     = ds.NewKey("/thread/meta")
	_         core.ThreadMetadata = (*dsThreadMetadata)(nil)
)

type dsThreadMetadata struct {
	ds ds.Datastore
}

func NewThreadMetadata(ds ds.Datastore) core.ThreadMetadata {
	return &dsThreadMetadata{
		ds: ds,
	}
}

func (m *dsThreadMetadata) GetInt64(t thread.ID, key string) (*int64, error) {
	var val int64
	err := m.getValue(t, key, &val)
	if err == ds.ErrNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &val, nil
}

func (m *dsThreadMetadata) PutInt64(t thread.ID, key string, val int64) error {
	return m.setValue(t, key, val)
}

func (m *dsThreadMetadata) GetString(t thread.ID, key string) (*string, error) {
	var val string
	err := m.getValue(t, key, &val)
	if err == ds.ErrNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &val, nil
}

func (m *dsThreadMetadata) PutString(t thread.ID, key string, val string) error {
	return m.setValue(t, key, val)
}

func (m *dsThreadMetadata) GetBool(t thread.ID, key string) (*bool, error) {
	var val bool
	err := m.getValue(t, key, &val)
	if err == ds.ErrNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &val, nil
}

func (m *dsThreadMetadata) PutBool(t thread.ID, key string, val bool) error {
	return m.setValue(t, key, val)
}

func (m *dsThreadMetadata) GetBytes(t thread.ID, key string) (*[]byte, error) {
	var val []byte
	err := m.getValue(t, key, &val)
	if err == ds.ErrNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &val, nil
}

func (m *dsThreadMetadata) PutBytes(t thread.ID, key string, val []byte) error {
	return m.setValue(t, key, val)
}

func keyMeta(t thread.ID, k string) ds.Key {
	key := tmetaBase.ChildString(base32.RawStdEncoding.EncodeToString(t.Bytes()))
	key = key.ChildString(k)
	return key
}

func (m *dsThreadMetadata) getValue(t thread.ID, key string, res interface{}) error {
	k := keyMeta(t, key)
	v, err := m.ds.Get(k)
	if err == ds.ErrNotFound {
		return err
	}
	if err != nil {
		return fmt.Errorf("error when getting key from meta datastore: %w", err)
	}
	r := bytes.NewReader(v)
	if err := gob.NewDecoder(r).Decode(res); err != nil {
		return fmt.Errorf("error when deserializing value in datastore for %s: %v", key, err)
	}
	return nil
}

func (m *dsThreadMetadata) setValue(t thread.ID, key string, val interface{}) error {
	k := keyMeta(t, key)

	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(val); err != nil {
		return fmt.Errorf("error when marshaling value: %w", err)
	}
	if err := m.ds.Put(k, buf.Bytes()); err != nil {
		return fmt.Errorf("error when saving marshaled value in datastore: %w", err)
	}
	return nil
}

func (m *dsThreadMetadata) ClearMetadata(t thread.ID) error {
	q := query.Query{
		Prefix:   tmetaBase.ChildString(base32.RawStdEncoding.EncodeToString(t.Bytes())).String(),
		KeysOnly: true,
	}
	results, err := m.ds.Query(q)
	if err != nil {
		return err
	}
	defer results.Close()

	for result := range results.Next() {
		if err := m.ds.Delete(ds.NewKey(result.Key)); err != nil {
			return fmt.Errorf("error when clearing key: %w", err)
		}
	}
	return nil
}
