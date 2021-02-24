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
	//hash, err := k.Hash()
	//if err != nil {
	//	return "", err
	//}
	//id, err := mbase.Encode(mbase.Base32, hash)
	//if err != nil {
	//	return "", err
	//}
	id, err := peer.IDFromPublicKey(k.PubKey)
	if err != nil {
		return "", err
	}
	return did.DID("did:key:" + id.String()), nil
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

/*
// Token is a concrete type for a JWT token string, which provides
// a claim to an identity.
type Token string

// ErrTokenNotFound indicates the token was not found in the context.
var ErrTokenNotFound = fmt.Errorf("thread token not found")

// ErrInvalidToken indicates the token is invalid.
var ErrInvalidToken = fmt.Errorf("invalid thread token")

// NewToken issues a new JWT token from issuer for the given pubic key.
func NewToken(issuer crypto.PrivKey, key PubKey) (tok Token, err error) {
	var ok bool
	issuer, ok = issuer.(*crypto.Ed25519PrivateKey)
	if !ok {
		log.Fatal("issuer must be an Ed25519PrivateKey")
	}
	claims := jwt.StandardClaims{
		Subject:  key.String(),
		Issuer:   NewLibp2pIdentity(issuer).GetPublic().String(),
		IssuedAt: time.Now().Unix(),
	}
	str, err := jwt.NewWithClaims(jwted25519.SigningMethodEd25519i, claims).SignedString(issuer)
	if err != nil {
		return
	}
	return Token(str), nil
}

// PubKey returns the public key encoded in the token.
// Note: This does NOT verify the token.
func (t Token) PubKey() (PubKey, error) {
	if t == "" {
		return nil, nil
	}
	var claims jwt.StandardClaims
	tok, _, err := new(jwt.Parser).ParseUnverified(string(t), &claims)
	if err != nil {
		if tok == nil {
			return nil, ErrTokenNotFound
		} else {
			return nil, ErrInvalidToken
		}
	}
	key := &Libp2pPubKey{}
	if err = key.UnmarshalString(claims.Subject); err != nil {
		return nil, err
	}
	return key, nil
}

// Validate token against an issuer.
// If token is present and was issued by issuer (is valid), the embedded public key is returned.
// If token is not present, both the returned public key and error will be nil.
func (t Token) Validate(issuer crypto.PrivKey) (PubKey, error) {
	if issuer == nil {
		return nil, fmt.Errorf("cannot validate with nil issuer")
	}
	var ok bool
	issuer, ok = issuer.(*crypto.Ed25519PrivateKey)
	if !ok {
		log.Fatal("issuer must be an Ed25519PrivateKey")
	}
	if t == "" {
		return nil, nil
	}
	keyfunc := func(*jwt.Token) (interface{}, error) {
		return issuer.GetPublic(), nil
	}
	var claims jwt.StandardClaims
	tok, err := jwt.ParseWithClaims(string(t), &claims, keyfunc)
	if err != nil {
		if tok == nil {
			return nil, ErrTokenNotFound
		} else {
			return nil, ErrInvalidToken
		}
	}
	key := &Libp2pPubKey{}
	if err = key.UnmarshalString(claims.Subject); err != nil {
		return nil, err
	}
	return key, nil
}

// Defined returns true if token is not empty.
func (t Token) Defined() bool {
	return t != ""
}

// NewTokenFromMD returns Token from the given context, if present.
func NewTokenFromMD(ctx context.Context) (tok Token, err error) {
	val := metautils.ExtractIncoming(ctx).Get("authorization")
	if val == "" {
		return
	}
	parts := strings.SplitN(val, " ", 2)
	if len(parts) < 2 {
		return "", status.Error(codes.Unauthenticated, "Bad authorization string")
	}
	if !strings.EqualFold(parts[0], "bearer") {
		return "", status.Errorf(codes.Unauthenticated, "Request unauthenticated with bearer")
	}
	return Token(parts[1]), nil
}

type ctxKey string

// NewTokenContext adds Token to a context.
func NewTokenContext(ctx context.Context, token Token) context.Context {
	if token == "" {
		return ctx
	}
	return context.WithValue(ctx, ctxKey("token"), token)
}

// TokenFromContext returns Token from a context.
func TokenFromContext(ctx context.Context) (Token, bool) {
	token, ok := ctx.Value(ctxKey("token")).(Token)
	return token, ok
}
*/
