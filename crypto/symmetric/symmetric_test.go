package symmetric_test

import (
	"testing"

	. "github.com/textileio/go-threads/crypto/symmetric"
)

var symmetricTestData = struct {
	key        *Key
	plaintext  []byte
	ciphertext []byte
}{
	plaintext: []byte("Hello World!!!"),
}

func TestNewKey(t *testing.T) {
	key, err := CreateKey()
	if err != nil {
		t.Fatal(err)
	}
	symmetricTestData.key = key
}

func TestEncrypt(t *testing.T) {
	ciphertext, err := symmetricTestData.key.Encrypt(symmetricTestData.plaintext)
	if err != nil {
		t.Fatal(err)
	}
	symmetricTestData.ciphertext = ciphertext
}

func TestDecrypt(t *testing.T) {
	plaintext, err := symmetricTestData.key.Decrypt(symmetricTestData.ciphertext)
	if err != nil {
		t.Fatal(err)
	}
	if string(symmetricTestData.plaintext) != string(plaintext) {
		t.Error("decrypt AES failed")
	}
	key, err := CreateKey()
	if err != nil {
		t.Fatal(err)
	}
	plaintext, err = key.Decrypt(symmetricTestData.ciphertext)
	if err == nil {
		t.Error("decrypt AES with bad key succeeded")
	}
}
