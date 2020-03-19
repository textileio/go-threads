package crypto

import (
	"fmt"

	ic "github.com/libp2p/go-libp2p-core/crypto"
	"github.com/textileio/go-threads/crypto/asymmetric"
	"github.com/textileio/go-threads/crypto/symmetric"
)

// EncryptionKey represents a key that can be used for encryption.
type EncryptionKey interface {
	// Encrypt bytes.
	Encrypt([]byte) ([]byte, error)

	// Marshal to bytes.
	MarshalBinary() ([]byte, error)
}

// EncryptionKey represents a key that can be used for decryption.
type DecryptionKey interface {
	EncryptionKey

	// Decrypt bytes.
	Decrypt([]byte) ([]byte, error)
}

// EncryptionKeyFromBytes returns an EncryptionKey from k.
func EncryptionKeyFromBytes(k []byte) (EncryptionKey, error) {
	pk, err := ic.UnmarshalPublicKey(k)
	if err == nil {
		aek, err := asymmetric.FromPubKey(pk)
		if err == nil {
			return aek, nil
		}
	}
	sk, err := symmetric.FromBytes(k)
	if err == nil {
		return sk, nil
	}

	return nil, fmt.Errorf("parse encryption key failed")
}

// DecryptionKeyFromBytes returns a DecryptionKey from k.
func DecryptionKeyFromBytes(k []byte) (DecryptionKey, error) {
	pk, err := ic.UnmarshalPrivateKey(k)
	if err == nil {
		adk, err := asymmetric.FromPrivKey(pk)
		if err == nil {
			return adk, nil
		}
	}
	sk, err := symmetric.FromBytes(k)
	if err == nil {
		return sk, nil
	}

	return nil, fmt.Errorf("parse decryption key failed")
}
