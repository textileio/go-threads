package thread

import (
	"crypto/rand"
	"fmt"
	"testing"

	mbase "github.com/multiformats/go-multibase"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRandomIDV1(t *testing.T) {

}

func TestNewPubKeyIDV1(t *testing.T) {

}

func TestDecode(t *testing.T) {
	id := NewRandomIDV1()
	decoded, err := Decode(id.String())
	require.NoError(t, err)
	assert.True(t, decoded.Equals(id))
}

func TestExtractEncoding(t *testing.T) {
	e, err := ExtractEncoding(NewRandomIDV1().String())
	require.NoError(t, err)
	assert.Equal(t, mbase.Base32, int32(e))
}

func TestCast(t *testing.T) {
	id := NewRandomIDV1()
	casted, err := Cast(id.Bytes())
	require.NoError(t, err)
	assert.True(t, casted.Equals(id))
}

func TestFromAddr(t *testing.T) {

}

func TestToAddr(t *testing.T) {

}

func TestID_Defined(t *testing.T) {

}

func TestID_Validate(t *testing.T) {
	require.NoError(t, NewRandomIDV1().Validate())
	require.NoError(t, NewPubKeyIDV1(makeLibp2pIdentity(t).GetPublic()).Validate())
	require.Error(t, newID(Version(5), RandomVariant, randBytes(t, 16)).Validate())
	require.Error(t, newID(Version1, Variant(50), randBytes(t, 16)).Validate())
	require.Error(t, newID(Version1, RandomVariant, randBytes(t, 0)).Validate())
}

func TestID_UnmarshalBinary(t *testing.T) {

}

func TestID_UnmarshalText(t *testing.T) {

}

func TestID_Version(t *testing.T) {
	assert.Equal(t, Version1, NewRandomIDV1().Version())
	assert.Equal(t, Version1, NewPubKeyIDV1(makeLibp2pIdentity(t).GetPublic()).Version())
}

func TestID_Variant(t *testing.T) {
	assert.Equal(t, RandomVariant, NewRandomIDV1().Variant())
	assert.Equal(t, PubKeyVariant, NewPubKeyIDV1(makeLibp2pIdentity(t).GetPublic()).Variant())
}

func TestID_String(t *testing.T) {

}

func TestID_StringOfBase(t *testing.T) {

}

func TestID_DID(t *testing.T) {
	fmt.Println(NewRandomIDV1().DID())
	fmt.Println(NewPubKeyIDV1(makeLibp2pIdentity(t).GetPublic()).DID())
	assert.NotEmpty(t, NewRandomIDV1().DID())
	assert.NotEmpty(t, NewPubKeyIDV1(makeLibp2pIdentity(t).GetPublic()).DID())
}

func TestID_Encode(t *testing.T) {

}

func TestID_Bytes(t *testing.T) {

}

func TestID_MarshalBinary(t *testing.T) {

}

func TestID_MarshalText(t *testing.T) {

}

func TestID_Equals(t *testing.T) {

}

func TestID_KeyString(t *testing.T) {

}

func TestID_Loggable(t *testing.T) {

}

func randBytes(t *testing.T, size uint8) []byte {
	bytes := make([]byte, size)
	_, err := rand.Read(bytes)
	require.NoError(t, err)
	return bytes
}
