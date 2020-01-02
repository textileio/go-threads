package logstore

import (
	"fmt"
	"io"

	"github.com/libp2p/go-libp2p-core/peer"
	pstore "github.com/libp2p/go-libp2p-core/peerstore"
	core "github.com/textileio/go-threads/core/logstore"
	"github.com/textileio/go-threads/core/thread"
)

// logstore is a collection of books for storing thread logs.
type logstore struct {
	core.KeyBook
	core.AddrBook
	core.ThreadMetadata
	core.HeadBook
}

// NewLogstore creates a new log store from the given books.
func NewLogstore(kb core.KeyBook, ab core.AddrBook, hb core.HeadBook, md core.ThreadMetadata) core.Logstore {
	return &logstore{
		KeyBook:        kb,
		AddrBook:       ab,
		HeadBook:       hb,
		ThreadMetadata: md,
	}
}

// Close the logstore.
func (ts *logstore) Close() (err error) {
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
		return fmt.Errorf("failed while closing logstore; err(s): %q", errs)
	}
	return nil
}

// Threads returns a list of the thread IDs in the store.
func (ts *logstore) Threads() (thread.IDSlice, error) {
	set := map[thread.ID]struct{}{}
	threadsFromKeys, err := ts.ThreadsFromKeys()
	if err != nil {
		return nil, err
	}
	for _, t := range threadsFromKeys {
		set[t] = struct{}{}
	}
	threadsFromAddrs, err := ts.ThreadsFromAddrs()
	if err != nil {
		return nil, err
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

// AddThread adds a thread with keys.
func (ts *logstore) AddThread(info thread.Info) error {
	if info.FollowKey == nil {
		return fmt.Errorf("a follow-key is required to add a thread")
	}
	if err := ts.AddFollowKey(info.ID, info.FollowKey); err != nil {
		return err
	}
	if info.ReadKey != nil {
		if err := ts.AddReadKey(info.ID, info.ReadKey); err != nil {
			return err
		}
	}
	return nil
}

// ThreadInfo returns thread info of the given id.
func (ts *logstore) ThreadInfo(id thread.ID) (info thread.Info, err error) {
	set := map[peer.ID]struct{}{}
	logsWithKeys, err := ts.LogsWithKeys(id)
	if err != nil {
		return
	}
	for _, l := range logsWithKeys {
		set[l] = struct{}{}
	}
	logsWithAddrs, err := ts.LogsWithAddrs(id)
	if err != nil {
		return
	}
	for _, l := range logsWithAddrs {
		set[l] = struct{}{}
	}

	ids := make(peer.IDSlice, 0, len(set))
	for l := range set {
		ids = append(ids, l)
	}

	fk, err := ts.FollowKey(id)
	if err != nil {
		return
	}
	rk, err := ts.ReadKey(id)
	if err != nil {
		return
	}

	return thread.Info{
		ID:        id,
		Logs:      ids,
		FollowKey: fk,
		ReadKey:   rk,
	}, nil
}

// AddLog adds a log under the given thread.
func (ts *logstore) AddLog(id thread.ID, lg thread.LogInfo) error {
	err := ts.AddPubKey(id, lg.ID, lg.PubKey)
	if err != nil {
		return err
	}

	if lg.PrivKey != nil {
		err = ts.AddPrivKey(id, lg.ID, lg.PrivKey)
		if err != nil {
			return err
		}
	}

	err = ts.AddAddrs(id, lg.ID, lg.Addrs, pstore.PermanentAddrTTL)
	if err != nil {
		return err
	}
	err = ts.AddHeads(id, lg.ID, lg.Heads)
	if err != nil {
		return err
	}

	return nil
}

// LogInfo returns info about the given thread.
func (ts *logstore) LogInfo(id thread.ID, lid peer.ID) (info thread.LogInfo, err error) {
	pk, err := ts.PubKey(id, lid)
	if err != nil {
		return
	}
	sk, err := ts.PrivKey(id, lid)
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

	info.ID = lid
	info.PubKey = pk
	info.PrivKey = sk
	info.Addrs = addrs
	info.Heads = heads
	return
}
