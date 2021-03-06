package thread

import (
	"context"
	"encoding"
	"errors"
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/dgrijalva/jwt-go"
	"github.com/google/uuid"
	"github.com/libp2p/go-libp2p-core/crypto"
	mbase "github.com/multiformats/go-multibase"
	mhash "github.com/multiformats/go-multihash"
	"github.com/textileio/go-threads/core/did"
	"github.com/textileio/go-threads/crypto/asymmetric"
	jwted25519 "github.com/textileio/go-threads/jwt"
)

// Identity represents an entity capable of signing a message
// and returning the associated public key for verification.
// In many cases, this will just be a private key, but callers
// can use any setup that suits their needs.
type Identity interface {
	encoding.BinaryMarshaler
	encoding.BinaryUnmarshaler

	// String encodes the private key into a base32 string.
	fmt.Stringer
	// UnmarshalString decodes the private key from a base32 string.
	UnmarshalString(string) error
	// Sign the given bytes cryptographically.
	Sign(context.Context, []byte) ([]byte, error)
	// GetPublic returns the public key paired with this identity.
	GetPublic() PubKey
	// Decrypt returns decrypted data.
	Decrypt(context.Context, []byte) ([]byte, error)
	// Token returns a JWT-encoded verifiable claim to identity.
	Token(aud did.DID, dur time.Duration) (did.Token, error)
	// Equals returns true if the identities are equal.
	Equals(Identity) bool
}

// IdentityClaims defines a verifiable claim to an identity.
type IdentityClaims struct {
	jwt.StandardClaims
	VerifiableCredential did.VerifiableCredential `json:"vc"`
}

// Libp2pIdentity wraps crypto.PrivKey, overwriting GetPublic with thread.PubKey.
type Libp2pIdentity struct {
	crypto.PrivKey
}

// NewLibp2pIdentity returns a new Libp2pIdentity.
func NewLibp2pIdentity(key crypto.PrivKey) Identity {
	return &Libp2pIdentity{PrivKey: key}
}

func (i *Libp2pIdentity) MarshalBinary() ([]byte, error) {
	return crypto.MarshalPrivateKey(i.PrivKey)
}

func (i *Libp2pIdentity) UnmarshalBinary(bytes []byte) (err error) {
	i.PrivKey, err = crypto.UnmarshalPrivateKey(bytes)
	if err != nil {
		return err
	}
	return err
}

func (i *Libp2pIdentity) String() string {
	bytes, err := crypto.MarshalPrivateKey(i.PrivKey)
	if err != nil {
		panic(fmt.Errorf("marshal privkey: %v", err))
	}
	str, err := mbase.Encode(mbase.Base32, bytes)
	if err != nil {
		panic(fmt.Errorf("multibase encoding privkey: %v", err))
	}
	return str
}

func (i *Libp2pIdentity) UnmarshalString(str string) error {
	_, bytes, err := mbase.Decode(str)
	if err != nil {
		return err
	}
	i.PrivKey, err = crypto.UnmarshalPrivateKey(bytes)
	return err
}

func (i *Libp2pIdentity) Sign(_ context.Context, msg []byte) ([]byte, error) {
	return i.PrivKey.Sign(msg)
}

func (i *Libp2pIdentity) GetPublic() PubKey {
	return NewLibp2pPubKey(i.PrivKey.GetPublic())
}

func (i *Libp2pIdentity) Decrypt(_ context.Context, data []byte) ([]byte, error) {
	dk, err := asymmetric.FromPrivKey(i.PrivKey)
	if err != nil {
		return nil, err
	}
	return dk.Decrypt(data)
}

func (i *Libp2pIdentity) Token(aud did.DID, dur time.Duration) (did.Token, error) {
	id, err := i.GetPublic().DID()
	if err != nil {
		return "", err
	}
	claims := IdentityClaims{
		StandardClaims: jwt.StandardClaims{
			Id:        uuid.New().URN(),
			Subject:   string(id),
			Issuer:    string(id),
			Audience:  string(aud),
			ExpiresAt: time.Now().Add(dur).Unix(),
			IssuedAt:  time.Now().Unix(),
			NotBefore: time.Now().Unix(),
		},
		VerifiableCredential: did.VerifiableCredential{
			Context: []string{
				"https://www.w3.org/2018/credentials/v1",
			},
			Type: []string{
				"VerifiableCredential",
			},
			CredentialSubject: did.VerifiableCredentialSubject{
				ID: id,
				Document: did.Document{
					Context: []string{
						"https://www.w3.org/ns/did/v1",
					},
					ID: id,
					Authentication: []did.VerificationMethod{
						{
							ID:                 id + "#keys-1",
							Type:               "Ed25519VerificationKey2018",
							Controller:         id,
							PublicKeyMultiBase: i.GetPublic().String(),
						},
					},
				},
			},
		},
	}
	t, err := jwt.NewWithClaims(jwted25519.SigningMethodEd25519i, claims).SignedString(i.PrivKey)
	if err != nil {
		return "", err
	}
	return did.Token(t), nil
}

