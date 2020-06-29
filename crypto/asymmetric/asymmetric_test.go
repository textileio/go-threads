package asymmetric

import (
	"encoding/hex"
	"testing"

	"github.com/libp2p/go-libp2p-core/crypto"
)

func TestEncrypt(t *testing.T) {
	priv, pub, err := crypto.GenerateKeyPair(crypto.Ed25519, 0)
	if err != nil {
		t.Error(err)
	}

	ek, err := FromPubKey(pub)
	if err != nil {
		t.Error(err)
	}

	plaintext := "Hello World!!!"
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
}

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

	ciphertextInvalidHex := "7974c0016a2bb90d6f132b666fc6c6e2955096a58f37b0e9a97bb43067e66dc21fe8dcc13a8534fcd27492e2fea85c002398c8f166bd7f4cf3555e43eeb7e434"
	cipherTextInvalidBytes, err := hex.DecodeString(ciphertextInvalidHex)
	if err != nil {
		t.Error(err)
		return
	}
	_, err = dk.Decrypt(cipherTextInvalidBytes)
	if err != BoxDecryptionError {
		t.Error("failed to catch curve25519 decryption error")
	}
}
