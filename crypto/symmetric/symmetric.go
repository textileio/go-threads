package symmetric

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
)

// Key is a wrapper for a symmetric key.
type Key struct {
	raw []byte
}

// CreateKey returns 44 random bytes, 32 for the key and 12 for a nonce.
func CreateKey() (*Key, error) {
	raw := make([]byte, 44)
	if _, err := rand.Read(raw); err != nil {
		return nil, err
	}
	return &Key{raw: raw}, nil
}

// NewKey returns a key by wrapping k.
func NewKey(k []byte) (*Key, error) {
	if len(k) != 44 {
		return nil, fmt.Errorf("invalid key")
	}
	return &Key{raw: k}, nil
}

// Encrypt performs AES-256 GCM encryption on plaintext.
func (k *Key) Encrypt(plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(k.raw[:32])
	if err != nil {
		return nil, err
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	ciph := aesgcm.Seal(nil, k.raw[32:], plaintext, nil)
	return ciph, nil
}

// Decrypt uses key (:32 key, 32:12 nonce) to perform AES-256 GCM decryption on ciphertext.
func (k *Key) Decrypt(ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(k.raw[:32])
	if err != nil {
		return nil, err
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	plain, err := aesgcm.Open(nil, k.raw[32:], ciphertext, nil)
	if err != nil {
		return nil, err
	}
	return plain, nil
}

// Marshal returns raw key bytes.
func (k *Key) Marshal() ([]byte, error) {
	return k.raw, nil
}

// Bytes returns raw key bytes.
// This is cleaner than marshal when your not dealing with an interface type.
func (k *Key) Bytes() []byte {
	return k.raw
}
