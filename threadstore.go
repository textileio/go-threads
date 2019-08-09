package threadstore

import (
	"fmt"
	"io"

	ic "github.com/libp2p/go-libp2p-core/crypto"
	"github.com/textileio/go-textile-core/thread"
	tstore "github.com/textileio/go-textile-core/threadstore"
)

type threadstore struct {
	tstore.LogKeyBook
	tstore.LogAddrBook
	tstore.ThreadMetadata
}

func NewThreadstore(kb tstore.LogKeyBook, ab tstore.LogAddrBook, md tstore.ThreadMetadata) tstore.Threadstore {
	return &threadstore{
		LogKeyBook:     kb,
		LogAddrBook:    ab,
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

	weakClose("logkeybook", ts.LogKeyBook)
	weakClose("logaddressbook", ts.LogAddrBook)
	weakClose("threadmetadata", ts.ThreadMetadata)

	if len(errs) > 0 {
		return fmt.Errorf("failed while closing threadstore; err(s): %q", errs)
	}
	return nil
}

func (ts *threadstore) Threads() thread.IDSlice {
	// @todo
	return nil
}

func (ts *threadstore) ThreadInfo(t thread.ID) thread.Info {
	set := map[ic.PubKey]struct{}{}
	for _, l := range ts.LogsWithKeys(t) {
		set[l] = struct{}{}
	}
	for _, l := range ts.LogsWithAddrs(t) {
		set[l] = struct{}{}
	}

	//logs := make([]interface{}, 0, len(set))
	//for l := range set {
	//	logs = append(logs, l)
	//}

	return thread.Info{
		ID:   t,
		Logs: ts.LogsWithKeys(t),
	}
}
