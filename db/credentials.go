package db

import (
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/textileio/go-threads/core/thread"
)

type credentials struct {
	threadID thread.ID
	privKey  crypto.PrivKey
}

func (c credentials) ThreadID() thread.ID {
	return c.threadID
}

func (c credentials) Sign() (crypto.PubKey, thread.Signature, error) {
	if c.privKey == nil {
		return nil, nil, nil
	}
	sig, err := c.privKey.Sign(c.threadID.Bytes())
	if err != nil {
		return nil, nil, nil
	}
	return c.privKey.GetPublic(), sig, nil
}
