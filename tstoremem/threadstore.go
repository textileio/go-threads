package tstoremem

import (
	iface "github.com/textileio/go-textile-core/threadstore"
	threads "github.com/textileio/go-textile-threads"
)

// NewThreadstore creates an in-memory threadsafe collection of peers.
func NewThreadstore() iface.Threadstore {
	return threads.NewThreadstore(
		NewLogKeyBook(),
		NewLogAddrBook(),
		NewLogHeadBook(),
		NewThreadMetadata())
}
