package tstoremem

import (
	iface "github.com/textileio/go-textile-core/threadstore"
	threads "github.com/textileio/go-textile-threads"
)

// NewThreadstore creates an in-memory threadsafe collection of threads.
func NewThreadstore() iface.Threadstore {
	return threads.NewThreadstore(
		NewKeyBook(),
		NewAddrBook(),
		NewHeadBook(),
		NewThreadMetadata())
}
