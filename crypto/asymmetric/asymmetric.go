package asymmetric

import (
	"crypto/rand"
	"fmt"

	extra "github.com/agl/ed25519/extra25519"
	ic "github.com/libp2p/go-libp2p-core/crypto"
	"golang.org/x/crypto/nacl/box"
)

const (
	// NonceBytes is the length of nacl nonce.
	NonceBytes = 24

	// EphemeralPublicKeyBytes is the length of nacl ephemeral public key.
	EphemeralPublicKeyBytes = 32
)

var (
	// Nacl box decryption failed.
	BoxDecryptionError = fmt.Errorf("failed to decrypt curve25519")
)

// EncryptionKey is a public key wrapper that can perform encryption.
type EncryptionKey struct {
	pk ic.PubKey
}

// FromPubKey returns a key by parsing k into a public key.
func FromPubKey(pk ic.PubKey) (*EncryptionKey, error) {
	if _, ok := pk.(*ic.Ed25519PublicKey); !ok {
		return nil, fmt.Errorf("could not determine key type")
	}
	return &EncryptionKey{pk: pk}, nil
}

// Encrypt bytes with a public key.
func (k *EncryptionKey) Encrypt(plaintext []byte) ([]byte, error) {
	return encrypt(plaintext, k.pk)
}

// MarshalBinary implements BinaryMarshaler.
func (k *EncryptionKey) MarshalBinary() ([]byte, error) {
	return ic.MarshalPublicKey(k.pk)
}

// DecryptionKey is a private key wrapper that can perform decryption.
type DecryptionKey struct {
	sk ic.PrivKey
}

// FromPrivKey returns a key by parsing k into a private key.
func FromPrivKey(sk ic.PrivKey) (*DecryptionKey, error) {
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

// MarshalBinary implements BinaryMarshaler.
func (k *DecryptionKey) MarshalBinary() ([]byte, error) {
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

func publicToCurve25519(k *ic.Ed25519PublicKey) (*[EphemeralPublicKeyBytes]byte, error) {
	var cp [EphemeralPublicKeyBytes]byte
	var pk [EphemeralPublicKeyBytes]byte
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
	var nonce [NonceBytes]byte
	n := make([]byte, NonceBytes)
	if _, err = rand.Read(n); err != nil {
		return nil, err
	}
	for i := 0; i < NonceBytes; i++ {
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

	var ephemPubkey [EphemeralPublicKeyBytes]byte
	for i := 0; i < EphemeralPublicKeyBytes; i++ {
		ephemPubkey[i] = ephemPubkeyBytes[i]
	}

	var nonce [NonceBytes]byte
	for i := 0; i < NonceBytes; i++ {
		nonce[i] = n[i]
	}

	plaintext, success := box.Open(plaintext, ct, &nonce, &ephemPubkey, curve25519Privkey)
	if !success {
		return nil, BoxDecryptionError
	}
	return plaintext, nil
}

func privateToCurve25519(k *ic.Ed25519PrivateKey) (*[EphemeralPublicKeyBytes]byte, error) {
	var cs [EphemeralPublicKeyBytes]byte
	r, err := k.Raw()
	if err != nil {
		return nil, err
	}
	var sk [64]byte
	copy(sk[:], r)
	extra.PrivateKeyToCurve25519(&cs, &sk)
	return &cs, nil
}
