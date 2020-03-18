package thread

import (
	"fmt"

	mbase "github.com/multiformats/go-multibase"
	sym "github.com/textileio/go-threads/crypto/symmetric"
)

var (
	// Indicates an invalid byte slice was given to KeyFromBytes.
	ErrInvalidKey = fmt.Errorf("invalid key")
)

// Key is a thread encryption key with two components.
// Service key is used to encrypt outer log record linkages.
// Read key is used to encrypt inner record events.
type Key struct {
	sk *sym.Key
	rk *sym.Key
}

// NewKey wraps service and read keys.
func NewKey(sk, rk *sym.Key) *Key {
	return &Key{sk: sk, rk: rk}
}

// NewRandomFullKey returns a random full key, which includes a service and read key.
func NewRandomFullKey() (*Key, error) {
	sk, err := sym.NewRandom()
	if err != nil {
		return nil, err
	}
	rk, err := sym.NewRandom()
	if err != nil {
		return nil, err
	}
	return &Key{sk: sk, rk: rk}, nil
}

// NewFullKey returns a full Key if err is nil and panics otherwise.
func NewFullKey() *Key {
	k, err := NewRandomFullKey()
	if err != nil {
		panic(err)
	}
	return k
}

// NewRandomServiceKey returns a random service-only key, which does not include a read key.
func NewRandomServiceKey() (*Key, error) {
	sk, err := sym.NewRandom()
	if err != nil {
		return nil, err
	}
	return &Key{sk: sk}, nil
}

// NewServiceKey returns a service-only Key if err is nil and panics otherwise.
func NewServiceKey() *Key {
	k, err := NewRandomServiceKey()
	if err != nil {
		panic(err)
	}
	return k
}

// KeyFromBytes returns a key by wrapping k.
func KeyFromBytes(k []byte) (*Key, error) {
	if len(k) != sym.KeyBytes && len(k) != sym.KeyBytes*2 {
		return nil, ErrInvalidKey
	}
	sk, err := sym.FromBytes(k[:sym.KeyBytes])
	if err != nil {
		return nil, err
	}
	var rk *sym.Key
	if len(k) == sym.KeyBytes*2 {
		rk, err = sym.FromBytes(k[sym.KeyBytes:])
		if err != nil {
			return nil, err
		}
	}
	return &Key{sk: sk, rk: rk}, nil
}

// KeyFromString returns a key by decoding a base58-encoded string.
func KeyFromString(k string) (*Key, error) {
	_, b, err := mbase.Decode(k)
	if err != nil {
		return nil, err
	}
	return KeyFromBytes(b)
}

// Service returns the service key.
func (k *Key) Service() *sym.Key {
	return k.sk
}

// Read returns the read key.
func (k *Key) Read() *sym.Key {
	return k.rk
}

// CanRead returns whether or not read key is available.
func (k *Key) CanRead() bool {
	return k.rk != nil
}

// Marshal returns raw key bytes while conforming to Marshaler.
func (k *Key) Marshal() ([]byte, error) {
	return k.Bytes(), nil
}

// Bytes returns raw key bytes.
func (k *Key) Bytes() []byte {
	if k.rk != nil {
		return append(k.sk.Bytes(), k.rk.Bytes()...)
	} else {
		return k.sk.Bytes()
	}
}

// String returns the base32-encoded string representation of raw key bytes.
// For example,
// Full: "brv7t5l2h55uklz5qwpntcat26csaasfchzof3emmdy6povabcd3a2to2qdkqdkto2prfhizerqqudqsdvwherbiy4nazqxjejgdr4oy"
// Service: "bp2vvqody5zm6yqycsnazb4kpqvycbdosos352zvpsorxce5koh7q"
func (k *Key) String() string {
	str, err := mbase.Encode(mbase.Base32, k.Bytes())
	if err != nil {
		panic("should not error with hardcoded mbase: " + err.Error())
	}
	return str
}
