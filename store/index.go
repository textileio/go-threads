// Copyright 2019 Tim Shannon. All rights reserved.
// Use of this source code is governed by the MIT license
// that can be found in the LICENSE file.

package store

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	ds "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// size of iterator keys stored in memory before more are fetched
const (
	iteratorKeyMinCacheSize = 100
)

// ErrUniqueExists is the error thrown when data is being inserted for a unique constraint value that already exists
var (
	indexPrefix     = ds.NewKey("_index")
	ErrUniqueExists = errors.New("Unique constraint violation")
	ErrNotIndexable = errors.New("Value not indexable")
	ErrNoIndexFound = errors.New("No index found")
)

// Indexer is the interface to implement to support Model property indexes
type Indexer interface {
	BaseKey() ds.Key
	Indexes() map[string]Index //[indexname]indexFunc
}

// Index is a function that returns the indexable, encoded bytes of the passed in bytes
type Index struct {
	IndexFunc func(name string, value []byte) (ds.Key, error)
	Unique    bool
}

// IndexConfig stores the configuration for a given Index.
type IndexConfig struct {
	Path   string `json:"path"`
	Unique bool   `json:"unique,omitempty"`
}

// adds an item to the index
func indexAdd(indexer Indexer, tx ds.Txn, key ds.Key, data []byte) error {
	indexes := indexer.Indexes()
	for path, index := range indexes {
		err := indexUpdate(indexer.BaseKey(), path, index, tx, key, data, false)
		if err != nil {
			return err
		}
	}

	return nil
}

// removes an item from the index
// be sure to pass the data from the old record, not the new one
func indexDelete(indexer Indexer, tx ds.Txn, key ds.Key, originalData []byte) error {
	indexes := indexer.Indexes()

	for path, index := range indexes {
		err := indexUpdate(indexer.BaseKey(), path, index, tx, key, originalData, true)
		if err != nil {
			return err
		}
	}

	return nil
}

// adds or removes a specific index on an item
func indexUpdate(baseKey ds.Key, path string, index Index, tx ds.Txn, key ds.Key, value []byte,
	delete bool) error {

	valueKey, err := index.IndexFunc(path, value)
	if valueKey.String() == "" {
		return nil
	}
	if err != nil {
		return err
	}

	indexKey := indexPrefix.Child(baseKey).ChildString(path).Instance(valueKey.String()[1:])

	data, err := tx.Get(indexKey)
	if err != nil && err != ds.ErrNotFound {
		return err
	}

	indexValue := make(keyList, 0)

	if err != ds.ErrNotFound {
		if index.Unique && !delete {
			return ErrUniqueExists
		}
	}

	if data != nil {
		err = DefaultDecode(data, &indexValue)
		if err != nil {
			return err
		}
	}

	if delete {
		indexValue.remove(key)
	} else {
		indexValue.add(key)
	}

	if len(indexValue) == 0 {
		return tx.Delete(indexKey)
	}

	iVal, err := DefaultEncode(indexValue)
	if err != nil {
		return err
	}

	return tx.Put(indexKey, iVal)
}

// keyList is a slice of unique, sorted keys([]byte) such as what an index points to
type keyList [][]byte

func (v *keyList) add(key ds.Key) {
	b := key.Bytes()
	i := sort.Search(len(*v), func(i int) bool {
		return bytes.Compare((*v)[i], b) >= 0
	})

	if i < len(*v) && bytes.Equal((*v)[i], b) {
		// already added
		return
	}

	*v = append(*v, nil)
	copy((*v)[i+1:], (*v)[i:])
	(*v)[i] = b
}

func (v *keyList) remove(key ds.Key) {
	b := key.Bytes()
	i := sort.Search(len(*v), func(i int) bool {
		return bytes.Compare((*v)[i], b) >= 0
	})

	if i < len(*v) {
		copy((*v)[i:], (*v)[i+1:])
		(*v)[len(*v)-1] = nil
		*v = (*v)[:len(*v)-1]
	}
}

func (v *keyList) in(key ds.Key) bool {
	b := key.Bytes()
	i := sort.Search(len(*v), func(i int) bool {
		return bytes.Compare((*v)[i], b) >= 0
	})

	return (i < len(*v) && bytes.Equal((*v)[i], b))
}

