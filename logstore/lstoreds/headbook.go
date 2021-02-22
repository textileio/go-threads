package lstoreds

import (
	"encoding/binary"
	"fmt"

	"github.com/gogo/protobuf/proto"
	"github.com/ipfs/go-cid"
	ds "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
	"github.com/libp2p/go-libp2p-core/peer"
	core "github.com/textileio/go-threads/core/logstore"
	"github.com/textileio/go-threads/core/thread"
	pb "github.com/textileio/go-threads/net/pb"
	"github.com/textileio/go-threads/util"
)

type dsHeadBook struct {
	ds ds.TxnDatastore
}

var (
	// Heads are stored in db key pattern:
	// /thread/heads/<base32 thread id no padding>/<base32 peer id no padding>
	hbBase = ds.NewKey("/thread/heads")

	// Heads edges are stored in db key pattern:
	// /thread/heads:edge/<base32 thread id no padding>>
	hbEdge = ds.NewKey("/thread/heads:edge")

	_ core.HeadBook = (*dsHeadBook)(nil)
)

// NewHeadBook returns a new HeadBook backed by a datastore.
func NewHeadBook(ds ds.TxnDatastore) core.HeadBook {
	return &dsHeadBook{
		ds: ds,
	}
}

// AddHead addes a new head to a log.
func (hb *dsHeadBook) AddHead(t thread.ID, p peer.ID, head cid.Cid) error {
	return hb.AddHeads(t, p, []cid.Cid{head})
}

// AddHeads adds multiple heads to a log.
func (hb *dsHeadBook) AddHeads(t thread.ID, p peer.ID, heads []cid.Cid) error {
	txn, err := hb.ds.NewTransaction(false)
	if err != nil {
		return fmt.Errorf("error when creating txn in datastore: %w", err)
	}
	defer txn.Discard()
	key := dsLogKey(t, p, hbBase)
	hr := pb.HeadBookRecord{}
	v, err := txn.Get(key)
	if err == nil {
		if err := proto.Unmarshal(v, &hr); err != nil {
			return fmt.Errorf("error unmarshaling headbookrecord proto: %w", err)
		}
	}
	if err != nil && err != ds.ErrNotFound {
		return fmt.Errorf("error when getting current heads from log %v: %w", key, err)
	}

	set := make(map[cid.Cid]struct{})
	for i := range hr.Heads {
		set[hr.Heads[i].Cid.Cid] = struct{}{}
	}
	for i := range heads {
		if !heads[i].Defined() {
			log.Warnf("ignoring head %s is is undefined for %s", heads[i], key)
			continue
		}
		if _, ok := set[heads[i]]; !ok {
			entry := &pb.HeadBookRecord_HeadEntry{Cid: &pb.ProtoCid{Cid: heads[i]}}
			hr.Heads = append(hr.Heads, entry)
		}
	}
	if data, err := proto.Marshal(&hr); err != nil {
		return fmt.Errorf("error when marshaling headbookrecord proto for %v: %w", key, err)
	} else if err = txn.Put(key, data); err != nil {
		return fmt.Errorf("error when saving new head record in datastore for %v: %v", key, err)
	} else if err := hb.invalidateEdge(txn, t); err != nil {
		return fmt.Errorf("edge invalidation failed for thread %v: %w", t, err)
	}
	return txn.Commit()
}

func (hb *dsHeadBook) SetHead(t thread.ID, p peer.ID, c cid.Cid) error {
	return hb.SetHeads(t, p, []cid.Cid{c})
}

func (hb *dsHeadBook) SetHeads(t thread.ID, p peer.ID, heads []cid.Cid) error {
	txn, err := hb.ds.NewTransaction(false)
	if err != nil {
		return fmt.Errorf("error when creating txn in datastore: %w", err)
	}
	defer txn.Discard()

	var (
		hr  pb.HeadBookRecord
		key = dsLogKey(t, p, hbBase)
	)

	for i := range heads {
		if !heads[i].Defined() {
			log.Warnf("ignoring head %s is undefined for %s", heads[i], key)
			continue
		}
		entry := &pb.HeadBookRecord_HeadEntry{Cid: &pb.ProtoCid{Cid: heads[i]}}
		hr.Heads = append(hr.Heads, entry)
	}

	if data, err := proto.Marshal(&hr); err != nil {
		return fmt.Errorf("error when marshaling headbookrecord proto for %v: %w", key, err)
	} else if err = txn.Put(key, data); err != nil {
		return fmt.Errorf("error when saving new head record in datastore for %v: %w", key, err)
	} else if err := hb.invalidateEdge(txn, t); err != nil {
		return fmt.Errorf("edge invalidation failed for thread %v: %w", t, err)
	}
	return txn.Commit()
}

func (hb *dsHeadBook) Heads(t thread.ID, p peer.ID) ([]cid.Cid, error) {
	key := dsLogKey(t, p, hbBase)
	v, err := hb.ds.Get(key)
	if err == ds.ErrNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("error when getting current heads from log %s: %w", key, err)
	}
	hr := pb.HeadBookRecord{}
	if err := proto.Unmarshal(v, &hr); err != nil {
		return nil, fmt.Errorf("error unmarshaling headbookrecord proto: %v", err)
	}
	ret := make([]cid.Cid, len(hr.Heads))
	for i := range hr.Heads {
		ret[i] = hr.Heads[i].Cid.Cid
	}
	return ret, nil
}

