package tstoreds

import (
	"context"
	"time"

	ds "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/textileio/go-textile-core/thread"
	iface "github.com/textileio/go-textile-core/threadstore"
	threads "github.com/textileio/go-textile-threads"
	"github.com/whyrusleeping/base32"
)

// Configuration object for datastores
type Options struct {
	// The size of the in-memory cache. A value of 0 or lower disables the cache.
	CacheSize uint

	// Sweep interval to purge expired addresses from the datastore. If this is a zero value, GC will not run
	// automatically, but it'll be available on demand via explicit calls.
	GCPurgeInterval time.Duration

	// Initial delay before GC processes start. Intended to give the system breathing room to fully boot
	// before starting GC.
	GCInitialDelay time.Duration
}

// DefaultOpts returns the default options for a persistent peerstore, with the full-purge GC algorithm:
//
// * Cache size: 1024.
// * GC purge interval: 2 hours.
// * GC initial delay: 60 seconds.
func DefaultOpts() Options {
	return Options{
		CacheSize:       1024,
		GCPurgeInterval: 2 * time.Hour,
		GCInitialDelay:  60 * time.Second,
	}
}

// NewThreadstore creates a threadstore backed by the provided persistent datastore.
func NewThreadstore(ctx context.Context, store ds.Batching, opts Options) (iface.Threadstore, error) {
	addrBook, err := NewAddrBook(ctx, store, opts)
	if err != nil {
		return nil, err
	}

	keyBook, err := NewKeyBook(store)
	if err != nil {
		return nil, err
	}

	threadMetadata := NewThreadMetadata(store)

	headBook := NewHeadBook(store.(ds.TxnDatastore))

	ps := threads.NewThreadstore(keyBook, addrBook, headBook, threadMetadata)
	return ps, nil
}

// uniqueThreadIds extracts and returns unique thread IDs from database keys.
func uniqueThreadIds(ds ds.Datastore, prefix ds.Key, extractor func(result query.Result) string) (thread.IDSlice, error) {
	var (
		q       = query.Query{Prefix: prefix.String(), KeysOnly: true}
		results query.Results
		err     error
	)

	if results, err = ds.Query(q); err != nil {
		log.Error(err)
		return nil, err
	}

	defer results.Close()

	idset := make(map[string]struct{})
	for result := range results.Next() {
		k := extractor(result)
		idset[k] = struct{}{}
	}

	if len(idset) == 0 {
		return thread.IDSlice{}, nil
	}

	ids := make(thread.IDSlice, 0, len(idset))
	for id := range idset {
		pid, _ := base32.RawStdEncoding.DecodeString(id)
		id, _ := thread.Cast(pid)
		ids = append(ids, id)
	}
	return ids, nil
}

// uniqueLogIds extracts and returns unique thread IDs from database keys.
func uniqueLogIds(ds ds.Datastore, prefix ds.Key, extractor func(result query.Result) string) (peer.IDSlice, error) {
	var (
		q       = query.Query{Prefix: prefix.String(), KeysOnly: true}
		results query.Results
		err     error
	)

	if results, err = ds.Query(q); err != nil {
		log.Error(err)
		return nil, err
	}

	defer results.Close()

	idset := make(map[string]struct{})
	for result := range results.Next() {
		k := extractor(result)
		idset[k] = struct{}{}
	}

	if len(idset) == 0 {
		return peer.IDSlice{}, nil
	}

	ids := make(peer.IDSlice, 0, len(idset))
	for id := range idset {
		pid, _ := base32.RawStdEncoding.DecodeString(id)
		id, _ := peer.IDFromBytes(pid)
		ids = append(ids, id)
	}
	return ids, nil
}