func (i *Libp2pIdentity) Equals(id Identity) bool {
	i2, ok := id.(*Libp2pIdentity)
	if !ok {
		return false
	}
	return i.PrivKey.Equals(i2.PrivKey)
}

// Pubkey can be anything that provides a verify method.
type PubKey interface {
	encoding.BinaryMarshaler
	encoding.BinaryUnmarshaler

	// String encodes the public key into a base32 string.
	fmt.Stringer
	// UnmarshalString decodes the public key from a base32 string.
	UnmarshalString(string) error
	// Verify that 'sig' is the signed hash of 'data'
	Verify(data []byte, sig []byte) (bool, error)
	// Encrypt data with the public key.
	Encrypt([]byte) ([]byte, error)
	// Hash returns a multihash of the key.
	Hash() ([]byte, error)
	// DID returns a decentralized identifier in the form of did:key:multibase(key).
	DID() (did.DID, error)
	// Validate parses and validates an identity token and returns the associated key.
	Validate(identity did.Token) (PubKey, did.Document, error)
	// Equals returns true if the keys are equal.
	Equals(PubKey) bool
}

// Libp2pPubKey wraps crypto.PubKey.
type Libp2pPubKey struct {
	crypto.PubKey
}

// NewLibp2pPubKey returns a new PubKey.
func NewLibp2pPubKey(key crypto.PubKey) PubKey {
	return &Libp2pPubKey{PubKey: key}
}

func (k *Libp2pPubKey) MarshalBinary() ([]byte, error) {
	return crypto.MarshalPublicKey(k.PubKey)
}

func (k *Libp2pPubKey) UnmarshalBinary(bytes []byte) (err error) {
	k.PubKey, err = crypto.UnmarshalPublicKey(bytes)
	if err != nil {
		return err
	}
	return err
}

func (k *Libp2pPubKey) String() string {
	bytes, err := crypto.MarshalPublicKey(k.PubKey)
	if err != nil {
		panic(fmt.Errorf("marshal pubkey: %v", err))
	}
	str, err := mbase.Encode(mbase.Base32, bytes)
	if err != nil {
		panic(fmt.Errorf("multibase encoding pubkey: %v", err))
	}
	return str
}

func (k *Libp2pPubKey) UnmarshalString(str string) error {
	_, bytes, err := mbase.Decode(str)
	if err != nil {
		return err
	}
	k.PubKey, err = crypto.UnmarshalPublicKey(bytes)
	return err
}

func (k *Libp2pPubKey) Encrypt(data []byte) ([]byte, error) {
	ek, err := asymmetric.FromPubKey(k.PubKey)
	if err != nil {
		return nil, err
	}
	return ek.Encrypt(data)
}

func (k *Libp2pPubKey) Hash() ([]byte, error) {
	bytes, err := k.MarshalBinary()
	if err != nil {
		return nil, err
	}
	hash, err := mhash.Encode(bytes, mhash.SHA3_256)
	if err != nil {
		return nil, err
	}
	if len(hash) > 20 {
		hash = hash[len(hash)-20:]
	}
	return hash, nil
}

func (k *Libp2pPubKey) DID() (did.DID, error) {
	id, err := peer.IDFromPublicKey(k.PubKey)
	if err != nil {
		return "", err
	}
	return did.NewKeyDID(id.String()), nil
}

func (k *Libp2pPubKey) Validate(identity did.Token) (key PubKey, doc did.Document, err error) {
	var claims IdentityClaims
	_, _, err = new(jwt.Parser).ParseUnverified(string(identity), &claims)
	if err != nil {
		return nil, doc, fmt.Errorf("parsing token: %v", err)
	}
	doc = claims.VerifiableCredential.CredentialSubject.Document

	// todo: check jti is uuid urn
	// todo: parse issuer, subject, audience as dids
	// todo: check issuer is subject
	// todo: check audience is this
	// todo: check credential subject id is sub
	// todo: check document id is credential subject id

	key = &Libp2pPubKey{}
	keyfunc := func(*jwt.Token) (interface{}, error) {
		if len(doc.Authentication) == 0 {
			return nil, errors.New("authentication not found")
		}
		// todo: get auth method that matches kid?
		_, bytes, err := mbase.Decode(doc.Authentication[0].PublicKeyMultiBase)
		if err != nil {
			return nil, fmt.Errorf("decoding key: %v", err)
		}
		if err := key.UnmarshalBinary(bytes); err != nil {
			return nil, fmt.Errorf("unmarshalling key: %v", err)
		}
		// todo: ckeck that key.DID() equals subject
		return key.(*Libp2pPubKey).PubKey, nil
	}
	if _, err := jwt.Parse(string(identity), keyfunc); err != nil {
		return nil, doc, fmt.Errorf("parsing token: %v", err)
	}
	return key, doc, nil
}

func (k *Libp2pPubKey) Equals(pk PubKey) bool {
	k2, ok := pk.(*Libp2pPubKey)
	if !ok {
		return false
	}
	return k.PubKey.Equals(k2.PubKey)
}
