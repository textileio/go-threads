// Copyright 2019 Tim Shannon. All rights reserved.
// Use of this source code is governed by the MIT license
// that can be found in the LICENSE file.

package store

import (
	"bytes"
	"errors"
	"sort"

	ds "github.com/ipfs/go-datastore"
)

// size of iterator keys stored in memory before more are fetched
const (
	iteratorKeyMinCacheSize = 100
)

// ErrUniqueExists is the error thrown when data is being inserted for a unique constraint value that already exists
var (
	indexPrefix     = ds.NewKey("_index")
	ErrUniqueExists = errors.New("Unique constraint violation")
	// ErrNotIndexed indicates that the given value is not indexable
	ErrNotIndexable = errors.New("Value not indexable")
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

	indexKey := indexPrefix.Child(baseKey).ChildString(path).Child(valueKey)

	data, err := tx.Get(indexKey)
	if err != nil && err != ds.ErrNotFound {
		return err
	}

	indexValue := make(keyList, 0)

	if err != ds.ErrNotFound {
		if index.Unique && !delete {
			return ErrUniqueExists
		}
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

// func indexExists(it query.Results, typeName, indexName string) bool {
// 	iPrefix := indexKeyPrefix(typeName, indexName)
// 	tPrefix := typePrefix(typeName)
// 	// test if any data exists for type
// 	it.Seek(tPrefix)
// 	if !it.ValidForPrefix(tPrefix) {
// 		// store is empty for this data type so the index could possibly exist
// 		// we don't want to fail on a "bad index" because they could simply be running a query against
// 		// an empty dataset
// 		return true
// 	}

// 	// test if an index exists
// 	it.Seek(iPrefix)
// 	if it.ValidForPrefix(iPrefix) {
// 		return true
// 	}

// 	return false
// }

// type iterator struct {
// 	keyCache []ds.Key
// 	nextKeys func(query.Results) ([]ds.Key, error)
// 	iter     query.Results
// 	bookmark *iterBookmark
// 	lastSeek []byte
// 	tx       ds.Txn
// 	err      error
// }

// // iterBookmark stores a seek location in a specific iterator
// // so that a single RW iterator can be shared within a single transaction
// type iterBookmark struct {
// 	iter    query.Results
// 	seekKey []byte
// }

// func newIterator(tx ds.Txn, typeName string, q *Query, bookmark *iterBookmark) *iterator {
// 	i := &iterator{
// 		tx: tx,
// 	}

// 	if bookmark != nil {
// 		i.iter = bookmark.iter
// 	} else {
// 		i.iter = tx.Query()
// 	}

// 	var prefix []byte

// 	if q.index != "" {
// 		q.badIndex = !indexExists(i.iter, typeName, q.index)
// 	}

// 	criteria := query.fieldCriteria[q.index]
// 	if hasMatchFunc(criteria) {
// 		// can't use indexes on matchFuncs as the entire record isn't available for testing in the passed
// 		// in function
// 		criteria = nil
// 	}

// 	// Key field or index not specified - test key against criteria (if it exists) or return everything
// 	if q.index == "" || len(criteria) == 0 {
// 		prefix = typePrefix(typeName)
// 		i.iter.Seek(prefix)
// 		i.nextKeys = func(iter query.Results) ([]ds.Key, error) {
// 			var nKeys [][]byte

// 			for len(nKeys) < iteratorKeyMinCacheSize {
// 				if !iter.ValidForPrefix(prefix) {
// 					return nKeys, nil
// 				}

// 				item := iter.Item()
// 				key := item.KeyCopy(nil)
// 				var ok bool
// 				if len(criteria) == 0 {
// 					// nothing to check return key for value testing
// 					ok = true
// 				} else {

// 					val := reflect.New(q.dataType)

// 					err := item.Value(func(v []byte) error {
// 						return decode(v, val.Interface())
// 					})
// 					if err != nil {
// 						return nil, err
// 					}

// 					ok, err = matchesAllCriteria(criteria, key, true, typeName, val.Interface())
// 					if err != nil {
// 						return nil, err
// 					}
// 				}

// 				if ok {
// 					nKeys = append(nKeys, key)

// 				}
// 				i.lastSeek = key
// 				iter.Next()
// 			}
// 			return nKeys, nil
// 		}

// 		return i
// 	}

// 	// indexed field, get keys from index
// 	prefix = indexKeyPrefix(typeName, q.index)
// 	i.iter.Seek(prefix)
// 	i.nextKeys = func(iter query.Results) ([][]byte, error) {
// 		var nKeys [][]byte

// 		for len(nKeys) < iteratorKeyMinCacheSize {
// 			if !iter.ValidForPrefix(prefix) {
// 				return nKeys, nil
// 			}
// 			item, has := iter.NextSync()
// 			if !has {
// 				return nKeys, nil
// 			}
// 			key := item.KeyCopy(nil)
// 			// no currentRow on indexes as it refers to multiple rows
// 			// remove index prefix for matching
// 			ok, err := matchesAllCriteria(criteria, key[len(prefix):], true, "", nil)
// 			if err != nil {
// 				return nil, err
// 			}

// 			if ok {
// 				item.Value(func(v []byte) error {
// 					// append the slice of keys stored in the index
// 					var keys = make(keyList, 0)
// 					err := decode(v, &keys)
// 					if err != nil {
// 						return err
// 					}

// 					nKeys = append(nKeys, [][]byte(keys)...)
// 					return nil
// 				})
// 			}

// 			i.lastSeek = key
// 			iter.Next()

// 		}
// 		return nKeys, nil

// 	}

// 	return i
// }

// func (i *iterator) createBookmark() *iterBookmark {
// 	return &iterBookmark{
// 		iter:    i.iter,
// 		seekKey: i.lastSeek,
// 	}
// }

// // Next returns the next key value that matches the iterators criteria
// // If no more kv's are available the return nil, if there is an error, they return nil
// // and iterator.Error() will return the error
// func (i *iterator) Next() (key ds.Key, value []byte) {
// 	if i.err != nil {
// 		return nil, nil
// 	}

// 	if len(i.keyCache) == 0 {
// 		newKeys, err := i.nextKeys(i.iter)
// 		if err != nil {
// 			i.err = err
// 			return nil, nil
// 		}

// 		if len(newKeys) == 0 {
// 			return nil, nil
// 		}

// 		i.keyCache = append(i.keyCache, newKeys...)
// 	}

// 	key = i.keyCache[0]
// 	i.keyCache = i.keyCache[1:]

// 	value, err := i.tx.Get(key)
// 	if err != nil {
// 		i.err = err
// 		return nil, nil
// 	}
// 	return
// }

// // Error returns the last error, iterator.Next() will not continue if there is an error present
// func (i *iterator) Error() error {
// 	return i.err
// }

// func (i *iterator) Close() {
// 	if i.bookmark != nil {
// 		// @todo: We need to seek to the bookmark key?
// 		// i.iter.Seek(i.bookmark.seekKey)
// 		return
// 	}
// 	i.iter.Close()
// }
