package lstoremem

import (
	core "github.com/textileio/go-threads/core/logstore"
	lstore "github.com/textileio/go-threads/logstore"
)

// NewLogstore creates an in-memory threadsafe collection of thread logs.
func NewLogstore() core.Logstore {
	return lstore.NewLogstore(
		NewKeyBook(),
		NewAddrBook(),
		NewHeadBook(),
		NewThreadMetadata())
}
