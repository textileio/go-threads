package cbor

import (
	"github.com/ipfs/go-cid"
	cbornode "github.com/ipfs/go-ipld-cbor"
	"github.com/ipfs/go-ipld-format"
	ic "github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"
	mh "github.com/multiformats/go-multihash"
	"github.com/textileio/go-textile-core/thread"
)

func init() {
	cbornode.RegisterCborType(invite{})
	cbornode.RegisterCborType(loginfo{})
}

type invite struct {
	Readable bool
	Logs     []loginfo
}

type loginfo struct {
	ID        string
	PubKey    []byte
	FollowKey []byte
	ReadKey   []byte
	Addrs     [][]byte
	Heads     [][]byte
}

func NewInvite(logs []thread.LogInfo, readable bool) (format.Node, error) {
	ls := make([]loginfo, len(logs))
	for i, l := range logs {
		pk, err := ic.MarshalPublicKey(l.PubKey)
		if err != nil {
			return nil, err
		}
		addrs := make([][]byte, len(l.Addrs))
		for j, a := range l.Addrs {
			addrs[j] = a.Bytes()
		}
		heads := make([][]byte, len(l.Heads))
		for k, h := range l.Heads {
			heads[k] = h.Bytes()
		}
		log := loginfo{
			ID:        l.ID.String(),
			PubKey:    pk,
			FollowKey: l.FollowKey,
			Addrs:     addrs,
			Heads:     heads,
		}
		if readable {
			log.ReadKey = l.ReadKey
		}
		ls[i] = log
	}
	return cbornode.WrapObject(&invite{
		Readable: readable,
		Logs:     ls,
	}, mh.SHA2_256, -1)
}

func InviteFromNode(node format.Node) ([]thread.LogInfo, bool, error) {
	i := new(invite)
	err := cbornode.DecodeInto(node.RawData(), i)
	if err != nil {
		return nil, false, err
	}

	logs := make([]thread.LogInfo, len(i.Logs))
	for i, l := range i.Logs {
		id, err := peer.IDB58Decode(l.ID)
		if err != nil {
			return nil, false, err
		}
		pk, err := ic.UnmarshalPublicKey(l.PubKey)
		if err != nil {
			return nil, false, err
		}
		addrs := make([]ma.Multiaddr, len(l.Addrs))
		for j, a := range l.Addrs {
			addrs[j], err = ma.NewMultiaddrBytes(a)
			if err != nil {
				return nil, false, err
			}
		}
		heads := make([]cid.Cid, len(l.Heads))
		for k, h := range l.Heads {
			heads[k], err = cid.Cast(h)
			if err != nil {
				return nil, false, err
			}
		}
		log := thread.LogInfo{
			ID:        id,
			PubKey:    pk,
			FollowKey: l.FollowKey,
			ReadKey:   l.ReadKey,
			Addrs:     addrs,
			Heads:     heads,
		}
		logs[i] = log
	}

	return logs, i.Readable, nil
}
