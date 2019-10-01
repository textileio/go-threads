package cbor

import (
	"context"
	"fmt"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-ipld-cbor"
	"github.com/ipfs/go-ipld-format"
	ic "github.com/libp2p/go-libp2p-core/crypto"
	mh "github.com/multiformats/go-multihash"
	"github.com/textileio/go-textile-core/crypto"
	"github.com/textileio/go-textile-core/thread"
)

func init() {
	cbornode.RegisterCborType(node{})
}

func NewNode(ctx context.Context, dag format.DAGService, block format.Node, prev cid.Cid, sk ic.PrivKey, fk crypto.EncryptionKey) (thread.Node, error) {
	payload := block.RawData()
	if prev.Defined() {
		payload = append(payload, prev.Bytes()...)
	}
	sig, err := sk.Sign(payload)
	if err != nil {
		return nil, err
	}
	n := &node{
		Block: block.Cid(),
		Sig:   sig,
		Prev:  prev,
	}
	node, err := cbornode.WrapObject(n, mh.SHA2_256, -1)
	if err != nil {
		return nil, err
	}
	coded, err := EncodeBlock(node, fk)
	if err != nil {
		return nil, err
	}

	err = dag.AddMany(ctx, []format.Node{coded})
	if err != nil {
		return nil, err
	}

	return &Node{
		Node:  coded,
		n:     n,
		block: block,
	}, nil
}

func DecodeNode(ctx context.Context, dag format.DAGService, coded format.Node, key crypto.DecryptionKey) (thread.Node, error) {
	decoded, err := DecodeBlock(coded, key)
	if err != nil {
		return nil, err
	}
	n := new(node)
	err = cbornode.DecodeInto(decoded.RawData(), n)
	if err != nil {
		return nil, err
	}
	block, err := dag.Get(ctx, n.Block)
	if err != nil {
		return nil, err
	}
	return &Node{
		Node:  coded,
		n:     n,
		block: block,
	}, nil
}

func GetNode(ctx context.Context, dag format.DAGService, id cid.Cid, key crypto.DecryptionKey) (thread.Node, error) {
	coded, err := dag.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return DecodeNode(ctx, dag, coded, key)
}

type node struct {
	Block cid.Cid
	Sig   []byte
	Prev  cid.Cid `refmt:",omitempty"`
}

type Node struct {
	format.Node

	n     *node
	block format.Node
}

func (n *Node) Block() format.Node {
	return n.block
}

func (n *Node) Sig() []byte {
	return n.n.Sig
}

func (n *Node) Prev() cid.Cid {
	return n.n.Prev
}

func (n *Node) Verify(pk ic.PubKey) error {
	payload := n.block.RawData()
	if n.Prev().Defined() {
		payload = append(payload, n.Prev().Bytes()...)
	}
	ok, err := pk.Verify(payload, n.Sig())
	if !ok || err != nil {
		return fmt.Errorf("bad signature")
	}
	return nil
}
