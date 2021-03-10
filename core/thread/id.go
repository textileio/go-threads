package thread

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/google/uuid"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	maddr "github.com/multiformats/go-multiaddr"
	mbase "github.com/multiformats/go-multibase"
	d "github.com/textileio/go-threads/core/did"
	jwted25519 "github.com/textileio/go-threads/jwt"
)

var (
	// ErrVarintBuffSmall indicates a buffer passed to the ID parser was not
	// long enough, or contained an invalid ID.
	ErrVarintBuffSmall = errors.New("reading varint: buffer too small")

	// ErrVarintTooBig indicates the varint in the given ID was above the limit of 2^64.
	ErrVarintTooBig = errors.New("reading varint: varint bigger than 64bits and not supported")

	// ErrIDTooShort indicates the ID passed to decode was not long enough to be a valid ID.
	ErrIDTooShort = errors.New("id too short")

	randomVariantSize = 20
)

// Version is a type for thread versions.
type Version uint64

const (
	// Version1 is the current thread ID version.
	Version1 Version = 0x01
)

func (v Version) String() string {
	switch v {
	case Version1:
		return "1"
	default:
		panic(fmt.Errorf("version %d is invalid", v))
	}
}

// Variant is a type for thread variants.
type Variant uint64

const (
	// RandomVariant IDs are generated from random bytes.
	RandomVariant Variant = 0x55
	// PubKeyVariant IDs are generated from multihash(multibase(key))
	PubKeyVariant Variant = 0x70
)

func (v Variant) String() string {
	switch v {
	case RandomVariant:
		return "random"
	case PubKeyVariant:
		return "pubkey"
	default:
		panic(fmt.Errorf("variant %d is invalid", v))
	}
}

// NewRandomIDV1 returns a new random Version1 ID.
func NewRandomIDV1() ID {
	p := make([]byte, randomVariantSize)
	if _, err := rand.Read(p); err != nil {
		panic(fmt.Errorf("random read: %v", err))
	}
	return newID(Version1, RandomVariant, p)
}

// NewPubKeyIDV1 returns a new pubkey Version1 ID.
func NewPubKeyIDV1(k PubKey) ID {
	p, err := k.Hash()
	if err != nil {
		panic(fmt.Errorf("getting key hash: %v", err))
	}
	return newID(Version1, PubKeyVariant, p)
}

func newID(version Version, variant Variant, payload []byte) ID {
	l := len(payload)
	// two 8 bytes (max) numbers plus num
	buf := make([]byte, 2*binary.MaxVarintLen64+l)
	n := binary.PutUvarint(buf, uint64(version))
	n += binary.PutUvarint(buf[n:], uint64(variant))
	c := copy(buf[n:], payload)
	if c != l {
		panic(errors.New("copy length is inconsistent"))
	}
	return ID(buf[:n+l])
}

// ID represents a self-describing thread identifier.
// It is formed by a Version, a Variant, and a random number of a given length.
type ID string

// Undef can be used to represent a nil or undefined Cid, using Cid{} directly is also acceptable.
var Undef = ID("")

// Defined returns true if an ID is defined.
// Calling any other methods on an undefined ID will result in undefined behavior.
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

// MustDecode panics if ID is not decodable.
func MustDecode(v string) ID {
	id, err := Decode(v)
	if err != nil {
		panic(errors.New("could not decode thread id"))
	}
	return id
}

// ExtractEncoding from an ID. If Decode on the same string did not return an error neither will this function.
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
func FromAddr(addr maddr.Multiaddr) (ID, error) {
	idstr, err := addr.ValueForProtocol(ProtocolCode)
	if err != nil {
		return Undef, err
	}
	return Decode(idstr)
}

