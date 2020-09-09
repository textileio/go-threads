package thread

import (
	"context"
	"encoding"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/gogo/status"
	"github.com/grpc-ecosystem/go-grpc-middleware/util/metautils"
	"github.com/libp2p/go-libp2p-core/crypto"
	mbase "github.com/multiformats/go-multibase"
	"github.com/textileio/go-threads/crypto/asymmetric"
	jwted25519 "github.com/textileio/go-threads/jwt"
	"google.golang.org/grpc/codes"
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
	// Equals returns true if the identities are equal.
	Equals(Identity) bool
}

// Libp2pIdentity wraps crypto.PrivKey, overwriting GetPublic with thread.PubKey.
type Libp2pIdentity struct {
	crypto.PrivKey
}

// NewLibp2pIdentity returns a new Libp2pIdentity.
func NewLibp2pIdentity(key crypto.PrivKey) Identity {
	return &Libp2pIdentity{PrivKey: key}
}

func (p *Libp2pIdentity) MarshalBinary() ([]byte, error) {
	return crypto.MarshalPrivateKey(p.PrivKey)
}

func (p *Libp2pIdentity) UnmarshalBinary(bytes []byte) (err error) {
	p.PrivKey, err = crypto.UnmarshalPrivateKey(bytes)
	if err != nil {
		return err
	}
	return err
}

func (p *Libp2pIdentity) Sign(_ context.Context, msg []byte) ([]byte, error) {
	return p.PrivKey.Sign(msg)
}

func (p *Libp2pIdentity) GetPublic() PubKey {
	return NewLibp2pPubKey(p.PrivKey.GetPublic())
}

func (p *Libp2pIdentity) Decrypt(_ context.Context, data []byte) ([]byte, error) {
	dk, err := asymmetric.FromPrivKey(p.PrivKey)
	if err != nil {
		return nil, err
	}
	return dk.Decrypt(data)
}

func (p *Libp2pIdentity) Equals(i Identity) bool {
	li, ok := i.(*Libp2pIdentity)
	if !ok {
		return false
	}
	return p.PrivKey.Equals(li.PrivKey)
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

func (p *Libp2pPubKey) MarshalBinary() ([]byte, error) {
	return crypto.MarshalPublicKey(p.PubKey)
}

func (p *Libp2pPubKey) UnmarshalBinary(bytes []byte) (err error) {
	p.PubKey, err = crypto.UnmarshalPublicKey(bytes)
	if err != nil {
		return err
	}
	return err
}

func (p *Libp2pPubKey) String() string {
	bytes, err := crypto.MarshalPublicKey(p.PubKey)
	if err != nil {
		panic(err)
	}
	str, err := mbase.Encode(mbase.Base32, bytes)
	if err != nil {
		panic(err)
	}
	return str
}

func (p *Libp2pPubKey) UnmarshalString(str string) error {
	_, bytes, err := mbase.Decode(str)
	if err != nil {
		return err
	}
	p.PubKey, err = crypto.UnmarshalPublicKey(bytes)
	return err
}

func (p *Libp2pPubKey) Encrypt(data []byte) ([]byte, error) {
	ek, err := asymmetric.FromPubKey(p.PubKey)
	if err != nil {
		return nil, err
	}
	return ek.Encrypt(data)
}

func (p *Libp2pPubKey) Equals(k PubKey) bool {
	lk, ok := k.(*Libp2pPubKey)
	if !ok {
		return false
	}
	return p.PubKey.Equals(lk.PubKey)
}

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
	var ok bool
	if t == "" {
		return nil, nil
	}
	issuer, ok = issuer.(*crypto.Ed25519PrivateKey)
	if !ok {
		log.Fatal("issuer must be an Ed25519PrivateKey")
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

// Credentials implements PerRPCCredentials, allowing context values
// to be included in request metadata.
type Credentials struct {
	Secure bool
}

func (c Credentials) GetRequestMetadata(ctx context.Context, _ ...string) (map[string]string, error) {
	md := map[string]string{}
	token, ok := TokenFromContext(ctx)
	if ok {
		md["authorization"] = "bearer " + string(token)
	}
	return md, nil
}

func (c Credentials) RequireTransportSecurity() bool {
	return c.Secure
}
