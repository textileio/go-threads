package jwted25519_test

import (
	"strings"
	"testing"

	"github.com/golang-jwt/jwt"
	"github.com/libp2p/go-libp2p-core/crypto"
	. "github.com/textileio/go-threads/jwt"
)

var publicKey = "CAESIP1G8uGFpX+iduqgJfKLt0nw870MI9ydHcKg9gDIr5Tb"
var privateKey = "CAESYKcFG4UOHb1fyF+GlyGWfjfX47DH3y/K9fYMMMdy3Ow2/Uby4YWlf6J26qAl8ou3SfDzvQwj3J0dwqD2AMivlNv9RvLhhaV/onbqoCXyi7dJ8PO9DCPcnR3CoPYAyK+U2w=="

var ed25519TestData = []struct {
	name        string
	tokenString string
	alg         string
	claims      map[string]interface{}
	valid       bool
}{
	{
		"Ed25519",
		"eyJhbGciOiJFZDI1NTE5IiwidHlwIjoiSldUIn0.eyJqdGkiOiJmb28iLCJzdWIiOiJiYXIifQ.E73qcBjcCSsYto_Pa5CpwZUu9lA3ecCVkZ8pJiFYNaOe2x-uZCDmZnx52AByO78oxft09GosVcJtqYNv1VBxDQ",
		"Ed25519",
		map[string]interface{}{"jti": "foo", "sub": "bar"},
		true,
	},
	{
		"invalid key",
		"eyJhbGciOiJFZDI1NTE5IiwidHlwIjoiSldUIn0.eyJqdGkiOiJmb28iLCJzdWIiOiJiYXIifQ.7FSQFedbbRl42nvUWJqBswvjmyMaBBLKk0opiARjxtZmQ86dVMYs5wcZ0gItVV8YLVu6F5065IFD699tVcacBA",
		"Ed25519",
		map[string]interface{}{"jti": "foo", "sub": "bar"},
		false,
	},
}

func TestSigningMethodEd25519_Alg(t *testing.T) {
	if SigningMethodEd25519i.Alg() != "Ed25519" {
		t.Fatal("wrong alg")
	}
}

func TestSigningMethodEd25519_Sign(t *testing.T) {
	sk, err := unmarshalPrivateKeyFromString(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	for _, data := range ed25519TestData {
		if data.valid {
			parts := strings.Split(data.tokenString, ".")
			method := jwt.GetSigningMethod(data.alg)
			sig, err := method.Sign(strings.Join(parts[0:2], "."), sk)
			if err != nil {
				t.Errorf("[%v] error signing token: %v", data.name, err)
			}
			if sig != parts[2] {
				t.Errorf("[%v] incorrect signature.\nwas:\n%v\nexpecting:\n%v", data.name, sig, parts[2])
			}
		}
	}
}

func TestSigningMethodEd25519_Verify(t *testing.T) {
	pk, err := unmarshalPublicKeyFromString(publicKey)
	if err != nil {
		t.Fatal(err)
	}
	for _, data := range ed25519TestData {
		parts := strings.Split(data.tokenString, ".")

		method := jwt.GetSigningMethod(data.alg)
		err := method.Verify(strings.Join(parts[0:2], "."), parts[2], pk)
		if data.valid && err != nil {
			t.Errorf("[%v] error while verifying key: %v", data.name, err)
		}
		if !data.valid && err == nil {
			t.Errorf("[%v] invalid key passed validation", data.name)
		}
	}
}

func TestGenerateEd25519Token(t *testing.T) {
	sk, err := unmarshalPrivateKeyFromString(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	claims := &jwt.StandardClaims{
		Id:      "bar",
		Subject: "foo",
	}
	_, err = jwt.NewWithClaims(SigningMethodEd25519i, claims).SignedString(sk)
	if err != nil {
		t.Fatal(err)
	}
}

func unmarshalPrivateKeyFromString(key string) (crypto.PrivKey, error) {
	keyb, err := crypto.ConfigDecodeKey(key)
	if err != nil {
		return nil, err
	}
	return crypto.UnmarshalPrivateKey(keyb)
}

func unmarshalPublicKeyFromString(key string) (crypto.PubKey, error) {
	keyb, err := crypto.ConfigDecodeKey(key)
	if err != nil {
		return nil, err
	}
	return crypto.UnmarshalPublicKey(keyb)
}
