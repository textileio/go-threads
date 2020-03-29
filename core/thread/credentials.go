package thread

import (
	"github.com/libp2p/go-libp2p-core/crypto"
)

// Signature of an ID
type Signature []byte

// Credentials are used to prove a caller identity, which is needed
// for thread ACL checks.
type Credentials interface {
	// Sign must return a public key and signature that can
	// be used to verify the caller.
	Sign([]byte) (crypto.PubKey, Signature, error)
}

// PrivKeyAuth wraps a private key for signing.
// This is useful when the private key is available for local signing.
type PrivKeyAuth struct {
	sk crypto.PrivKey
}

// NewPrivKeyAuth returns credentials with private key..
func NewPrivKeyAuth(privKey crypto.PrivKey) Credentials {
	return PrivKeyAuth{sk: privKey}
}

func (c PrivKeyAuth) Sign(msg []byte) (crypto.PubKey, Signature, error) {
	if c.sk == nil {
		return nil, nil, nil
	}
	sig, err := c.sk.Sign(msg)
	if err != nil {
		return nil, nil, nil
	}
	return c.sk.GetPublic(), sig, nil
}

// PubKeyAuth wrap an existing public key and signature.
type PubKeyAuth struct {
	pk  crypto.PubKey
	sig Signature
}

// NewPubKeyAuth returns new credentials with public key and signature.
func NewPubKeyAuth(pubKey crypto.PubKey, signature []byte) Credentials {
	return PubKeyAuth{pk: pubKey, sig: signature}
}

func (c PubKeyAuth) Sign([]byte) (crypto.PubKey, Signature, error) {
	return c.pk, c.sig, nil
}
