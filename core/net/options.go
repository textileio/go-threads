package net

import (
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/textileio/go-threads/core/thread"
)

// NewThreadOptions defines options to be used when creating / adding a thread.
type NewThreadOptions struct {
	ThreadKey thread.Key
	LogKey    crypto.Key
	Identity  thread.Identity
	Auth      thread.Auth
}

func (o *NewThreadOptions) MarshalBinary() ([]byte, error) {
	bytes := o.ThreadKey.Bytes()
	if o.LogKey != nil {
		lkb, err := o.LogKey.Bytes()
		if err != nil {
			return nil, err
		}
		bytes = append(bytes, lkb...)
	}
	return bytes, nil
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

// WithNewThreadIdentity provides a signing identity for creating / adding a new thread.
// This option will overwrite WithNewThreadAuth.
func WithNewThreadIdentity(i thread.Identity) NewThreadOption {
	return func(args *NewThreadOptions) {
		args.Identity = i
	}
}

// WithNewThreadAuth provides authentication for creating / adding a new thread.
// This option should only be used directly by apps that receive auth over an API.
func WithNewThreadAuth(a thread.Auth) NewThreadOption {
	return func(args *NewThreadOptions) {
		args.Auth = a
	}
}

// ThreadOptions defines options to be used interacting with a thread.
type ThreadOptions struct {
	Identity thread.Identity
	Auth     thread.Auth
}

func (o *ThreadOptions) MarshalBinary() ([]byte, error) {
	return nil, nil
}

// ThreadOption specifies thread options.
type ThreadOption func(*ThreadOptions)

// WithThreadIdentity provides a signing identity for interacting with a thread.
// This option will overwrite WithNewThreadAuth.
func WithThreadIdentity(i thread.Identity) ThreadOption {
	return func(args *ThreadOptions) {
		args.Identity = i
	}
}

// WithThreadAuth provides authorization for interacting with a thread.
// This option should only be used directly by apps that receive auth over an API.
func WithThreadAuth(a thread.Auth) ThreadOption {
	return func(args *ThreadOptions) {
		args.Auth = a
	}
}

// SubOptions defines options for a thread subscription.
type SubOptions struct {
	ThreadIDs thread.IDSlice
	Identity  thread.Identity
	Auth      thread.Auth
}

func (o *SubOptions) MarshalBinary() ([]byte, error) {
	var bytes []byte
	for _, id := range o.ThreadIDs {
		bytes = append(bytes, id.Bytes()...)
	}
	return bytes, nil
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

// WithSubIdentity provides a signing identity for a thread subscription.
// This option will overwrite WithSubAuth.
func WithSubIdentity(i thread.Identity) SubOption {
	return func(args *SubOptions) {
		args.Identity = i
	}
}

// WithSubAuth provides authorization for interacting with a thread.
func WithSubAuth(a thread.Auth) SubOption {
	return func(args *SubOptions) {
		args.Auth = a
	}
}