type MarshaledResult struct {
	query.Result
	MarshaledValue map[string]interface{}
}

type iterator struct {
	nextKeys func() ([]ds.Key, error)
	txn      ds.Txn
	query    *JSONQuery
	err      error
	keyCache []ds.Key
	iter     query.Results
}

func newIterator(txn ds.Txn, baseKey ds.Key, q *JSONQuery) *iterator {
	i := &iterator{
		txn:   txn,
		query: q,
	}
	// Key field or index not specified, pass thru to base 'iterator'
	if q.Index == "" {
		dsq := query.Query{
			Prefix: baseKey.String(),
		}
		i.iter, i.err = txn.Query(dsq)
		i.nextKeys = func() ([]ds.Key, error) {
			return nil, nil
		}
		return i
	}

	// indexed field, get keys from index
	indexKey := indexPrefix.Child(baseKey).ChildString(q.Index)
	dsq := query.Query{
		Prefix: indexKey.String(),
	}
	i.iter, i.err = txn.Query(dsq)
	first := true
	i.nextKeys = func() ([]ds.Key, error) {
		var nKeys []ds.Key

		for len(nKeys) < iteratorKeyMinCacheSize {
			result, ok := i.iter.NextSync()
			if !ok {
				if first {
					return nil, ErrNoIndexFound
				}
				return nKeys, result.Error
			}
			first = false
			// result.Key contains the indexed value, extract here first
			key := ds.RawKey(result.Key)
			base := key.Type()
			name := key.Name()
			val := gjson.Parse(name).Value()
			if val == nil {
				val = name
			}
			doc, err := sjson.Set("", base, val)
			if err != nil {
				return nil, err
			}
			value := make(map[string]interface{})
			if err := json.Unmarshal([]byte(doc), &value); err != nil {
				return nil, fmt.Errorf("error when unmarshaling query result: %v", err)
			}
			ok, err = q.matchJSON(value)
			if err != nil {
				return nil, fmt.Errorf("error when matching entry with query: %v", err)
			}
			if ok {
				indexValue := make(keyList, 0)
				if err := DefaultDecode(result.Value, &indexValue); err != nil {
					return nil, err
				}
				for _, v := range indexValue {
					nKeys = append(nKeys, ds.RawKey(string(v)))
				}
			}
		}
		return nKeys, nil
	}

	return i
}

// NextSync returns the next key value that matches the iterators criteria
// If there is an error, ok is false and result.Error() will return the error
func (i *iterator) NextSync() (MarshaledResult, bool) {
	if i.query.Index == "" {
		value := MarshaledResult{}
		var ok bool
		for res := range i.iter.Next() {
			val := make(map[string]interface{})
			if value.Error = json.Unmarshal(res.Value, &val); value.Error != nil {
				break
			}
			ok, value.Error = i.query.matchJSON(val)
			if value.Error != nil {
				break
			}
			if ok {
				return MarshaledResult{
					Result:         res,
					MarshaledValue: val,
				}, true
			}
		}
		return value, ok
	}
	if len(i.keyCache) == 0 {
		newKeys, err := i.nextKeys()
		if err != nil {
			return MarshaledResult{
				Result: query.Result{
					Entry: query.Entry{},
					Error: err,
				},
			}, false
		}

		if len(newKeys) == 0 {
			return MarshaledResult{
				Result: query.Result{
					Entry: query.Entry{},
					Error: nil,
				},
			}, false
		}

		i.keyCache = append(i.keyCache, newKeys...)
	}

	key := i.keyCache[0]
	i.keyCache = i.keyCache[1:]

	value, err := i.txn.Get(key)
	if err != nil {
		return MarshaledResult{
			Result: query.Result{
				Entry: query.Entry{},
				Error: err,
			}}, false
	}
	return MarshaledResult{
		Result: query.Result{
			Entry: query.Entry{
				Key:   key.String(),
				Value: value,
			},
			Error: nil,
		}}, true
}

func (i *iterator) Close() {
	i.iter.Close()
}

// Error returns the last error on the iterator
func (i *iterator) Error() error {
	return i.err
}
