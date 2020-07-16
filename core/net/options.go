package net

import (
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/textileio/go-threads/core/thread"
)

// NewThreadOptions defines options to be used when creating / adding a thread.
type NewThreadOptions struct {
	ThreadKey thread.Key
	LogKey    crypto.Key
	Token     thread.Token
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

// WithNewThreadToken provides authorization for creating a new thread.
func WithNewThreadToken(t thread.Token) NewThreadOption {
	return func(args *NewThreadOptions) {
		args.Token = t
	}
}

// ThreadOptions defines options for interacting with a thread.
type ThreadOptions struct {
	Token    thread.Token
	APIToken Token
}

// ThreadOption specifies thread options.
type ThreadOption func(*ThreadOptions)

// WithThreadToken provides authorization for interacting with a thread.
func WithThreadToken(t thread.Token) ThreadOption {
	return func(args *ThreadOptions) {
		args.Token = t
	}
}

// WithAPIToken provides additional authorization for interacting
// with a thread as an application.
// For example, this is used by a db.DB to ensure that only it can
// create records or delete the underlying thread.
func WithAPIToken(t Token) ThreadOption {
	return func(args *ThreadOptions) {
		args.APIToken = t
	}
}

// SubOptions defines options for a thread subscription.
type SubOptions struct {
	ThreadIDs thread.IDSlice
	Token     thread.Token
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

// WithSubToken provides authorization for a subscription.
func WithSubToken(t thread.Token) SubOption {
	return func(args *SubOptions) {
		args.Token = t
	}
}
