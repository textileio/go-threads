package cbor

import (
	cbornode "github.com/ipfs/go-ipld-cbor"
	"github.com/ipfs/go-ipld-format"
	mh "github.com/multiformats/go-multihash"
	"github.com/textileio/go-textile-core/thread"
)

func init() {
	cbornode.RegisterCborType(invite{})
}

type invite struct {
	Readable bool
	Logs     []thread.LogInfo
}

func NewInvite(logs []thread.LogInfo, readable bool) (format.Node, error) {
	for _, log := range logs {
		log.PrivKey = nil
		if !readable {
			log.ReadKey = nil
		}
	}
	return cbornode.WrapObject(&invite{
		Readable: readable,
		Logs:     logs,
	}, mh.SHA2_256, -1)
}

func DecodeInvite(node format.Node) ([]thread.LogInfo, bool, error) {
	i := new(invite)
	err := cbornode.DecodeInto(node.RawData(), i)
	if err != nil {
		return nil, false, err
	}
	return i.Logs, i.Readable, nil
}
