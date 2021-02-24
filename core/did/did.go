package did

import (
	"context"
	"strings"

	"github.com/grpc-ecosystem/go-grpc-middleware/util/metautils"
	"github.com/ockam-network/did"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// DID is a concrete type for a decentralized identifier.
// See https://www.w3.org/TR/did-core/#dfn-did-schemes.
type DID string

// NewKeyDID prefixes "did:key:" to an identifier.
func NewKeyDID(id string) DID {
	return DID("did:key:" + id)
}

// Decode returns info about a DID.
func (d DID) Decode() (*did.DID, error) {
	return did.Parse(string(d))
}

// Document is a DID document that describes a DID subject.
// See https://www.w3.org/TR/did-core/#dfn-did-documents.
type Document struct {
	Context        []string             `json:"@context"`
	ID             DID                  `json:"id"`
	Conroller      []DID                `json:"conroller,omitempty"`
	Authentication []VerificationMethod `json:"authentication"`
	Services       []Service            `json:"services,omitempty"`
}

// VerificationMethod describes how to authenticate or authorize interactions with a DID subject.
// See https://www.w3.org/TR/did-core/#dfn-verification-method.
type VerificationMethod struct {
	ID                 DID    `json:"id"`
	Type               string `json:"type"`
	Controller         DID    `json:"controller"`
	PublicKeyMultiBase string `json:"publicKeyMultiBase"`
}

// Service describes a service provided by the DID subject.
// See https://www.w3.org/TR/did-core/#dfn-service.
type Service struct {
	ID              DID    `json:"id"`
	Type            string `json:"type"`
	ServiceEndpoint string `json:"serviceEndpoint"`
	ServiceProtocol string `json:"serviceProtocol"`
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
	ID       DID      `json:"id"`
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
