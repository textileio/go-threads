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
func (ts *threadstore) Threads() (thread.IDSlice, error) {
	set := map[thread.ID]struct{}{}
	threadsFromKeys, err := ts.ThreadsFromKeys()
	if err != nil {
		return nil, fmt.Errorf("couldn't fetch threads from keys: %v", err)
	}
	for _, t := range threadsFromKeys {
		set[t] = struct{}{}
	}
	threadsFromAddrs, err := ts.ThreadsFromAddrs()
	if err != nil {
		return nil, fmt.Errorf("couldn't fetch threads from addrs: %v", err)
	}
	for _, t := range threadsFromAddrs {
		set[t] = struct{}{}
	}

	ids := make(thread.IDSlice, 0, len(set))
	for t := range set {
		ids = append(ids, t)
	}

	return ids, nil
}

// ThreadInfo returns thread info of the given id.
func (ts *threadstore) ThreadInfo(id thread.ID) (thread.Info, error) {
	set := map[peer.ID]struct{}{}
	logsWithKeys, err := ts.LogsWithKeys(id)
	if err != nil {
		return thread.Info{}, fmt.Errorf("couldn't fetch logs from keys: %v", err)
	}
	for _, l := range logsWithKeys {
		set[l] = struct{}{}
	}
	logsWithAddrs, err := ts.LogsWithAddrs(id)
	if err != nil {
		return thread.Info{}, fmt.Errorf("couldn't fetch logs from addrs: %v", err)
	}
	for _, l := range logsWithAddrs {
		set[l] = struct{}{}
	}

	ids := make(peer.IDSlice, 0, len(set))
	for l := range set {
		ids = append(ids, l)
	}

	return thread.Info{
		ID:   id,
		Logs: ids,
	}, nil
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
func (ts *threadstore) LogInfo(id thread.ID, lid peer.ID) (li thread.LogInfo, err error) {
	pk, err := ts.PubKey(id, lid)
	if err != nil {
		return
	}
	sk, err := ts.PrivKey(id, lid)
	if err != nil {
		return
	}
	fk, err := ts.FollowKey(id, lid)
	if err != nil {
		return
	}
	rk, err := ts.ReadKey(id, lid)
	if err != nil {
		return
	}
	addrs, err := ts.Addrs(id, lid)
	if err != nil {
		return
	}
	heads, err := ts.Heads(id, lid)
	if err != nil {
		return
	}

	li.ID = lid
	li.PubKey = pk
	li.PrivKey = sk
	li.FollowKey = fk
	li.ReadKey = rk
	li.Addrs = addrs
	li.Heads = heads
	return
}
