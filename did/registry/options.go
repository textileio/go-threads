package registry

import (
	"time"
)

// Options defines options for interacting with the registry.
type Options struct {
	Timeout time.Duration
}

// Option specifies registry options.
type Option func(*Options)

//// WithThreadToken provides authorization for interacting with a thread.
//func WithTimeout(duration time.Duration) ThreadOption {
//	return func(args *ThreadOptions) {
//		args.Token = t
//	}
//}
