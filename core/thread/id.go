package thread

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"
	mbase "github.com/multiformats/go-multibase"
	sym "github.com/textileio/go-threads/crypto/symmetric"
)

var (
	// ErrVarintBuffSmall means that a buffer passed to the ID parser was not
	// long enough, or did not contain an invalid ID.
	ErrVarintBuffSmall = fmt.Errorf("reading varint: buffer too small")

	// ErrVarintTooBig means that the varint in the given ID was above the
	// limit of 2^64.
	ErrVarintTooBig = fmt.Errorf("reading varint: varint bigger than 64bits" +
		" and not supported")

	// ErrIDTooShort means that the ID passed to decode was not long
	// enough to be a valid ID.
	ErrIDTooShort = fmt.Errorf("id too short")
)

// Versions.
const (
	V1 = 0x01
)

// Variant is a type for thread variants.
type Variant uint64

// Variants.
const (
	Raw              Variant = 0x55
	AccessControlled Variant = 0x70 // Supports access control lists
)

func (v Variant) String() string {
	switch v {
	case Raw:
		return "raw"
	case AccessControlled:
		return "access_controlled"
	default:
		panic(fmt.Errorf("variant %d is invalid", v))
	}
}

// NewIDV1 returns a new random ID using the given variant.
func NewIDV1(variant Variant, size uint8) ID {
	num := make([]byte, size)
	_, err := rand.Read(num)
	if err != nil {
		panic("random read failed")
	}

	numlen := len(num)
	// two 8 bytes (max) numbers plus num
	buf := make([]byte, 2*binary.MaxVarintLen64+numlen)
	n := binary.PutUvarint(buf, V1)
	n += binary.PutUvarint(buf[n:], uint64(variant))
	cn := copy(buf[n:], num)
	if cn != numlen {
		panic("copy length is inconsistent")
	}

	return ID{string(buf[:n+numlen])}
}

// ID represents a self-describing thread identifier.
// It is formed by a Version, a Variant, and a random number
// of a given length.
type ID struct{ str string }

// Undef can be used to represent a nil or undefined Cid, using Cid{}
// directly is also acceptable.
var Undef = ID{}

// Defined returns true if an ID is defined.
// Calling any other methods on an undefined ID will result in
// undefined behavior.
func (i ID) Defined() bool {
	return i.str != ""
}

// Decode parses an ID-encoded string and returns an ID object.
// For IDV1, an ID-encoded string is primarily a multibase string:
//
//     <multibase-type-code><base-encoded-string>
//
// The base-encoded string represents a:
//
// <version><variant><random-number>
func Decode(v string) (ID, error) {
	if len(v) < 2 {
		return Undef, ErrIDTooShort
	}

	_, data, err := mbase.Decode(v)
	if err != nil {
		return Undef, err
	}

	return Cast(data)
}

// Extract the encoding from an ID. If Decode on the same string did
// not return an error neither will this function.
func ExtractEncoding(v string) (mbase.Encoding, error) {
	if len(v) < 2 {
		return -1, ErrIDTooShort
	}

	encoding := mbase.Encoding(v[0])

	// check encoding is valid
	_, err := mbase.NewEncoder(encoding)
	if err != nil {
		return -1, err
	}

	return encoding, nil
}

// Cast takes an ID data slice, parses it and returns an ID.
// For IDV1, the data buffer is in the form:
//
//     <version><variant><random-number>
//
// Please use decode when parsing a regular ID string, as Cast does not
// expect multibase-encoded data. Cast accepts the output of ID.Bytes().
func Cast(data []byte) (ID, error) {
	vers, n := binary.Uvarint(data)
	if err := uvError(n); err != nil {
		return Undef, err
	}

	if vers != 1 {
		return Undef, fmt.Errorf("expected 1 as the id version number, got: %d", vers)
	}

	_, cn := binary.Uvarint(data[n:])
	if err := uvError(cn); err != nil {
		return Undef, err
	}

	id := data[n+cn:]

	return ID{string(data[0 : n+cn+len(id)])}, nil
}

