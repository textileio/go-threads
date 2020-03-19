package thread

import (
	"fmt"

	mbase "github.com/multiformats/go-multibase"
	sym "github.com/textileio/go-threads/crypto/symmetric"
)

var (
	// ErrInvalidKey indicates an invalid byte slice was given to KeyFromBytes.
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
func NewKey(sk, rk *sym.Key) Key {
	if sk == nil {
		panic("service-key must not be nil")
	}
	return Key{sk: sk, rk: rk}
}

// NewServiceKey wraps a service-only key.
func NewServiceKey(sk *sym.Key) Key {
	return Key{sk: sk}
}

// NewRandomKey returns a random key, which includes a service and read key.
func NewRandomKey() Key {
	return Key{sk: sym.New(), rk: sym.New()}
}

// NewRandomServiceKey returns a random service-only key.
func NewRandomServiceKey() Key {
	return Key{sk: sym.New()}
}

// KeyFromBytes returns a key by wrapping k.
func KeyFromBytes(b []byte) (k Key, err error) {
	if len(b) != sym.KeyBytes && len(b) != sym.KeyBytes*2 {
		return k, ErrInvalidKey
	}
	sk, err := sym.FromBytes(b[:sym.KeyBytes])
	if err != nil {
		return k, err
	}
	var rk *sym.Key
	if len(b) == sym.KeyBytes*2 {
		rk, err = sym.FromBytes(b[sym.KeyBytes:])
		if err != nil {
			return k, err
		}
	}
	return Key{sk: sk, rk: rk}, nil
}

// KeyFromString returns a key by decoding a base32-encoded string.
func KeyFromString(s string) (k Key, err error) {
	_, b, err := mbase.Decode(s)
	if err != nil {
		return k, err
	}
	return KeyFromBytes(b)
}

// Service returns the service key.
func (k Key) Service() *sym.Key {
	return k.sk
}

// Read returns the read key.
func (k Key) Read() *sym.Key {
	return k.rk
}

// Defined returns whether or not key has any defined components.
// Since it's not possible to have a read key w/o a service key,
// we just need to check service key.
func (k Key) Defined() bool {
	return k.sk != nil
}

// CanRead returns whether or not read key is available.
func (k Key) CanRead() bool {
	return k.rk != nil
}

// MarshalBinary implements BinaryMarshaler.
func (k Key) MarshalBinary() ([]byte, error) {
	return k.Bytes(), nil
}

// Bytes returns raw key bytes.
func (k Key) Bytes() []byte {
	if k.rk != nil {
		return append(k.sk.Bytes(), k.rk.Bytes()...)
	} else if k.sk != nil {
		return k.sk.Bytes()
	} else {
		return nil
	}
}

// String returns the base32-encoded string representation of raw key bytes.
// For example,
// Full: "brv7t5l2h55uklz5qwpntcat26csaasfchzof3emmdy6povabcd3a2to2qdkqdkto2prfhizerqqudqsdvwherbiy4nazqxjejgdr4oy"
// Service: "bp2vvqody5zm6yqycsnazb4kpqvycbdosos352zvpsorxce5koh7q"
func (k Key) String() string {
	str, err := mbase.Encode(mbase.Base32, k.Bytes())
	if err != nil {
		panic("should not error with hardcoded mbase: " + err.Error())
	}
	return str
}
