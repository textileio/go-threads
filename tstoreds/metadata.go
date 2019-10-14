package tstoreds

import (
	"bytes"
	"encoding/gob"
	"fmt"

	ds "github.com/ipfs/go-datastore"
	"github.com/textileio/go-textile-core/thread"
	tstore "github.com/textileio/go-textile-core/threadstore"
	"github.com/whyrusleeping/base32"
)

// Thread metadata is stored in db key pattern:
// /thread/meta/<base32 thread id no padding>
var (
	tmetaBase                       = ds.NewKey("/thread/meta")
	_         tstore.ThreadMetadata = (*dsThreadMetadata)(nil)
)

type dsThreadMetadata struct {
	ds ds.Datastore
}

func NewThreadMetadata(ds ds.Datastore) tstore.ThreadMetadata {
	return &dsThreadMetadata{
		ds: ds,
	}
}

func (ts *dsThreadMetadata) GetInt64(t thread.ID, key string) (*int64, error) {
	var val int64
	err := ts.getValue(t, key, &val)
	if err == ds.ErrNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &val, nil
}

func (ts *dsThreadMetadata) PutInt64(t thread.ID, key string, val int64) error {
	return ts.setValue(t, key, val)
}

func (ts *dsThreadMetadata) GetString(t thread.ID, key string) (*string, error) {
	var val string
	err := ts.getValue(t, key, &val)
	if err == ds.ErrNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &val, nil
}

func (ts *dsThreadMetadata) PutString(t thread.ID, key string, val string) error {
	return ts.setValue(t, key, val)
}

func (ts *dsThreadMetadata) GetBytes(t thread.ID, key string) (*[]byte, error) {
	var val []byte
	err := ts.getValue(t, key, &val)
	if err == ds.ErrNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &val, nil
}

func (ts *dsThreadMetadata) PutBytes(t thread.ID, key string, val []byte) error {
	return ts.setValue(t, key, val)
}

func keyMeta(t thread.ID, k string) ds.Key {
	key := tmetaBase.ChildString(base32.RawStdEncoding.EncodeToString(t.Bytes()))
	key = key.ChildString(k)
	return key
}

func (ts *dsThreadMetadata) getValue(t thread.ID, key string, res interface{}) error {
	k := keyMeta(t, key)
	v, err := ts.ds.Get(k)
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

func (ts *dsThreadMetadata) setValue(t thread.ID, key string, val interface{}) error {
	k := keyMeta(t, key)

	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(val); err != nil {
		return fmt.Errorf("error when marshaling value: %w", err)
	}
	if err := ts.ds.Put(k, buf.Bytes()); err != nil {
		return fmt.Errorf("error when saving marshaled value in datastore: %w", err)
	}

	return nil
}