func uvError(read int) error {
	switch {
	case read == 0:
		return ErrVarintBuffSmall
	case read < 0:
		return ErrVarintTooBig
	default:
		return nil
	}
}

// UnmarshalBinary is equivalent to Cast(). It implements the
// encoding.BinaryUnmarshaler interface.
func (i *ID) UnmarshalBinary(data []byte) error {
	casted, err := Cast(data)
	if err != nil {
		return err
	}
	i.str = casted.str
	return nil
}

// UnmarshalText is equivalent to Decode(). It implements the
// encoding.TextUnmarshaler interface.
func (i *ID) UnmarshalText(text []byte) error {
	decodedID, err := Decode(string(text))
	if err != nil {
		return err
	}
	i.str = decodedID.str
	return nil
}

// Version returns the ID version.
func (i ID) Version() uint64 {
	return V1
}

// Variant returns the variant of an ID.
func (i ID) Variant() Variant {
	_, n := uvarint(i.str)
	variant, _ := uvarint(i.str[n:])
	return Variant(variant)
}

// String returns the default string representation of an ID.
// Currently, Base32 is used as the encoding for the multibase string.
func (i ID) String() string {
	switch i.Version() {
	case V1:
		b := []byte(i.str)
		mbstr, err := mbase.Encode(mbase.Base32, b)
		if err != nil {
			panic("should not error with hardcoded mbase: " + err.Error())
		}

		return mbstr
	default:
		panic("not possible to reach this point")
	}
}

// String returns the string representation of an ID
// encoded is selected base.
func (i ID) StringOfBase(base mbase.Encoding) (string, error) {
	switch i.Version() {
	case V1:
		return mbase.Encode(base, i.Bytes())
	default:
		panic("not possible to reach this point")
	}
}

// Encode return the string representation of an ID in a given base
// when applicable.
func (i ID) Encode(base mbase.Encoder) string {
	switch i.Version() {
	case V1:
		return base.Encode(i.Bytes())
	default:
		panic("not possible to reach this point")
	}
}

// Bytes returns the byte representation of an ID.
// The output of bytes can be parsed back into an ID
// with Cast().
func (i ID) Bytes() []byte {
	return []byte(i.str)
}

// MarshalBinary is equivalent to Bytes(). It implements the
// encoding.BinaryMarshaler interface.
func (i ID) MarshalBinary() ([]byte, error) {
	return i.Bytes(), nil
}

// MarshalText is equivalent to String(). It implements the
// encoding.TextMarshaler interface.
func (i ID) MarshalText() ([]byte, error) {
	return []byte(i.String()), nil
}

// Equals checks that two IDs are the same.
func (i ID) Equals(o ID) bool {
	return i == o
}

// KeyString returns the binary representation of the ID as a string.
func (i ID) KeyString() string {
	return i.str
}

// Loggable returns a Loggable (as defined by
// https://godoc.org/github.com/ipfs/go-log).
func (i ID) Loggable() map[string]interface{} {
	return map[string]interface{}{
		"id": i,
	}
}

// IDSlice for sorting threads.
type IDSlice []ID

func (s IDSlice) Len() int           { return len(s) }
func (s IDSlice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s IDSlice) Less(i, j int) bool { return s[i].str < s[j].str }

// Info holds thread logs and keys.
type Info struct {
	ID        ID
	Logs      []LogInfo
	FollowKey *sym.Key
	ReadKey   *sym.Key
}

// GetOwnLog returns the first log found with a private key.
func (i Info) GetOwnLog() *LogInfo {
	for _, lg := range i.Logs {
		if lg.PrivKey != nil {
			return &lg
		}
	}
	return nil
}

// LogInfo holds log keys, addresses, and heads.
type LogInfo struct {
	ID      peer.ID
	PubKey  crypto.PubKey
	PrivKey crypto.PrivKey
	Addrs   []ma.Multiaddr
	Heads   []cid.Cid
}
