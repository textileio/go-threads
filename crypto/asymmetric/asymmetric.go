package asymmetric

import (
	"crypto/rand"
	"fmt"

	extra "github.com/agl/ed25519/extra25519"
	ic "github.com/libp2p/go-libp2p-core/crypto"
	"golang.org/x/crypto/nacl/box"
)

const (
	// Length of nacl nonce
	NonceBytes = 24

	// Length of nacl ephemeral public key
	EphemeralPublicKeyBytes = 32
)

var (
	// Nacl box decryption failed
	BoxDecryptionError = fmt.Errorf("failed to decrypt curve25519")
)

// EncryptionKey is a public key wrapper that can perform encryption.
type EncryptionKey struct {
	pk ic.PubKey
}

// NewEncryptionKey returns a key by parsing k into a public key.
func NewEncryptionKey(pk ic.PubKey) (*EncryptionKey, error) {
	if _, ok := pk.(*ic.Ed25519PublicKey); !ok {
		return nil, fmt.Errorf("could not determine key type")
	}
	return &EncryptionKey{pk: pk}, nil
}

// Encrypt bytes with a public key.
func (k *EncryptionKey) Encrypt(plaintext []byte) ([]byte, error) {
	return encrypt(plaintext, k.pk)
}

// Marshal returns raw key bytes.
func (k *EncryptionKey) Marshal() ([]byte, error) {
	return ic.MarshalPublicKey(k.pk)
}

// DecryptionKey is a private key wrapper that can perform decryption.
type DecryptionKey struct {
	sk ic.PrivKey
}

// NewDecryptionKey returns a key by parsing k into a private key.
func NewDecryptionKey(sk ic.PrivKey) (*DecryptionKey, error) {
	if _, ok := sk.(*ic.Ed25519PrivateKey); !ok {
		return nil, fmt.Errorf("could not determine key type")
	}
	return &DecryptionKey{sk: sk}, nil
}

// Encrypt bytes with a public key.
func (k *DecryptionKey) Encrypt(plaintext []byte) ([]byte, error) {
	return encrypt(plaintext, k.sk.GetPublic())
}

// Decrypt ciphertext with a private key.
func (k *DecryptionKey) Decrypt(ciphertext []byte) ([]byte, error) {
	return decrypt(ciphertext, k.sk)
}

// Marshal returns raw key bytes.
func (k *DecryptionKey) Marshal() ([]byte, error) {
	return ic.MarshalPrivateKey(k.sk)
}

func encrypt(plaintext []byte, pk ic.PubKey) ([]byte, error) {
	ed25519Pubkey, ok := pk.(*ic.Ed25519PublicKey)
	if ok {
		return encryptCurve25519(ed25519Pubkey, plaintext)
	}
	return nil, fmt.Errorf("could not determine key type")
}

func decrypt(ciphertext []byte, sk ic.PrivKey) ([]byte, error) {
	ed25519Privkey, ok := sk.(*ic.Ed25519PrivateKey)
	if ok {
		return decryptCurve25519(ed25519Privkey, ciphertext)
	}
	return nil, fmt.Errorf("could not determine key type")
}

func publicToCurve25519(k *ic.Ed25519PublicKey) (*[32]byte, error) {
	var cp [32]byte
	var pk [32]byte
	r, err := k.Raw()
	if err != nil {
		return nil, err
	}
	copy(pk[:], r)
	success := extra.PublicKeyToCurve25519(&cp, &pk)
	if !success {
		return nil, fmt.Errorf("error converting ed25519 pubkey to curve25519 pubkey")
	}
	return &cp, nil
}

func encryptCurve25519(pubKey *ic.Ed25519PublicKey, bytes []byte) ([]byte, error) {
	// generated ephemeral key pair
	ephemPub, ephemPriv, err := box.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}

	// convert recipient's key into curve25519
	pk, err := publicToCurve25519(pubKey)
	if err != nil {
		return nil, err
	}

	// encrypt with nacl
	var ciphertext []byte
	var nonce [24]byte
	n := make([]byte, 24)
	_, err = rand.Read(n)
	if err != nil {
		return nil, err
	}
	for i := 0; i < 24; i++ {
		nonce[i] = n[i]
	}
	ciphertext = box.Seal(ciphertext, bytes, &nonce, pk, ephemPriv)

	// prepend the ephemeral public key
	ciphertext = append(ephemPub[:], ciphertext...)

	// prepend nonce
	ciphertext = append(nonce[:], ciphertext...)
	return ciphertext, nil
}

func decryptCurve25519(privKey *ic.Ed25519PrivateKey, ciphertext []byte) ([]byte, error) {
	curve25519Privkey, err := privateToCurve25519(privKey)
	if err != nil {
		return nil, err
	}

	var plaintext []byte

	n := ciphertext[:NonceBytes]
	ephemPubkeyBytes := ciphertext[NonceBytes : NonceBytes+EphemeralPublicKeyBytes]
	ct := ciphertext[NonceBytes+EphemeralPublicKeyBytes:]

	var ephemPubkey [32]byte
	for i := 0; i < 32; i++ {
		ephemPubkey[i] = ephemPubkeyBytes[i]
	}

	var nonce [24]byte
	for i := 0; i < 24; i++ {
		nonce[i] = n[i]
	}

	plaintext, success := box.Open(plaintext, ct, &nonce, &ephemPubkey, curve25519Privkey)
	if !success {
		return nil, BoxDecryptionError
	}
	return plaintext, nil
}

func privateToCurve25519(k *ic.Ed25519PrivateKey) (*[32]byte, error) {
	var cs [32]byte
	r, err := k.Raw()
	if err != nil {
		return nil, err
	}
	var sk [64]byte
	copy(sk[:], r)
	extra.PrivateKeyToCurve25519(&cs, &sk)
	return &cs, nil
}