// FromDID returns ID from a DID.
func FromDID(did d.DID) (ID, error) {
	decoded, err := did.Decode()
	if err != nil {
		return Undef, err
	}
	return Decode(decoded.ID)
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

// MustValidate panics if ID is not valid.
func (i ID) MustValidate() {
	if err := i.Validate(); err != nil {
		panic(errors.New("invalid thread id"))
	}
}

func getVersion(data []byte) (Version, int, error) {
	vers, n := binary.Uvarint(data)
	if err := uvError(n); err != nil {
		return 0, 0, err
	}
	return Version(vers), n, nil
}

func validateIDData(data []byte) error {
	vers, n, err := getVersion(data)
	if err != nil {
		return err
	}

	if vers != Version1 {
		return fmt.Errorf("expected 1 as the id version number, got: %d", vers)
	}

	variant, cn := binary.Uvarint(data[n:])
	if err := uvError(cn); err != nil {
		return err
	}

	if variant != uint64(RandomVariant) && variant != uint64(PubKeyVariant) {
		return fmt.Errorf("expected RandomVariant or PubKeyVariant as the id variant, got: %d", variant)
	}

	id := data[n+cn:]
	if len(id) == 0 {
		return fmt.Errorf("expected random id bytes but there are none")
	}

	return nil
}

// UnmarshalBinary is equivalent to Cast(). It implements the encoding.BinaryUnmarshaler interface.
func (i *ID) UnmarshalBinary(data []byte) error {
	id, err := Cast(data)
	if err != nil {
		return err
	}
	*i = id
	return nil
}

// UnmarshalText is equivalent to Decode(). It implements the encoding.TextUnmarshaler interface.
func (i *ID) UnmarshalText(text []byte) error {
	id, err := Decode(string(text))
	if err != nil {
		return err
	}
	*i = id
	return nil
}

// Version returns the ID version.
func (i ID) Version() Version {
	version, _, err := getVersion(i.Bytes())
	if err != nil {
		panic(fmt.Errorf("getting version: %v", err))
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
	i.MustValidate()
	switch i.Version() {
	case Version1:
		b := []byte(i)
		mbstr, err := mbase.Encode(mbase.Base32, b)
		if err != nil {
			panic(fmt.Errorf("should not error with hardcoded mbase: %v", err))
		}

		return mbstr
	default:
		panic(errors.New("unknown thread id version"))
	}
}

// StringOfBase returns the string representation of an ID encoded is selected base.
func (i ID) StringOfBase(base mbase.Encoding) (string, error) {
	i.MustValidate()
	switch i.Version() {
	case Version1:
		return mbase.Encode(base, i.Bytes())
	default:
		panic(errors.New("unknown thread id version"))
	}
}

// Encode return the string representation of an ID in a given base when applicable.
func (i ID) Encode(base mbase.Encoder) string {
	i.MustValidate()
	switch i.Version() {
	case Version1:
		return base.Encode(i.Bytes())
	default:
		panic(errors.New("unknown thread id version"))
	}
}

// DID returns a decentralized identifier in the form of did:thread:string(id).
func (i ID) DID() d.DID {
	return d.DID("did:thread:" + i.String())
}

// Bytes returns the byte representation of an ID.
// The output of bytes can be parsed back into an ID with Cast().
func (i ID) Bytes() []byte {
	return []byte(i)
}

// MarshalBinary is equivalent to Bytes(). It implements the encoding.BinaryMarshaler interface.
func (i ID) MarshalBinary() ([]byte, error) {
	return i.Bytes(), nil
}

// MarshalText is equivalent to String(). It implements the encoding.TextMarshaler interface.
func (i ID) MarshalText() ([]byte, error) {
	i.MustValidate()
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
// https://godoc.org/github.com/ipfs/go-log/v2).
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
	// ID is the thread's unique identifier.
	ID ID
	// Key wraps the thread's encryption keys.
	Key Key
	// Logs are the thread's currently known single-writer logs.
	Logs []LogInfo
	// Addrs are full addresses where the thread can be found without interacting with the peer DHT, e.g.,
	//     /ip4/<host_ip>/tcp/<host_port>/p2p/<peer_id_1>/thread/<thread_id>
	//     /dnsaddr/<host_name>/p2p/<peer_id_2>/thread/<thread_id>
	Addrs []maddr.Multiaddr
}

// Token returns a JWT-encoded verifiable claim representing thread info.
func (i Info) Token(issuer Identity, aud d.DID, dur time.Duration) (d.Token, error) {
	id := i.ID.DID()
	iss, err := issuer.GetPublic().DID()
	if err != nil {
		return "", err
	}
	services := make([]d.Service, len(i.Addrs))
	for i, a := range i.Addrs {
		p, err := a.ValueForProtocol(maddr.P_P2P)
		if err != nil {
			return "", err
		}
		services[i] = d.Service{
			ID:              d.DID(string(id) + "#" + p),
			Type:            "ThreadService",
			ServiceEndpoint: a.String(),
			ServiceProtocol: string(Protocol),
		}
	}
	claims := IdentityClaims{
		StandardClaims: jwt.StandardClaims{
			Id:        uuid.New().URN(),
			Subject:   string(id),
			Issuer:    string(iss),
			Audience:  string(aud),
			ExpiresAt: time.Now().Add(dur).Unix(),
			IssuedAt:  time.Now().Unix(),
			NotBefore: time.Now().Unix(),
		},
		VerifiableCredential: d.VerifiableCredential{
			Context: []string{
				"https://www.w3.org/2018/credentials/v1",
			},
			Type: []string{
				"VerifiableCredential",
			},
			CredentialSubject: d.VerifiableCredentialSubject{
				ID: id,
				Document: d.Document{
					Context: []string{
						"https://www.w3.org/ns/did/v1",
					},
					ID: id,
					Conroller: []d.DID{
						iss,
					},
					Authentication: []d.VerificationMethod{
						{
							ID:                 iss + "#keys-1",
							Type:               "Ed25519VerificationKey2018",
							Controller:         iss,
							PublicKeyMultiBase: issuer.GetPublic().String(),
						},
					},
					Services: services,
				},
			},
		},
	}
	t, err := jwt.NewWithClaims(jwted25519.SigningMethodEd25519i, claims).
		SignedString(issuer.(*Libp2pIdentity).PrivKey)
	if err != nil {
		return "", err
	}
	return d.Token(t), nil
}

// GetFirstPrivKeyLog returns the first log found with a private key.
// This is a strict owership check, vs returning all directly 'managed' logs.
// Deprecated: This is no longer safe to use.
func (i Info) GetFirstPrivKeyLog() *LogInfo {
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
	// Addrs are the peer addresses associated with the given log, e.g.,
	//     /p2p/<peer_id_1>
	//     /p2p/<peer_id_2>
	Addrs []maddr.Multiaddr
	// Head is the log's current head.
	Head cid.Cid
	// Managed logs are any logs directly added/created by the host, and/or logs for which we have the private key
	Managed bool
}
