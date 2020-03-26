package thread

import (
	"github.com/libp2p/go-libp2p-core/crypto"
)

// Signature of an ID
type Signature []byte

// Credentials are used to prove a caller identity, which is needed
// for thread ACL checks.
type Credentials interface {
	// ThreadID must return the signed thread ID.
	ThreadID() ID

	// Sign must return the public key and signature which can
	// be used to verify the caller.
	Sign() (crypto.PubKey, Signature, error)
}

// DefaultCreds are useful for either threads that don't require
// credentials or when the private key is available for local signing.
type DefaultCreds struct {
	threadID ID
	privKey  crypto.PrivKey
}

// NewDefaultCreds returns new default credentials.
func NewDefaultCreds(id ID, opts ...CredOption) Credentials {
	args := &CredOptions{}
	for _, opt := range opts {
		opt(args)
	}
	return DefaultCreds{threadID: id, privKey: args.PrivKey}
}

func (c DefaultCreds) ThreadID() ID {
	return c.threadID
}

func (c DefaultCreds) Sign() (crypto.PubKey, Signature, error) {
	if c.privKey == nil {
		return nil, nil, nil
	}
	sig, err := c.privKey.Sign(c.threadID.Bytes())
	if err != nil {
		return nil, nil, nil
	}
	return c.privKey.GetPublic(), sig, nil
}

// CredOptions defines options for thread credentials.
type CredOptions struct {
	PrivKey crypto.PrivKey
}

// CredOption specifies thread credential options.
type CredOption func(*CredOptions)

// WithPrivKey allows for the credentials to be signed.
func WithPrivKey(sk crypto.PrivKey) CredOption {
	return func(args *CredOptions) {
		args.PrivKey = sk
	}
}

// SignedCreds are useful when the private key is not available.
// In this case, the public key and signature are used directly.
type SignedCreds struct {
	threadID  ID
	pubKey    crypto.PubKey
	signature Signature
}

// NewSignedCreds returns new pre-signed credentials from the source.
func NewSignedCredsFromBytes(id, pubKey, signature []byte) (creds SignedCreds, err error) {
	creds.threadID, err = Cast(id)
	if err != nil {
		return
	}
	if pubKey == nil {
		return
	}
	key, err := crypto.UnmarshalPublicKey(pubKey)
	if err != nil {
		return
	}
	return SignedCreds{
		pubKey:    key,
		signature: signature,
	}, nil
}

func (c SignedCreds) ThreadID() ID {
	return c.threadID
}

func (c SignedCreds) Sign() (crypto.PubKey, Signature, error) {
	return c.pubKey, c.signature, nil
}
