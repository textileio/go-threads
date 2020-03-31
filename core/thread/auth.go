package thread

import (
	"bytes"
	"context"
	"encoding"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"strings"

	"github.com/gogo/status"
	"github.com/grpc-ecosystem/go-grpc-middleware/util/metautils"
	format "github.com/ipfs/go-ipld-format"
	"github.com/libp2p/go-libp2p-core/crypto"
	"google.golang.org/grpc/codes"
)

// Identity represents an entity capable of signing a message
// and returning the associated public key for verification.
// In many cases, this will just be a private key, but callers
// can use any setup that suits their use case.
type Identity interface {
	Sign([]byte) ([]byte, error)
	GetPublic() crypto.PubKey
}

// Auth simply wraps a public key and message signature.
type Auth struct {
	pk  crypto.PubKey
	sig []byte
}

// Method is included in signed messages as a means to
// restrict the caller's authorization to their real intention.
type Method byte

const (
	CreateThread  Method = 0x01
	AddThread            = 0x02
	GetThread            = 0x03
	PullThread           = 0x04
	DeleteThread         = 0x05
	AddReplicator        = 0x06
	CreateRecord         = 0x07
	AddRecord            = 0x08
	GetRecord            = 0x09
	Subscribe            = 0x0A
)

// NewAuth creates Auth from the given thread method and args,
// which is then signed by the identity.
func NewAuth(id Identity, method Method, args ...encoding.BinaryMarshaler) (auth Auth, err error) {
	if id == nil {
		return
	}
	msg, err := marshalBinary(method, args)
	if err != nil {
		return
	}
	sig, err := id.Sign(msg)
	if err != nil {
		return
	}
	return Auth{
		pk:  id.GetPublic(),
		sig: sig,
	}, nil
}

// NewAuthFromMD returns Auth from the given context, if present.
func NewAuthFromMD(ctx context.Context) (auth Auth, err error) {
	val := metautils.ExtractIncoming(ctx).Get("authorization")
	if val == "" {
		return
	}
	val = strings.TrimPrefix(val, "Signature ")
	parts := strings.SplitN(val, ",", 2)
	if len(parts) < 2 {
		return auth, status.Error(codes.Unauthenticated, "Bad authorization string")
	}
	pkstr := strings.TrimPrefix(parts[0], "keyId=")
	pkb, err := base64.StdEncoding.DecodeString(pkstr)
	if err != nil {
		return auth, status.Error(codes.Unauthenticated, "Bad authorization string")
	}
	pk, err := crypto.UnmarshalPublicKey(pkb)
	if err != nil {
		return auth, status.Error(codes.Unauthenticated, "Bad public key")
	}
	sigstr := strings.TrimPrefix(parts[1], "signature=")
	sig, err := base64.StdEncoding.DecodeString(sigstr)
	if err != nil {
		return auth, status.Error(codes.Unauthenticated, "Bad authorization string")
	}
	return Auth{
		pk:  pk,
		sig: sig,
	}, nil
}

// String returns a string form of Auth, which is used
// in an authorization header.
func (a Auth) String() string {
	if a.pk == nil {
		return ""
	}
	pkb, err := crypto.MarshalPublicKey(a.pk)
	if err != nil {
		panic(err)
	}
	keystr := base64.StdEncoding.EncodeToString(pkb)
	sigstr := base64.StdEncoding.EncodeToString(a.sig)
	return fmt.Sprintf("Signature keyId=%s,signature=%s", keystr, sigstr)
}

// Verify the given thread method and args against
// signature with public key.
func (a Auth) Verify(method Method, args ...encoding.BinaryMarshaler) (crypto.PubKey, error) {
	if a.pk == nil {
		return nil, nil
	}
	msg, err := marshalBinary(method, args)
	if err != nil {
		return nil, err
	}
	ok, err := a.pk.Verify(msg, a.sig)
	if !ok || err != nil {
		return nil, fmt.Errorf("bad signature")
	}
	return a.pk, nil
}

// BinaryNode adds BinaryMarshaler to format.Node.
type BinaryNode struct {
	format.Node
}

func NewBinaryNode(n format.Node) BinaryNode {
	return BinaryNode{Node: n}
}

func (n BinaryNode) MarshalBinary() ([]byte, error) {
	return n.RawData(), nil
}

func marshalBinary(method Method, args []encoding.BinaryMarshaler) ([]byte, error) {
	data := []byte{byte(method)}
	for _, arg := range args {
		b, err := arg.MarshalBinary()
		if err != nil {
			return nil, err
		}
		buf := new(bytes.Buffer)
		l := uint64(len(b))
		if err = binary.Write(buf, binary.LittleEndian, l); err != nil {
			return nil, err
		}
		if _, err = buf.Write(b); err != nil {
			return nil, err
		}
		data = append(data, buf.Bytes()...)
	}
	return data, nil
}

type ctxKey string

// NewAuthContext adds Auth to a context.
func NewAuthContext(ctx context.Context, auth Auth) context.Context {
	return context.WithValue(ctx, ctxKey("auth"), auth)
}

// AuthFromContext returns Auth from a context.
func AuthFromContext(ctx context.Context) Auth {
	auth, _ := ctx.Value(ctxKey("auth")).(Auth)
	return auth
}

// Credentials implements PerRPCCredentials, allowing context values
// to be included in request metadata.
type Credentials struct {
	Secure bool
}

func (c Credentials) GetRequestMetadata(ctx context.Context, _ ...string) (map[string]string, error) {
	md := map[string]string{}
	auth := AuthFromContext(ctx)
	if auth.pk != nil {
		md["authorization"] = auth.String()
	}
	return md, nil
}

func (c Credentials) RequireTransportSecurity() bool {
	return c.Secure
}
