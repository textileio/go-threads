package cbor

import (
	"fmt"

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

// invite defines the node structure of an invite.
type invite struct {
	Logs []loginfo
}

// loginfo defines the node structure of loginfo.
type loginfo struct {
	ID        string
	PubKey    []byte
	FollowKey []byte
	ReadKey   []byte
	Addrs     [][]byte
	Heads     [][]byte
}

// NewInvite creates a new invite with the given logs.
// The read keys will be included if readable is true.
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
	return cbornode.WrapObject(&invite{Logs: ls}, mh.SHA2_256, -1)
}

// InviteFromNode returns invite info from a node.
func InviteFromNode(node format.Node) (*Invite, error) {
	n := new(invite)
	err := cbornode.DecodeInto(node.RawData(), n)
	if err != nil {
		return nil, err
	}

	invite := &Invite{
		Logs: make([]thread.LogInfo, len(n.Logs)),
	}
	for i, l := range n.Logs {
		id, err := peer.IDB58Decode(l.ID)
		if err != nil {
			return nil, err
		}
		pk, err := ic.UnmarshalPublicKey(l.PubKey)
		if err != nil {
			return nil, err
		}
		if !id.MatchesPublicKey(pk) {
			return nil, fmt.Errorf("log id does not match public key")
		}
		addrs := make([]ma.Multiaddr, len(l.Addrs))
		for j, a := range l.Addrs {
			addrs[j], err = ma.NewMultiaddrBytes(a)
			if err != nil {
				return nil, err
			}
		}
		heads := make([]cid.Cid, len(l.Heads))
		for k, h := range l.Heads {
			heads[k], err = cid.Cast(h)
			if err != nil {
				return nil, err
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
		invite.Logs[i] = log
	}

	return invite, nil
}

// Invite contains logs needed to load a thread.
type Invite struct {
	Logs []thread.LogInfo
}

// Readable returns whether or not all logs contain a read key.
func (i *Invite) Readable() bool {
	for _, l := range i.Logs {
		if l.ReadKey == nil {
			return false
		}
	}
	return true
}
