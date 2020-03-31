package thread

import (
	"context"
	"encoding"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/gogo/status"
	"github.com/grpc-ecosystem/go-grpc-middleware/util/metautils"
	"github.com/libp2p/go-libp2p-core/crypto"
	"google.golang.org/grpc/codes"
)

type Identity interface {
	Sign([]byte) ([]byte, error)
	GetPublic() crypto.PubKey
}

type Auth struct {
	pk  crypto.PubKey
	sig []byte
}

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

func marshalBinary(method Method, args []encoding.BinaryMarshaler) ([]byte, error) {
	bytes := []byte{byte(method)}
	for _, arg := range args {
		d, err := arg.MarshalBinary()
		if err != nil {
			return nil, err
		}
		bytes = append(bytes, d...)
	}
	return bytes, nil
}

type ctxKey string

func NewAuthContext(ctx context.Context, auth Auth) context.Context {
	return context.WithValue(ctx, ctxKey("auth"), auth)
}

func AuthFromContext(ctx context.Context) Auth {
	auth, _ := ctx.Value(ctxKey("auth")).(Auth)
	return auth
}

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
