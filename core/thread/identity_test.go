package thread

import (
	"crypto/rand"
	"testing"

	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/stretchr/testify/require"
)

func makeLibp2pIdentity(t *testing.T) Identity {
	sk, _, err := crypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	return NewLibp2pIdentity(sk)
}
