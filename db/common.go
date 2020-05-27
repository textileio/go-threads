package db

import (
	"errors"
	"sync"
	"time"

	datastore "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
	core "github.com/textileio/go-threads/core/db"
)

type TxMapDatastore struct {
	*datastore.MapDatastore
	lock sync.RWMutex
}

func NewTxMapDatastore() *TxMapDatastore {
	return &TxMapDatastore{
		MapDatastore: datastore.NewMapDatastore(),
	}
}

func (d *TxMapDatastore) NewTransaction(_ bool) (datastore.Txn, error) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	return NewSimpleTx(d), nil
}

type op struct {
	delete bool
	value  []byte
}

// SimpleTx implements the transaction interface for datastores who do
// not have any sort of underlying transactional support.
type SimpleTx struct {
	ops    map[datastore.Key]op
	lock   sync.RWMutex
	target datastore.Datastore
}

func NewSimpleTx(ds datastore.Datastore) datastore.Txn {
	return &SimpleTx{
		ops:    make(map[datastore.Key]op),
		target: ds,
	}
}

func (bt *SimpleTx) Query(q query.Query) (query.Results, error) {
	bt.lock.RLock()
	defer bt.lock.RUnlock()
	return bt.target.Query(q)
}

func (bt *SimpleTx) Get(k datastore.Key) ([]byte, error) {
	bt.lock.RLock()
	defer bt.lock.RUnlock()
	return bt.target.Get(k)
}

func (bt *SimpleTx) Has(k datastore.Key) (bool, error) {
	bt.lock.RLock()
	defer bt.lock.RUnlock()
	return bt.target.Has(k)
}

func (bt *SimpleTx) GetSize(k datastore.Key) (int, error) {
	bt.lock.RLock()
	defer bt.lock.RUnlock()
	return bt.target.GetSize(k)
}

func (bt *SimpleTx) Put(key datastore.Key, val []byte) error {
	bt.lock.RLock()
	defer bt.lock.RUnlock()
	bt.ops[key] = op{value: val}
	return nil
}

func (bt *SimpleTx) Delete(key datastore.Key) error {
	bt.lock.RLock()
	defer bt.lock.RUnlock()
	bt.ops[key] = op{delete: true}
	return nil
}

func (bt *SimpleTx) Discard() {
	bt.lock.RLock()
	defer bt.lock.RUnlock()
}

func (bt *SimpleTx) Commit() error {
	bt.lock.RLock()
	defer bt.lock.RUnlock()
	var err error
	for k, op := range bt.ops {
		if op.delete {
			err = bt.target.Delete(k)
		} else {
			err = bt.target.Put(k, op.value)
		}
		if err != nil {
			break
		}
	}

	return err
}

type nullReducer struct{}

var _ Reducer = (*nullReducer)(nil)

func (n *nullReducer) Reduce(_ []core.Event) error {
	return nil
}

type errorReducer struct{}

var _ Reducer = (*errorReducer)(nil)

func (n *errorReducer) Reduce(_ []core.Event) error {
	return errors.New("error")
}

type slowReducer struct{}

var _ Reducer = (*slowReducer)(nil)

func (n *slowReducer) Reduce(_ []core.Event) error {
	time.Sleep(2 * time.Second)
	return nil
}
