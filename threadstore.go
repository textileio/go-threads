package threads

import (
	"fmt"
	"io"

	"github.com/libp2p/go-libp2p-core/peer"
	pstore "github.com/libp2p/go-libp2p-core/peerstore"
	"github.com/textileio/go-textile-core/thread"
	tstore "github.com/textileio/go-textile-core/threadstore"
)

// threadstore is a collection of books for storing threads.
type threadstore struct {
	tstore.KeyBook
	tstore.AddrBook
	tstore.ThreadMetadata
	tstore.HeadBook
}

// NewThreadstore creates a new thread store from the given books.
func NewThreadstore(kb tstore.KeyBook, ab tstore.AddrBook, hb tstore.HeadBook, md tstore.ThreadMetadata) tstore.Threadstore {
	return &threadstore{
		KeyBook:        kb,
		AddrBook:       ab,
		HeadBook:       hb,
		ThreadMetadata: md,
	}
}

// Close the threadstore.
func (ts *threadstore) Close() (err error) {
	var errs []error
	weakClose := func(name string, c interface{}) {
		if cl, ok := c.(io.Closer); ok {
			if err = cl.Close(); err != nil {
				errs = append(errs, fmt.Errorf("%s error: %s", name, err))
			}
		}
	}

	weakClose("keybook", ts.KeyBook)
	weakClose("addressbook", ts.AddrBook)
	weakClose("headbook", ts.HeadBook)
	weakClose("threadmetadata", ts.ThreadMetadata)

	if len(errs) > 0 {
		return fmt.Errorf("failed while closing threadstore; err(s): %q", errs)
	}
	return nil
}

// Threads returns a list of the thread IDs in the store.
func (ts *threadstore) Threads() thread.IDSlice {
	set := map[thread.ID]struct{}{}
	for _, t := range ts.ThreadsFromKeys() {
		set[t] = struct{}{}
	}
	for _, t := range ts.ThreadsFromAddrs() {
		set[t] = struct{}{}
	}

	ids := make(thread.IDSlice, 0, len(set))
	for t := range set {
		ids = append(ids, t)
	}

	return ids
}

// ThreadInfo returns thread info of the given id.
func (ts *threadstore) ThreadInfo(id thread.ID) thread.Info {
	set := map[peer.ID]struct{}{}
	for _, l := range ts.LogsWithKeys(id) {
		set[l] = struct{}{}
	}
	for _, l := range ts.LogsWithAddrs(id) {
		set[l] = struct{}{}
	}

	ids := make(peer.IDSlice, 0, len(set))
	for l := range set {
		ids = append(ids, l)
	}

	return thread.Info{
		ID:   id,
		Logs: ids,
	}
}

// AddLog adds a log under the given thread.
func (ts *threadstore) AddLog(id thread.ID, log thread.LogInfo) error {
	err := ts.AddPubKey(id, log.ID, log.PubKey)
	if err != nil {
		return err
	}

	if log.PrivKey != nil {
		err = ts.AddPrivKey(id, log.ID, log.PrivKey)
		if err != nil {
			return err
		}
	}
	if log.FollowKey != nil {
		err = ts.AddFollowKey(id, log.ID, log.FollowKey)
		if err != nil {
			return err
		}
	}
	if log.ReadKey != nil {
		err = ts.AddReadKey(id, log.ID, log.ReadKey)
		if err != nil {
			return err
		}
	}

	ts.SetAddrs(id, log.ID, log.Addrs, pstore.PermanentAddrTTL)
	ts.SetHeads(id, log.ID, log.Heads)

	return nil
}

// LogInfo returns info about the given thread.
func (ts *threadstore) LogInfo(id thread.ID, lid peer.ID) thread.LogInfo {
	return thread.LogInfo{
		ID:        lid,
		PubKey:    ts.PubKey(id, lid),
		PrivKey:   ts.PrivKey(id, lid),
		FollowKey: ts.FollowKey(id, lid),
		ReadKey:   ts.ReadKey(id, lid),
		Addrs:     ts.Addrs(id, lid),
		Heads:     ts.Heads(id, lid),
	}
}
