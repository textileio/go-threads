package thread

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/gogo/status"
	"github.com/grpc-ecosystem/go-grpc-middleware/util/metautils"
	"github.com/libp2p/go-libp2p-core/crypto"
	jwted25519 "github.com/textileio/go-threads/jwt"
	"google.golang.org/grpc/codes"
)

// Identity represents an entity capable of signing a message
// and returning the associated public key for verification.
// In many cases, this will just be a private key, but callers
// can use any setup that suits their needs.
type Identity interface {
	// Sign the given bytes cryptographically.
	Sign(context.Context, []byte) ([]byte, error)
	// Return a public key paired with this identity.
	GetPublic() PubKey
}

// Libp2pIdentity wraps crypto.PrivKey, overwriting GetPublic with thread.PubKey.
type Libp2pIdentity struct {
	crypto.PrivKey
}

// NewLibp2pIdentity returns a new Libp2pIdentity.
func NewLibp2pIdentity(key crypto.PrivKey) Identity {
	return &Libp2pIdentity{PrivKey: key}
}

func (p *Libp2pIdentity) Sign(_ context.Context, msg []byte) ([]byte, error) {
	return p.PrivKey.Sign(msg)
}

func (p *Libp2pIdentity) GetPublic() PubKey {
	return NewLibp2pPubKey(p.PrivKey.GetPublic())
}

// Pubkey can be anything that provides a verify method.
type PubKey interface {
	// String encodes the public key into a base64 string.
	fmt.Stringer
	// UnmarshalString decodes a public key from a base64 string.
	UnmarshalString(string) error
	// Verify that 'sig' is the signed hash of 'data'
	Verify(data []byte, sig []byte) (bool, error)
}

// Libp2pPubKey wraps crypto.PubKey.
type Libp2pPubKey struct {
	crypto.PubKey
}

// NewLibp2pPubKey returns a new PubKey.
func NewLibp2pPubKey(key crypto.PubKey) PubKey {
	return &Libp2pPubKey{PubKey: key}
}

func (p *Libp2pPubKey) String() string {
	bytes, err := crypto.MarshalPublicKey(p)
	if err != nil {
		panic(err)
	}
	return base64.StdEncoding.EncodeToString(bytes)
}

func (p *Libp2pPubKey) UnmarshalString(str string) error {
	bytes, err := base64.StdEncoding.DecodeString(str)
	if err != nil {
		return err
	}
	p.PubKey, err = crypto.UnmarshalPublicKey(bytes)
	return err
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

// ValidateToken returns non-nil if token was issued by issuer.
// If token is present and valid, the embedded public key is returned.
// If token is not present, both the returned public key and error will be nil.
func ValidateToken(issuer crypto.PrivKey, token Token) (PubKey, error) {
	var ok bool
	issuer, ok = issuer.(*crypto.Ed25519PrivateKey)
	if !ok {
		log.Fatal("issuer must be an Ed25519PrivateKey")
	}
	if token == "" {
		return nil, nil
	}
	keyfunc := func(*jwt.Token) (interface{}, error) {
		return issuer.GetPublic(), nil
	}
	var claims jwt.StandardClaims
	tok, err := jwt.ParseWithClaims(string(token), &claims, keyfunc)
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
