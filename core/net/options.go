package net

import (
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/textileio/go-threads/core/thread"
)

// KeyOptions defines options for keys when creating / adding a thread.
type KeyOptions struct {
	ThreadKey thread.Key
	LogKey    crypto.Key
}

// KeyOption specifies encryption keys.
type KeyOption func(*KeyOptions)

// WithThreadKey handles log encryption.
func WithThreadKey(key thread.Key) KeyOption {
	return func(args *KeyOptions) {
		args.ThreadKey = key
	}
}

// WithLogKey is the public or private key used to write log records.
// If this is just a public key, the service itself won't be able to create records.
// In other words, all records must be pre-created and added with AddRecord.
// If no log key is provided, one will be created internally.
func WithLogKey(key crypto.Key) KeyOption {
	return func(args *KeyOptions) {
		args.LogKey = key
	}
}

// SubOptions defines options for a thread subscription.
type SubOptions struct {
	Credentials []thread.Credentials
}

// SubOption is a thread subscription option.
type SubOption func(*SubOptions)

// WithCredentials restricts the subscription to a given thread.
// Use this option multiple times to subscribe to multiple threads.
func WithCredentials(creds thread.Credentials) SubOption {
	return func(args *SubOptions) {
		args.Credentials = append(args.Credentials, creds)
	}
}
