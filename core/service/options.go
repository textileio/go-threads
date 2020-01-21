package service

import (
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/go-threads/crypto/symmetric"
)

// KeyOptions defines options for keys when creating / adding a thread.
type KeyOptions struct {
	FollowKey *symmetric.Key
	ReadKey   *symmetric.Key
	LogKey    crypto.Key
}

// KeyOption specifies encryption keys.
type KeyOption func(*KeyOptions)

// FollowKey allows thread record traversal.
func FollowKey(key *symmetric.Key) KeyOption {
	return func(args *KeyOptions) {
		args.FollowKey = key
	}
}

// ReadKey allows for thread record decryption.
func ReadKey(key *symmetric.Key) KeyOption {
	return func(args *KeyOptions) {
		args.ReadKey = key
	}
}

// LogKey defines the public or private key used to write a log records.
// If this is just a public key, the service itself won't be able to create records.
// In other words, all records must pre-created and added with AddRecord.
// If no log key is provided, one will be created internally.
func LogKey(key crypto.Key) KeyOption {
	return func(args *KeyOptions) {
		args.LogKey = key
	}
}

// SubOptions defines options for a thread subscription.
type SubOptions struct {
	ThreadIDs thread.IDSlice
}

// SubOption is a thread subscription option.
type SubOption func(*SubOptions)

// ThreadID restricts the subscription to the given thread.
// Use this option multiple times to build up a list of threads
// to subscribe to.
func ThreadID(id thread.ID) SubOption {
	return func(args *SubOptions) {
		args.ThreadIDs = append(args.ThreadIDs, id)
	}
}
