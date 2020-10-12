package lstoreds

import (
	"context"
	"time"

	ds "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
	"github.com/libp2p/go-libp2p-core/peer"
	core "github.com/textileio/go-threads/core/logstore"
	"github.com/textileio/go-threads/core/thread"
	lstore "github.com/textileio/go-threads/logstore"
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

// NewLogstore creates a logstore backed by the provided persistent datastore.
func NewLogstore(ctx context.Context, store ds.Batching, opts Options) (core.Logstore, error) {
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

	ps := lstore.NewLogstore(keyBook, addrBook, headBook, threadMetadata)
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
		id, err := parseThreadID(id)
		if err == nil {
			ids = append(ids, id)
		}
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
		id, err := parseLogID(id)
		if err == nil {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

func dsThreadKey(t thread.ID, baseKey ds.Key) ds.Key {
	key := baseKey.ChildString(base32.RawStdEncoding.EncodeToString(t.Bytes()))
	return key
}

func dsLogKey(t thread.ID, p peer.ID, baseKey ds.Key) ds.Key {
	key := baseKey.ChildString(base32.RawStdEncoding.EncodeToString(t.Bytes()))
	key = key.ChildString(base32.RawStdEncoding.EncodeToString([]byte(p)))
	return key
}

func parseThreadID(id string) (thread.ID, error) {
	pid, err := base32.RawStdEncoding.DecodeString(id)
	if err != nil {
		return thread.Undef, err
	}

	return thread.Cast(pid)
}

func parseLogID(id string) (peer.ID, error) {
	pid, err := base32.RawStdEncoding.DecodeString(id)
	if err != nil {
		return "", err
	}

	return peer.IDFromBytes(pid)
}
