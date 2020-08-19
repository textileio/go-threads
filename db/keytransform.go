package db

import (
	ds "github.com/textileio/go-datastore"
	kt "github.com/textileio/go-datastore/keytransform"
	dsq "github.com/textileio/go-datastore/query"
)

func wrapTxnDatastore(child ds.TxnDatastore, t kt.KeyTransform) *Datastore {
	nds := &Datastore{
		child:        child,
		Datastore:    kt.Wrap(child, t),
		KeyTransform: t,
	}
	return nds
}

// Datastore keeps a KeyTransform function
type Datastore struct {
	child ds.TxnDatastore
	kt.KeyTransform
	ds.Datastore
}

type txn struct {
	ds.Txn
	ds *Datastore
}

func (d *Datastore) NewTransaction(readOnly bool) (ds.Txn, error) {
	t, err := d.child.NewTransaction(readOnly)
	if err != nil {
		return nil, err
	}

	return &txn{Txn: t, ds: d}, nil
}

func (t *txn) Commit() error {
	return t.Txn.Commit()
}

func (t *txn) Discard() {
	t.Txn.Discard()
}

// Put stores the given value, transforming the key first.
func (t *txn) Put(key ds.Key, value []byte) (err error) {
	return t.Txn.Put(t.ds.ConvertKey(key), value)
}

// Delete removes the value for given key
func (t *txn) Delete(key ds.Key) (err error) {
	return t.Txn.Delete(t.ds.ConvertKey(key))
}

// Get returns the value for given key, transforming the key first.
func (t *txn) Get(key ds.Key) (value []byte, err error) {
	return t.Txn.Get(t.ds.ConvertKey(key))
}

// Has returns whether the datastore has a value for a given key, transforming
// the key first.
func (t *txn) Has(key ds.Key) (exists bool, err error) {
	return t.Txn.Has(t.ds.ConvertKey(key))
}

// GetSize returns the size of the value named by the given key, transforming
// the key first.
func (t *txn) GetSize(key ds.Key) (size int, err error) {
	return t.Txn.GetSize(t.ds.ConvertKey(key))
}

// Query implements Query, inverting keys on the way back out.
func (t *txn) Query(q dsq.Query) (dsq.Results, error) {
	nq, cq := t.prepareQuery(q)

	cqr, err := t.Txn.Query(cq)
	if err != nil {
		return nil, err
	}

	qr := dsq.ResultsFromIterator(q, dsq.Iterator{
		Next: func() (dsq.Result, bool) {
			r, ok := cqr.NextSync()
			if !ok {
				return r, false
			}
			if r.Error == nil {
				r.Entry.Key = t.ds.InvertKey(ds.RawKey(r.Entry.Key)).String()
			}
			return r, true
		},
		Close: func() error {
			return cqr.Close()
		},
	})
	return dsq.NaiveQueryApply(nq, qr), nil
}

// Split the query into a child query and a naive query. That way, we can make
// the child datastore do as much work as possible.
func (t *txn) prepareQuery(q dsq.Query) (naive, child dsq.Query) {

	// First, put everything in the child query. Then, start taking things
	// out.
	child = q

	// Always let the child handle the key prefix.
	child.Prefix = t.ds.ConvertKey(ds.NewKey(child.Prefix)).String()
	if child.SeekPrefix != "" {
		child.SeekPrefix = t.ds.ConvertKey(ds.NewKey(child.SeekPrefix)).String()
	}

	// Check if the key transform is order-preserving so we can use the
	// child datastore's built-in ordering.
	orderPreserving := false
	switch t.ds.KeyTransform.(type) {
	case kt.PrefixTransform, *kt.PrefixTransform:
		orderPreserving = true
	}

	// Try to let the child handle ordering.
orders:
	for i, o := range child.Orders {
		switch o.(type) {
		case dsq.OrderByValue, *dsq.OrderByValue,
			dsq.OrderByValueDescending, *dsq.OrderByValueDescending:
			// Key doesn't matter.
			continue
		case dsq.OrderByKey, *dsq.OrderByKey,
			dsq.OrderByKeyDescending, *dsq.OrderByKeyDescending:
			// if the key transform preserves order, we can delegate
			// to the child datastore.
			if orderPreserving {
				// When sorting, we compare with the first
				// Order, then, if equal, we compare with the
				// second Order, etc. However, keys are _unique_
				// so we'll never apply any additional orders
				// after ordering by key.
				child.Orders = child.Orders[:i+1]
				break orders
			}
		}

		// Can't handle this order under transform, punt it to a naive
		// ordering.
		naive.Orders = q.Orders
		child.Orders = nil
		naive.Offset = q.Offset
		child.Offset = 0
		naive.Limit = q.Limit
		child.Limit = 0
		break
	}

	// Try to let the child handle the filters.

	// don't modify the original filters.
	child.Filters = append([]dsq.Filter(nil), child.Filters...)

	for i, f := range child.Filters {
		switch f := f.(type) {
		case dsq.FilterValueCompare, *dsq.FilterValueCompare:
			continue
		case dsq.FilterKeyCompare:
			child.Filters[i] = dsq.FilterKeyCompare{
				Op:  f.Op,
				Key: t.ds.ConvertKey(ds.NewKey(f.Key)).String(),
			}
			continue
		case *dsq.FilterKeyCompare:
			child.Filters[i] = &dsq.FilterKeyCompare{
				Op:  f.Op,
				Key: t.ds.ConvertKey(ds.NewKey(f.Key)).String(),
			}
			continue
		case dsq.FilterKeyPrefix:
			child.Filters[i] = dsq.FilterKeyPrefix{
				Prefix: t.ds.ConvertKey(ds.NewKey(f.Prefix)).String(),
			}
			continue
		case *dsq.FilterKeyPrefix:
			child.Filters[i] = &dsq.FilterKeyPrefix{
				Prefix: t.ds.ConvertKey(ds.NewKey(f.Prefix)).String(),
			}
			continue
		}

		// Not a known filter, defer to the naive implementation.
		naive.Filters = q.Filters
		child.Filters = nil
		naive.Offset = q.Offset
		child.Offset = 0
		naive.Limit = q.Limit
		child.Limit = 0
		break
	}
	return
}

var _ ds.TxnDatastore = (*Datastore)(nil)
