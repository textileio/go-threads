package cbor

import (
	cbornode "github.com/ipfs/go-ipld-cbor"
	"github.com/ipfs/go-ipld-format"
	mh "github.com/multiformats/go-multihash"
	"github.com/textileio/go-textile-core/thread"
)

func init() {
	cbornode.RegisterCborType(Invite{})
}

type Invite struct {
	Logs map[string]thread.LogInfo
}

func NewInvite(invite Invite) (format.Node, error) {
	return cbornode.WrapObject(&invite, mh.SHA2_256, -1)
}
