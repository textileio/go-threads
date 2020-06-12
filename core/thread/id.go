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

	return ID(buf[:n+numlen])
}

// ID represents a self-describing thread identifier.
// It is formed by a Version, a Variant, and a random number
// of a given length.
type ID string

// Undef can be used to represent a nil or undefined Cid, using Cid{}
// directly is also acceptable.
var Undef = ID("")

// Defined returns true if an ID is defined.
// Calling any other methods on an undefined ID will result in
// undefined behavior.
func (i ID) Defined() bool {
	return i != Undef
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

// ExtractEncoding from an ID. If Decode on the same string did
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
	if err := validateIDData(data); err != nil {
		return Undef, err
	}
	return ID(data), nil
}

// FromAddr returns ID from a multiaddress if present.
func FromAddr(addr ma.Multiaddr) (ID, error) {
	idstr, err := addr.ValueForProtocol(Code)
	if err != nil {
		return Undef, err
	}
	return Decode(idstr)
}

// ToAddr returns ID wrapped as a multiaddress.
func ToAddr(id ID) ma.Multiaddr {
	addr, err := ma.NewMultiaddr("/" + Name + "/ " + string(id))
	if err != nil {
		panic(err) // This should not happen
	}
	return addr
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

// Validate the ID.
func (i ID) Validate() error {
	data := i.Bytes()
	return validateIDData(data)
}

func getVersion(data []byte) (uint64, int, error) {
	vers, n := binary.Uvarint(data)
	if err := uvError(n); err != nil {
		return 0, 0, err
	}
	return vers, n, nil
}

func validateIDData(data []byte) error {
	vers, n, err := getVersion(data)
	if err != nil {
		return err
	}

	if vers != V1 {
		return fmt.Errorf("expected 1 as the id version number, got: %d", vers)
	}

	variant, cn := binary.Uvarint(data[n:])
	if err := uvError(cn); err != nil {
		return err
	}

	if variant != uint64(Raw) && variant != uint64(AccessControlled) {
		return fmt.Errorf("expected Raw or AccessControlled as the id variant, got: %d", variant)
	}

	id := data[n+cn:]
	if len(id) == 0 {
		return fmt.Errorf("expected random id bytes but there are none")
	}

	return nil
}

// UnmarshalBinary is equivalent to Cast(). It implements the
// encoding.BinaryUnmarshaler interface.
func (i *ID) UnmarshalBinary(data []byte) error {
	id, err := Cast(data)
	if err != nil {
		return err
	}
	*i = id
	return nil
}

// UnmarshalText is equivalent to Decode(). It implements the
// encoding.TextUnmarshaler interface.
func (i *ID) UnmarshalText(text []byte) error {
	id, err := Decode(string(text))
	if err != nil {
		return err
	}
	*i = id
	return nil
}

// Version returns the ID version.
func (i ID) Version() uint64 {
	version, _, err := getVersion(i.Bytes())
	if err != nil {
		panic("error getting version: " + err.Error())
	}
	return version
}

// Variant returns the variant of an ID.
func (i ID) Variant() Variant {
	_, n := uvarint(string(i))
	variant, _ := uvarint(string(i)[n:])
	return Variant(variant)
}

// String returns the default string representation of an ID.
// Currently, Base32 is used as the encoding for the multibase string.
func (i ID) String() string {
	if err := i.Validate(); err != nil {
		panic("invalid thread id")
	}
	switch i.Version() {
	case V1:
		b := []byte(i)
		mbstr, err := mbase.Encode(mbase.Base32, b)
		if err != nil {
			panic("should not error with hardcoded mbase: " + err.Error())
		}

		return mbstr
	default:
		panic("unknown thread id version")
	}
}

// StringOfBase returns the string representation of an ID
// encoded is selected base.
func (i ID) StringOfBase(base mbase.Encoding) (string, error) {
	if err := i.Validate(); err != nil {
		panic("invalid thread id")
	}
	switch i.Version() {
	case V1:
		return mbase.Encode(base, i.Bytes())
	default:
		panic("unknown thread id version")
	}
}

// Encode return the string representation of an ID in a given base
// when applicable.
func (i ID) Encode(base mbase.Encoder) string {
	if err := i.Validate(); err != nil {
		panic("invalid thread id")
	}
	switch i.Version() {
	case V1:
		return base.Encode(i.Bytes())
	default:
		panic("unknown thread id version")
	}
}

// Bytes returns the byte representation of an ID.
// The output of bytes can be parsed back into an ID
// with Cast().
func (i ID) Bytes() []byte {
	return []byte(i)
}

// MarshalBinary is equivalent to Bytes(). It implements the
// encoding.BinaryMarshaler interface.
func (i ID) MarshalBinary() ([]byte, error) {
	return i.Bytes(), nil
}

// MarshalText is equivalent to String(). It implements the
// encoding.TextMarshaler interface.
func (i ID) MarshalText() ([]byte, error) {
	if err := i.Validate(); err != nil {
		panic("invalid thread id")
	}
	return []byte(i.String()), nil
}

// Equals checks that two IDs are the same.
func (i ID) Equals(o ID) bool {
	return i == o
}

// KeyString returns the binary representation of the ID as a string.
func (i ID) KeyString() string {
	return string(i)
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
func (s IDSlice) Less(i, j int) bool { return s[i] < s[j] }

// Info holds thread logs, keys and addresses.
type Info struct {
	ID    ID
	Key   Key
	Logs  []LogInfo
	Addrs []ma.Multiaddr
}

// GetOwnLog returns the first log found with a private key.
// This is a strict owership check, vs returning all directly 'managed' logs.
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
	// ID is the log's identifier.
	ID peer.ID
	// PubKey is the log's public key.
	PubKey crypto.PubKey
	// PrivKey is the log's private key.
	PrivKey crypto.PrivKey
	// Addrs are the addresses associated with the given log.
	Addrs []ma.Multiaddr
	// Head is the log's current head.
	Head cid.Cid
	// Managed logs are any logs directly added/created by the host, and/or logs for which we have the private key
	Managed bool
}
