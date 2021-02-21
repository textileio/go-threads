package did

import (
	"context"
	"strings"

	"github.com/grpc-ecosystem/go-grpc-middleware/util/metautils"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

/*
{
  "jti": "urn:uuid:288b92c1-46a6-4c0d-9e16-bb4be1ff5a36",
  "iss": "did:key:bcysaqaisecgbahlntn4ampcqasnwpepe7alrf2xzcw76azgjyh3qsjnz6zv4k",
  "sub": "did:key:bcysaqaisecgbahlntn4ampcqasnwpepe7alrf2xzcw76azgjyh3qsjnz6zv4k",
  "aud": "did:key:bcysaqaiseb6ghomt5cct3a3iysh6fwc6w7ibujfkk7oe6ohynh45olzpqcppw",
  "iat": 1613863106,
  "exp": 1613863166,
  "nbf": 1613863106,
  "vc": {
    "@context": [
      "https://www.w3.org/2018/credentials/v1"
    ],
    "type": [
      "VerifiableCredential"
    ],
    "credentialSubject": {
      "id": "did:key:bcysaqaisecgbahlntn4ampcqasnwpepe7alrf2xzcw76azgjyh3qsjnz6zv4k",
      "document": {
        "@context": [
          "https://www.w3.org/ns/did/v1"
        ],
        "id": "did:key:bcysaqaisecgbahlntn4ampcqasnwpepe7alrf2xzcw76azgjyh3qsjnz6zv4k",
        "authentication": [
          {
            "id": "did:key:bcysaqaisecgbahlntn4ampcqasnwpepe7alrf2xzcw76azgjyh3qsjnz6zv4k#keys-1",
            "type": "Ed25519VerificationKey2018",
            "controller": "did:key:bcysaqaisecgbahlntn4ampcqasnwpepe7alrf2xzcw76azgjyh3qsjnz6zv4k",
            "publicKeyMultiBase": "bbaareiemcaow3g3yay6fabe3m6i6j6axclvpsfn74bsmtqpxbes3t5tlyu"
          }
        ]
      }
    }
  }
}
*/

// DID is a concrete type for a decentralized identifier.
// See https://www.w3.org/TR/did-core/#dfn-did-schemes.
type DID string

// Document is a DID document that describes a DID subject.
// See https://www.w3.org/TR/did-core/#dfn-did-documents.
type Document struct {
	Context        []string             `json:"@context"`
	ID             string               `json:"id"`
	Conroller      []string             `json:"conroller,omitempty"`
	Authentication []VerificationMethod `json:"authentication"`
}

// VerificationMethod describes how to authenticate or authorize interactions with a DID subject.
// See https://www.w3.org/TR/did-core/#dfn-verification-method.
type VerificationMethod struct {
	ID                 string `json:"id"`
	Type               string `json:"type"`
	Controller         string `json:"controller"`
	PublicKeyMultiBase string `json:"publicKeyMultiBase"`
}

// VerifiableCredential is a set of claims about a subject.
// See https://www.w3.org/TR/vc-data-model/#dfn-verifiable-credentials.
type VerifiableCredential struct {
	Context           []string                    `json:"@context"`
	Type              []string                    `json:"type"`
	CredentialSubject VerifiableCredentialSubject `json:"credentialSubject"`
}

// VerifiableCredentialSubject is an entity about which claims are made.
// See https://www.w3.org/TR/vc-data-model/#dfn-subjects.
type VerifiableCredentialSubject struct {
	ID       string   `json:"id"`
	Document Document `json:"document,omitempty"`
}

// Token is a concrete type for a JWT token string.
type Token string

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

// RPCCredentials implements PerRPCCredentials, allowing context values
// to be included in request metadata.
type RPCCredentials struct {
	Secure bool
}

func (c RPCCredentials) GetRequestMetadata(ctx context.Context, _ ...string) (map[string]string, error) {
	md := map[string]string{}
	token, ok := TokenFromContext(ctx)
	if ok {
		md["authorization"] = "bearer " + string(token)
	}
	return md, nil
}

func (c RPCCredentials) RequireTransportSecurity() bool {
	return c.Secure
}
