package net

import (
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/textileio/go-threads/core/thread"
)

// NewThreadOptions defines options to be used when creating / adding a thread.
type NewThreadOptions struct {
	ThreadKey thread.Key
	LogKey    crypto.Key
	Auth      *thread.Auth
}

// NewThreadOption specifies new thread options.
type NewThreadOption func(*NewThreadOptions)

// WithThreadKey handles log encryption.
func WithThreadKey(key thread.Key) NewThreadOption {
	return func(args *NewThreadOptions) {
		args.ThreadKey = key
	}
}

// WithLogKey is the public or private key used to write log records.
// If this is just a public key, the service itself won't be able to create records.
// In other words, all records must be pre-created and added with AddRecord.
// If no log key is provided, one will be created internally.
func WithLogKey(key crypto.Key) NewThreadOption {
	return func(args *NewThreadOptions) {
		args.LogKey = key
	}
}

// WithNewThreadAuth provides authentication for creating / adding a new thread.
func WithNewThreadAuth(a *thread.Auth) NewThreadOption {
	return func(args *NewThreadOptions) {
		args.Auth = a
	}
}

// ThreadOptions defines options to be used interacting with a thread.
type ThreadOptions struct {
	Auth *thread.Auth
}

// ThreadOption specifies thread options.
type ThreadOption func(*ThreadOptions)

// WithThreadAuth provides authentication for interacting with a thread.
func WithThreadAuth(a *thread.Auth) ThreadOption {
	return func(args *ThreadOptions) {
		args.Auth = a
	}
}

// SubOptions defines options for a thread subscription.
type SubOptions struct {
	ThreadIDs thread.IDSlice
	Auth      *thread.Auth
}

// SubOption is a thread subscription option.
type SubOption func(*SubOptions)

// WithSubFilter restricts the subscription to a given thread.
// Use this option multiple times to subscribe to multiple threads.
func WithSubFilter(id thread.ID) SubOption {
	return func(args *SubOptions) {
		args.ThreadIDs = append(args.ThreadIDs, id)
	}
}

// WithSubAuth provides authentication for interacting with a thread.
func WithSubAuth(a *thread.Auth) SubOption {
	return func(args *SubOptions) {
		args.Auth = a
	}
}
