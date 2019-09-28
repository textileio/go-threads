package cbor

import (
	"context"

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

func NewNode(block format.Node, prev cid.Cid, sk ic.PrivKey) (thread.Node, error) {
	tnode := &Node{block: block}
	n := &node{
		Block: tnode.block.Cid(),
	}
	payload := tnode.block.RawData()
	if prev.Defined() {
		n.Prev = prev
		payload = append(payload, n.Prev.Bytes()...)
	}
	var err error
	n.Sig, err = sk.Sign(payload)
	if err != nil {
		return nil, err
	}
	tnode.Node, err = cbornode.WrapObject(n, mh.SHA2_256, -1)
	if err != nil {
		return nil, err
	}
	tnode.sig = n.Sig
	tnode.prev = n.Prev
	return tnode, nil
}

func EncodeNode(node format.Node, key crypto.EncryptionKey) (format.Node, error) {
	return EncodeBlock(node, key)
}

func DecodeNode(ctx context.Context, dag format.DAGService, id cid.Cid, key crypto.DecryptionKey) (thread.Node, error) {
	coded, err := dag.Get(ctx, id)
	if err != nil {
		return nil, err
	}
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
		Node:  decoded,
		block: block,
		sig:   n.Sig,
		prev:  n.Prev,
	}, nil
}

type node struct {
	Block cid.Cid
	Sig   []byte
	Prev  cid.Cid `refmt:",omitempty"`
}

type Node struct {
	format.Node

	block format.Node
	sig   []byte
	prev  cid.Cid
}

func (n *Node) Block() format.Node {
	return n.block
}

func (n *Node) Sig() []byte {
	return n.sig
}

func (n *Node) Prev() cid.Cid {
	return n.prev
}
