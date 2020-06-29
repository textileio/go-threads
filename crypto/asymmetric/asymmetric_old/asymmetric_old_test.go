package asymmetric_old

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"testing"

	extra "github.com/agl/ed25519/extra25519"
	"github.com/textileio/go-threads/crypto/asymmetric"

	"github.com/libp2p/go-libp2p-core/crypto"
	"golang.org/x/crypto/nacl/box"
)

func TestCompatEncryptDecrypt(t *testing.T) {
	plaintext := "Hello, this is a compatibility test!!"
	priv, pub, err := crypto.GenerateKeyPair(crypto.Ed25519, 0)
	if err != nil {
		t.Error(err)
	}

	t.Run("Encrypt New -> Decrypt Old", func(t *testing.T) {
		ek, err := asymmetric.FromPubKey(pub)
		if err != nil {
			t.Error(err)
		}
		ciphertext, err := ek.Encrypt([]byte(plaintext))
		if err != nil {
			t.Error(err)
			return
		}

		dk, err := FromPrivKey(priv)
		if err != nil {
			t.Error(err)
		}

		decryptedPlaintext, err := dk.Decrypt(ciphertext)
		if err != nil {
			t.Error(err)
			return
		}
		if string(decryptedPlaintext) != plaintext {
			t.Error("result plaintext doesn't match original plaintext")
		}
	})
	t.Run("Encrypt Old -> Decrypt New", func(t *testing.T) {
		ek, err := FromPubKey(pub)
		if err != nil {
			t.Error(err)
		}
		ciphertext, err := ek.Encrypt([]byte(plaintext))
		if err != nil {
			t.Error(err)
			return
		}

		dk, err := asymmetric.FromPrivKey(priv)
		if err != nil {
			t.Error(err)
		}

		decryptedPlaintext, err := dk.Decrypt(ciphertext)
		if err != nil {
			t.Error(err)
			return
		}
		if string(decryptedPlaintext) != plaintext {
			t.Error("result plaintext doesn't match original plaintext")
		}
	})
}

// TestDecrypt is *exactly* the same test scenario as in asymmetric_test.go.
// Decryption for the *current* (newer) implementation must pass in asymmetric_test.go.
// Decryption for the old (deprecated) implementation must pass here.
func TestDecrypt(t *testing.T) {
	privKeyHex := "08011260e20c8d1e941df644b652af88c714f502c62ba19480e89837b67f21dd24dff4550d105e312db07495cbb516d69764c91107842de30f47dd591e9c69df16e4fd0d0d105e312db07495cbb516d69764c91107842de30f47dd591e9c69df16e4fd0d"
	ciphertextHex := "7974c0016a2bb90d6f132b666fc6c6e2955096a58f37b0e9a97bb43067e66dc21fe8dcc13a8534fcd27492e2fea85c002398c8f16698550b621da2a65d18cf66f6d4961380b051fe8408d8bd7f4cf3555e43eeb7e434"

	privKeyBytes, err := hex.DecodeString(privKeyHex)
	if err != nil {
		t.Error(err)
		return
	}
	sk, err := crypto.UnmarshalPrivateKey(privKeyBytes)
	if err != nil {
		t.Error(err)
		return
	}
	dk, err := FromPrivKey(sk)
	if err != nil {
		t.Error(err)
		return
	}

	cipherTextBytes, err := hex.DecodeString(ciphertextHex)
	if err != nil {
		t.Error(err)
		return
	}
	plaintext, err := dk.Decrypt(cipherTextBytes)
	if err != nil {
		t.Error(err)
		return
	}
	if "Hello World!!!" != string(plaintext) {
		t.Error("result plaintext doesn't match original plaintext")
		return
	}
}

// Below is the prior implementation of asymmetric.go
// It used a deprecated lib for elliptic curve transformation from
// Ed25519 to Curve25519.
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
	pk crypto.PubKey
}

// FromPubKey returns a key by parsing k into a public key.
func FromPubKey(pk crypto.PubKey) (*EncryptionKey, error) {
	if _, ok := pk.(*crypto.Ed25519PublicKey); !ok {
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
	return crypto.MarshalPublicKey(k.pk)
}

// DecryptionKey is a private key wrapper that can perform decryption.
type DecryptionKey struct {
	sk crypto.PrivKey
}

// FromPrivKey returns a key by parsing k into a private key.
func FromPrivKey(sk crypto.PrivKey) (*DecryptionKey, error) {
	if _, ok := sk.(*crypto.Ed25519PrivateKey); !ok {
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
	return crypto.MarshalPrivateKey(k.sk)
}

func encrypt(plaintext []byte, pk crypto.PubKey) ([]byte, error) {
	ed25519Pubkey, ok := pk.(*crypto.Ed25519PublicKey)
	if ok {
		return encryptCurve25519(ed25519Pubkey, plaintext)
	}
	return nil, fmt.Errorf("could not determine key type")
}

func decrypt(ciphertext []byte, sk crypto.PrivKey) ([]byte, error) {
	ed25519Privkey, ok := sk.(*crypto.Ed25519PrivateKey)
	if ok {
		return decryptCurve25519(ed25519Privkey, ciphertext)
	}
	return nil, fmt.Errorf("could not determine key type")
}

func publicToCurve25519(k *crypto.Ed25519PublicKey) (*[EphemeralPublicKeyBytes]byte, error) {
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

func encryptCurve25519(pubKey *crypto.Ed25519PublicKey, bytes []byte) ([]byte, error) {
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

func decryptCurve25519(privKey *crypto.Ed25519PrivateKey, ciphertext []byte) ([]byte, error) {
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

func privateToCurve25519(k *crypto.Ed25519PrivateKey) (*[EphemeralPublicKeyBytes]byte, error) {
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
