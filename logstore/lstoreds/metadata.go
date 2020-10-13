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
	return m.clearKeys(tmetaBase.ChildString(base32.RawStdEncoding.EncodeToString(t.Bytes())).String())
}

func (m *dsThreadMetadata) DumpMeta() (core.DumpMetadata, error) {
	var (
		vBool   = make(map[core.MetadataKey]bool)
		vInt64  = make(map[core.MetadataKey]int64)
		vString = make(map[core.MetadataKey]string)
		vBytes  = make(map[core.MetadataKey][]byte)

		buff bytes.Buffer
		dump core.DumpMetadata
		dec  = gob.NewDecoder(&buff)
	)

	results, err := m.ds.Query(query.Query{Prefix: tmetaBase.String()})
	if err != nil {
		return dump, err
	}
	defer results.Close()

	for entry := range results.Next() {
		kns := ds.RawKey(entry.Key).Namespaces()
		if len(kns) < 4 {
			return dump, fmt.Errorf("bad metabook key detected: %s", entry.Key)
		}

		ts, key := kns[2], kns[3]
		tid, err := parseThreadID(ts)
		if err != nil {
			return dump, fmt.Errorf("cannot parse thread ID %s: %w", ts, err)
		}

		var mk = core.MetadataKey{T: tid, K: key}

		// we (kinda) don't know about type on the wire, so try to decode into every known type
		{
			var value int64
			buff.Reset()
			buff.Write(entry.Value)
			if dec.Decode(&value) == nil {
				vInt64[mk] = value
				continue
			}
		}
		{
			var value bool
			buff.Reset()
			buff.Write(entry.Value)
			if dec.Decode(&value) == nil {
				vBool[mk] = value
				continue
			}
		}
		{
			var value string
			buff.Reset()
			buff.Write(entry.Value)
			if dec.Decode(&value) == nil {
				vString[mk] = value
				continue
			}
		}
		{
			var value []byte
			buff.Reset()
			buff.Write(entry.Value)
			if dec.Decode(&value) == nil {
				vBytes[mk] = value
				continue
			}
		}

		return dump, fmt.Errorf("cannot decode value at key: %v, value: %v", mk, entry.Value)
	}

	dump.Data.Bool = vBool
	dump.Data.Int64 = vInt64
	dump.Data.String = vString
	dump.Data.Bytes = vBytes
	return dump, nil
}

func (m *dsThreadMetadata) RestoreMeta(dump core.DumpMetadata) error {
	var dataLen = len(dump.Data.Bool) +
		len(dump.Data.Int64) +
		len(dump.Data.String) +
		len(dump.Data.Bytes)
	if !AllowEmptyRestore && dataLen == 0 {
		return core.ErrEmptyDump
	}

	if err := m.clearKeys(tmetaBase.String()); err != nil {
		return err
	}

	for mk, val := range dump.Data.Bool {
		if err := m.setValue(mk.T, mk.K, val); err != nil {
			return err
		}
	}
	for mk, val := range dump.Data.Int64 {
		if err := m.setValue(mk.T, mk.K, val); err != nil {
			return err
		}
	}
	for mk, val := range dump.Data.String {
		if err := m.setValue(mk.T, mk.K, val); err != nil {
			return err
		}
	}
	for mk, val := range dump.Data.Bytes {
		if err := m.setValue(mk.T, mk.K, val); err != nil {
			return err
		}
	}

	return nil
}

func (m *dsThreadMetadata) clearKeys(prefix string) error {
	results, err := m.ds.Query(query.Query{Prefix: prefix, KeysOnly: true})
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
