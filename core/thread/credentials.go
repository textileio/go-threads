package thread

import (
	"github.com/libp2p/go-libp2p-core/crypto"
)

type Signature []byte

type Credentials interface {
	ThreadID() ID
	Sign() (crypto.PubKey, Signature, error)
}

type DefaultCreds struct {
	ID ID
}

func NewDefaultCreds(id ID) Credentials {
	return DefaultCreds{ID: id}
}

func (n DefaultCreds) ThreadID() ID {
	return n.ID
}

func (n DefaultCreds) Sign() (crypto.PubKey, Signature, error) {
	return nil, nil, nil
}
