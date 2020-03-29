package net

import (
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/textileio/go-threads/core/thread"
)

// NewThreadOptions defines options to be used when creating / adding a thread.
type NewThreadOptions struct {
	ThreadKey   thread.Key
	LogKey      crypto.Key
	Credentials thread.Credentials
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

// WithNewCredentials specifies credentials for creating / adding a new thread.
func WithNewCredentials(cr thread.Credentials) NewThreadOption {
	return func(args *NewThreadOptions) {
		args.Credentials = cr
	}
}

// ThreadOptions defines options to be used interacting with a thread.
type ThreadOptions struct {
	Credentials thread.Credentials
}

// ThreadOption specifies thread options.
type ThreadOption func(*ThreadOptions)

// WithCredentials specifies credentials for interacting with a thread.
func WithCredentials(cr thread.Credentials) ThreadOption {
	return func(args *ThreadOptions) {
		args.Credentials = cr
	}
}

// ThreadFilter wraps a thread ID and credentials for a subscription filter.
type ThreadFilter struct {
	ID          thread.ID
	Credentials thread.Credentials
}

// SubOptions defines options for a thread subscription.
type SubOptions struct {
	Filters []ThreadFilter
}

// SubOption is a thread subscription option.
type SubOption func(*SubOptions)

// WithSubFilter restricts the subscription to a given thread.
// Use this option multiple times to subscribe to multiple threads.
func WithSubFilter(f ThreadFilter) SubOption {
	return func(args *SubOptions) {
		args.Filters = append(args.Filters, f)
	}
}
