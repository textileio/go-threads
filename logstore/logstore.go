package logstore

import (
	"fmt"
	"io"
	"sync"

	"github.com/libp2p/go-libp2p-core/peer"
	pstore "github.com/libp2p/go-libp2p-core/peerstore"
	core "github.com/textileio/go-threads/core/logstore"
	"github.com/textileio/go-threads/core/thread"
)

// logstore is a collection of books for storing thread logs.
type logstore struct {
	sync.RWMutex

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
func (ls *logstore) Close() (err error) {
	var errs []error
	weakClose := func(name string, c interface{}) {
		if cl, ok := c.(io.Closer); ok {
			if err = cl.Close(); err != nil {
				errs = append(errs, fmt.Errorf("%s error: %s", name, err))
			}
		}
	}

	weakClose("keybook", ls.KeyBook)
	weakClose("addressbook", ls.AddrBook)
	weakClose("headbook", ls.HeadBook)
	weakClose("threadmetadata", ls.ThreadMetadata)

	if len(errs) > 0 {
		return fmt.Errorf("failed while closing logstore; err(s): %q", errs)
	}
	return nil
}

// Threads returns a list of the thread IDs in the store.
func (ls *logstore) Threads() (thread.IDSlice, error) {
	ls.RLock()
	defer ls.RUnlock()

	set := map[thread.ID]struct{}{}
	threadsFromKeys, err := ls.ThreadsFromKeys()
	if err != nil {
		return nil, err
	}
	for _, t := range threadsFromKeys {
		set[t] = struct{}{}
	}
	threadsFromAddrs, err := ls.ThreadsFromAddrs()
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
func (ls *logstore) AddThread(info thread.Info) error {
	ls.Lock()
	defer ls.Unlock()

	if info.Key.Service() == nil {
		return fmt.Errorf("a service-key is required to add a thread")
	}
	if err := ls.AddServiceKey(info.ID, info.Key.Service()); err != nil {
		return err
	}
	if info.Key.CanRead() {
		if err := ls.AddReadKey(info.ID, info.Key.Read()); err != nil {
			return err
		}
	}
	return nil
}

// GetThread returns thread info of the given id.
func (ls *logstore) GetThread(id thread.ID) (info thread.Info, err error) {
	ls.RLock()
	defer ls.RUnlock()

	sk, err := ls.ServiceKey(id)
	if err != nil {
		return
	}
	if sk == nil {
		return info, core.ErrThreadNotFound
	}
	rk, err := ls.ReadKey(id)
	if err != nil {
		return
	}

	set, err := ls.getLogIDs(id)
	if err != nil {
		return
	}

	logs := make([]thread.LogInfo, 0, len(set))
	for l := range set {
		i, err := ls.GetLog(id, l)
		if err != nil {
			return info, err
		}
		logs = append(logs, i)
	}

	return thread.Info{
		ID:   id,
		Logs: logs,
		Key:  thread.NewKey(sk, rk),
	}, nil
}

func (ls *logstore) getLogIDs(id thread.ID) (map[peer.ID]struct{}, error) {
	set := map[peer.ID]struct{}{}
	logsWithKeys, err := ls.LogsWithKeys(id)
	if err != nil {
		return nil, err
	}
	for _, l := range logsWithKeys {
		set[l] = struct{}{}
	}
	logsWithAddrs, err := ls.LogsWithAddrs(id)
	if err != nil {
		return nil, err
	}
	for _, l := range logsWithAddrs {
		set[l] = struct{}{}
	}
	return set, nil
}

// DeleteThread deletes a thread.
func (ls *logstore) DeleteThread(id thread.ID) error {
	ls.Lock()
	defer ls.Unlock()

	if err := ls.ClearKeys(id); err != nil {
		return err
	}

	set, err := ls.getLogIDs(id)
	if err != nil {
		return nil
	}
	for l := range set {
		if err := ls.ClearAddrs(id, l); err != nil {
			return err
		}
		if err := ls.ClearHeads(id, l); err != nil {
			return err
		}
	}
	return nil
}

// AddLog adds a log under the given thread.
func (ls *logstore) AddLog(id thread.ID, lg thread.LogInfo) error {
	ls.Lock()
	defer ls.Unlock()

	err := ls.AddPubKey(id, lg.ID, lg.PubKey)
	if err != nil {
		return err
	}
	if lg.PrivKey != nil {
		if err = ls.AddPrivKey(id, lg.ID, lg.PrivKey); err != nil {
			return err
		}
	}
	if err = ls.AddAddrs(id, lg.ID, lg.Addrs, pstore.PermanentAddrTTL); err != nil {
		return err
	}
	if err = ls.AddHeads(id, lg.ID, lg.Heads); err != nil {
		return err
	}
	return nil
}

// GetLog returns info about the given thread.
func (ls *logstore) GetLog(id thread.ID, lid peer.ID) (info thread.LogInfo, err error) {
	ls.RLock()
	defer ls.RUnlock()

	pk, err := ls.PubKey(id, lid)
	if err != nil {
		return
	}
	if pk == nil {
		return info, core.ErrLogNotFound
	}
	sk, err := ls.PrivKey(id, lid)
	if err != nil {
		return
	}
	addrs, err := ls.Addrs(id, lid)
	if err != nil {
		return
	}
	heads, err := ls.Heads(id, lid)
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

// DeleteLog deletes a log.
func (ls *logstore) DeleteLog(id thread.ID, lid peer.ID) (err error) {
	ls.Lock()
	defer ls.Unlock()

	if err = ls.ClearLogKeys(id, lid); err != nil {
		return
	}
	if err = ls.ClearAddrs(id, lid); err != nil {
		return
	}
	if err = ls.ClearHeads(id, lid); err != nil {
		return
	}
	return nil
}
