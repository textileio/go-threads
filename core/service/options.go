package service

import (
	"github.com/textileio/go-threads/crypto/symmetric"
)

// AddOptions defines options for adding a new thread.
type AddOptions struct {
	FollowKey *symmetric.Key
	ReadKey   *symmetric.Key
}

// AddOption is an add thread option.
type AddOption func(*AddOptions)

// FollowKey allows thread record traversal.
func FollowKey(key *symmetric.Key) AddOption {
	return func(args *AddOptions) {
		args.FollowKey = key
	}
}

// ReadKey allows for thread record decryption.
func ReadKey(key *symmetric.Key) AddOption {
	return func(args *AddOptions) {
		args.ReadKey = key
	}
}

// SubOptions defines options for a thread subscription.
type SubOptions struct {
	ThreadIDs IDSlice
}

// SubOption is a thread subscription option.
type SubOption func(*SubOptions)

// ThreadID restricts the subscription to the given thread.
// Use this option multiple times to build up a list of threads
// to subscribe to.
func ThreadID(id ID) SubOption {
	return func(args *SubOptions) {
		args.ThreadIDs = append(args.ThreadIDs, id)
	}
}
