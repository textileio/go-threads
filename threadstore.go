package threads

import (
	"fmt"
	"io"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/textileio/go-textile-core/thread"
	tstore "github.com/textileio/go-textile-core/threadstore"
)

type threadstore struct {
	tstore.KeyBook
	tstore.AddrBook
	tstore.ThreadMetadata
	tstore.HeadBook
}

func NewThreadstore(kb tstore.KeyBook, ab tstore.AddrBook, hb tstore.HeadBook, md tstore.ThreadMetadata) tstore.Threadstore {
	return &threadstore{
		KeyBook:        kb,
		AddrBook:       ab,
		HeadBook:       hb,
		ThreadMetadata: md,
	}
}

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

func (ts *threadstore) ThreadInfo(t thread.ID) thread.Info {
	set := map[peer.ID]struct{}{}
	for _, l := range ts.LogsWithKeys(t) {
		set[l] = struct{}{}
	}
	for _, l := range ts.LogsWithAddrs(t) {
		set[l] = struct{}{}
	}

	ids := make(peer.IDSlice, 0, len(set))
	for l := range set {
		ids = append(ids, l)
	}

	return thread.Info{
		ID:   t,
		Logs: ids,
	}
}