func (hb *dsHeadBook) ClearHeads(t thread.ID, p peer.ID) error {
	txn, err := hb.ds.NewTransaction(false)
	if err != nil {
		return fmt.Errorf("error when creating txn in datastore: %w", err)
	}
	defer txn.Discard()
	var key = dsLogKey(t, p, hbBase)
	if err := txn.Delete(key); err != nil {
		return fmt.Errorf("error when deleting heads from %s", key)
	} else if err := hb.invalidateEdge(txn, t); err != nil {
		return fmt.Errorf("edge invalidation failed for thread %v: %w", t, err)
	}
	return txn.Commit()
}

func (hb *dsHeadBook) HeadsEdge(tid thread.ID) (uint64, error) {
	txn, err := hb.ds.NewTransaction(false)
	if err != nil {
		return 0, fmt.Errorf("error when creating txn in datastore: %w", err)
	}
	defer txn.Discard()
	edge, err := hb.getEdge(txn, tid)
	if err != nil {
		return 0, err
	}
	return edge, txn.Commit()
}

func (hb *dsHeadBook) getEdge(txn ds.Txn, tid thread.ID) (uint64, error) {
	var key = dsThreadKey(tid, hbEdge)
	if v, err := txn.Get(key); err == nil {
		return binary.BigEndian.Uint64(v), nil
	} else if err != ds.ErrNotFound {
		return 0, err
	}

	// edge not evaluated/invalidated, let's compute it
	result, err := txn.Query(query.Query{Prefix: dsThreadKey(tid, hbBase).String(), KeysOnly: false})
	if err != nil {
		return 0, err
	}
	defer result.Close()

	var hs []util.LogHead
	for entry := range result.Next() {
		_, lid, heads, err := hb.decodeHeadEntry(entry, true)
		if err != nil {
			return 0, err
		}
		for i := 0; i < len(heads); i++ {
			hs = append(hs, util.LogHead{Head: heads[i], LogID: lid})
		}
	}
	if len(hs) == 0 {
		return 0, core.ErrThreadNotFound
	}

	var (
		buff [8]byte
		edge = util.ComputeHeadsEdge(hs)
	)
	binary.BigEndian.PutUint64(buff[:], edge)
	return edge, txn.Put(key, buff[:])
}

func (hb *dsHeadBook) invalidateEdge(txn ds.Txn, tid thread.ID) error {
	var key = dsThreadKey(tid, hbEdge)
	return txn.Delete(key)
}

// Dump entire headbook into the tree-structure.
// Not a thread-safe, should not be interleaved with other methods!
func (hb *dsHeadBook) DumpHeads() (core.DumpHeadBook, error) {
	data, err := hb.traverse(true)
	return core.DumpHeadBook{Data: data}, err
}

// Restore headbook from the provided dump replacing all the local data.
// Not a thread-safe, should not be interleaved with other methods!
func (hb *dsHeadBook) RestoreHeads(dump core.DumpHeadBook) error {
	if !AllowEmptyRestore && len(dump.Data) == 0 {
		return core.ErrEmptyDump
	}

	stored, err := hb.traverse(false)
	if err != nil {
		return fmt.Errorf("traversing datastore: %w", err)
	}

	// wipe out existing headbook
	for tid, logs := range stored {
		for lid := range logs {
			if err := hb.ClearHeads(tid, lid); err != nil {
				return fmt.Errorf("clearing heads for %s/%s: %w", tid, lid, err)
			}
		}
	}

	// ... and replace it with the dump
	for tid, logs := range dump.Data {
		for lid, heads := range logs {
			if err := hb.SetHeads(tid, lid, heads); err != nil {
				return fmt.Errorf("setting heads for %s/%s: %w", tid, lid, err)
			}
		}
	}

	return nil
}

func (hb *dsHeadBook) traverse(withHeads bool) (map[thread.ID]map[peer.ID][]cid.Cid, error) {
	var data = make(map[thread.ID]map[peer.ID][]cid.Cid)
	result, err := hb.ds.Query(query.Query{Prefix: hbBase.String(), KeysOnly: !withHeads})
	if err != nil {
		return nil, err
	}
	defer result.Close()

	for entry := range result.Next() {
		tid, lid, heads, err := hb.decodeHeadEntry(entry, withHeads)
		if err != nil {
			return nil, err
		}

		lh, exist := data[tid]
		if !exist {
			lh = make(map[peer.ID][]cid.Cid)
			data[tid] = lh
		}

		lh[lid] = heads
	}

	return data, nil
}

func (hb *dsHeadBook) decodeHeadEntry(
	entry query.Result,
	withHeads bool,
) (tid thread.ID, lid peer.ID, heads []cid.Cid, err error) {
	kns := ds.RawKey(entry.Key).Namespaces()
	if len(kns) < 3 {
		err = fmt.Errorf("bad headbook key detected: %s", entry.Key)
		return
	}
	// get thread and log IDs from the key components
	var ts, ls = kns[len(kns)-2], kns[len(kns)-1]
	if tid, err = parseThreadID(ts); err != nil {
		err = fmt.Errorf("cannot restore thread ID %s: %w", ts, err)
		return
	}
	if lid, err = parseLogID(ls); err != nil {
		err = fmt.Errorf("cannot restore log ID %s: %w", ls, err)
		return
	}
	if withHeads {
		var hr pb.HeadBookRecord
		if err = proto.Unmarshal(entry.Value, &hr); err != nil {
			err = fmt.Errorf("cannot decode headbook record: %w", err)
			return
		}
		heads = make([]cid.Cid, len(hr.Heads))
		for i := range hr.Heads {
			heads[i] = hr.Heads[i].Cid.Cid
		}
	}
	return
}
