package tstoremem

import (
	iface "github.com/textileio/go-textile-core/threadstore"
	tstore "github.com/textileio/go-textile-threads"
)

// NewThreadstore creates an in-memory threadsafe collection of peers.
func NewThreadstore() iface.Threadstore {
	return tstore.NewThreadstore(
		NewLogKeyBook(),
		NewLogAddrBook(),
		NewThreadMetadata())
}
