package tstoreds

import "time"

// Configuration object for datastores
type Options struct {
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
