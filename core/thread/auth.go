package thread

import (
	"fmt"

	"github.com/libp2p/go-libp2p-core/crypto"
)

// Auth provides a signed identity, which is needed for thread ACL checks.
type Auth struct {
	sk  crypto.PrivKey
	pk  crypto.PubKey
	msg []byte
	sig []byte
}

// NewAuthFromPrivKey returns auth from a private key.
// The auth must be signed before use.
func NewAuthFromPrivKey(privKey crypto.PrivKey) *Auth {
	return &Auth{sk: privKey}
}

// NewAuthFromSignature returns auth from a public key and signature.
func NewAuthFromSignature(pubKey crypto.PubKey, sig []byte) *Auth {
	return &Auth{pk: pubKey, sig: sig}
}

func (a *Auth) Sign(msg []byte) (sig []byte, pk crypto.PubKey, err error) {
	a.msg = msg
	if a.sk != nil {
		sig, err := a.sk.Sign(msg)
		if err != nil {
			return nil, nil, err
		}
		return sig, a.sk.GetPublic(), nil
	}
	return a.sig, a.pk, nil
}

func (a *Auth) Verify() (crypto.PubKey, error) {
	var pk crypto.PubKey
	if a.sk != nil {
		pk = a.sk.GetPublic()
	} else if a.pk != nil {
		pk = a.pk
	} else {
		return nil, fmt.Errorf("public key not found")
	}
	ok, err := pk.Verify(a.msg, a.sig)
	if !ok || err != nil {
		return nil, fmt.Errorf("bad signature")
	}
	return pk, nil
}
