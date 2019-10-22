package cbor

import (
	"fmt"

	"github.com/textileio/go-textile-core/crypto/symmetric"

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
	cbornode.RegisterCborType(log{})
	cbornode.RegisterCborType(logs{})
}

// log defines the node structure of loginfo.
type log struct {
	ID        string
	PubKey    []byte
	FollowKey []byte
	ReadKey   []byte
	Addrs     [][]byte
	Heads     [][]byte
}

// logs defines the node structure of logs.
type logs struct {
	Logs []log
}

// NewLogs creates a thread node with the given logs.
// The read keys will be included if readable is true.
func NewLogs(lgs []thread.LogInfo, readable bool) (format.Node, error) {
	ls := make([]log, len(lgs))
	for i, l := range lgs {
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
		lg := log{
			ID:        l.ID.String(),
			PubKey:    pk,
			FollowKey: l.FollowKey.Bytes(),
			Addrs:     addrs,
			Heads:     heads,
		}
		if readable {
			lg.ReadKey = l.ReadKey.Bytes()
		}
		ls[i] = lg
	}
	return cbornode.WrapObject(&logs{Logs: ls}, mh.SHA2_256, -1)
}

// LogsFromNode returns slice info from a node.
func LogsFromNode(node format.Node) (*Logs, error) {
	n := new(logs)
	err := cbornode.DecodeInto(node.RawData(), n)
	if err != nil {
		return nil, err
	}

	info := &Logs{
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
		fk, err := symmetric.NewKey(l.FollowKey)
		if err != nil {
			return nil, err
		}
		lg := thread.LogInfo{
			ID:        id,
			PubKey:    pk,
			FollowKey: fk,
			Addrs:     addrs,
			Heads:     heads,
		}
		if l.ReadKey != nil {
			lg.ReadKey, err = symmetric.NewKey(l.ReadKey)
			if err != nil {
				return nil, err
			}
		}
		info.Logs[i] = lg
	}

	return info, nil
}

// Logs contains logs that are part of a thread.
type Logs struct {
	Logs []thread.LogInfo
}

// Readable returns whether or not all logs contain a read key.
func (i *Logs) Readable() bool {
	for _, l := range i.Logs {
		if l.ReadKey == nil {
			return false
		}
	}
	return true
}
