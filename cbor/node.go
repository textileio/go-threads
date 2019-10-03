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

type node struct {
	Block cid.Cid
	Sig   []byte
	Prev  cid.Cid `refmt:",omitempty"`
}

func NewNode(ctx context.Context, dag format.DAGService, block format.Node, prev cid.Cid, sk ic.PrivKey, key crypto.EncryptionKey) (thread.Node, error) {
	payload := block.RawData()
	if prev.Defined() {
		payload = append(payload, prev.Bytes()...)
	}
	sig, err := sk.Sign(payload)
	if err != nil {
		return nil, err
	}
	obj := &node{
		Block: block.Cid(),
		Sig:   sig,
		Prev:  prev,
	}
	node, err := cbornode.WrapObject(obj, mh.SHA2_256, -1)
	if err != nil {
		return nil, err
	}
	coded, err := EncodeBlock(node, key)
	if err != nil {
		return nil, err
	}

	err = dag.AddMany(ctx, []format.Node{coded})
	if err != nil {
		return nil, err
	}

	return &Node{
		Node:  coded,
		obj:   obj,
		block: block,
	}, nil
}

func DecodeNode(coded format.Node, key crypto.DecryptionKey) (thread.Node, error) {
	obj := new(node)
	node, err := DecodeBlock(coded, key)
	if err != nil {
		return nil, err
	}
	err = cbornode.DecodeInto(node.RawData(), obj)
	if err != nil {
		return nil, err
	}
	return &Node{
		Node: coded,
		obj:  obj,
	}, nil
}

func GetNode(ctx context.Context, dag format.DAGService, id cid.Cid, key crypto.DecryptionKey) (thread.Node, error) {
	coded, err := dag.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return DecodeNode(coded, key)
}

type Node struct {
	format.Node

	obj   *node
	block format.Node
}

func (n *Node) BlockID() cid.Cid {
	return n.obj.Block
}

func (n *Node) GetBlock(ctx context.Context, dag format.DAGService) (format.Node, error) {
	if n.block != nil {
		return n.block, nil
	}

	var err error
	n.block, err = dag.Get(ctx, n.obj.Block)
	if err != nil {
		return nil, err
	}
	return n.block, nil
}

func (n *Node) PrevID() cid.Cid {
	return n.obj.Prev
}

func (n *Node) Sig() []byte {
	return n.obj.Sig
}

func (n *Node) Verify(pk ic.PubKey) error {
	if n.block == nil {
		return fmt.Errorf("block not loaded")
	}
	payload := n.block.RawData()
	if n.PrevID().Defined() {
		payload = append(payload, n.PrevID().Bytes()...)
	}
	ok, err := pk.Verify(payload, n.Sig())
	if !ok || err != nil {
		return fmt.Errorf("bad signature")
	}
	return nil
}
