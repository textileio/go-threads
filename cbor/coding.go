package cbor

import (
	"context"

	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-ipld-cbor"
	format "github.com/ipfs/go-ipld-format"
	mh "github.com/multiformats/go-multihash"
	"github.com/textileio/go-textile-core/crypto"
	"github.com/textileio/go-textile-core/thread"
)

func init() {
	cbornode.RegisterCborType(env{})
}

type env struct {
	Node   []byte
	Event  []byte
	Header []byte
	Body   []byte
}

// EncodeBlock returns a node by encrypting the block's raw bytes with key.
func EncodeBlock(block blocks.Block, key crypto.EncryptionKey) (format.Node, error) {
	coded, err := key.Encrypt(block.RawData())
	if err != nil {
		return nil, err
	}
	return cbornode.WrapObject(coded, mh.SHA2_256, -1)
}

// DecodeBlock returns a node by decrypting the block's raw bytes with key.
func DecodeBlock(block blocks.Block, key crypto.DecryptionKey) (format.Node, error) {
	var raw []byte
	err := cbornode.DecodeInto(block.RawData(), &raw)
	if err != nil {
		return nil, err
	}
	decoded, err := key.Decrypt(raw)
	if err != nil {
		return nil, err
	}
	return cbornode.Decode(decoded, mh.SHA2_256, -1)
}

func Marshal(ctx context.Context, dag format.DAGService, node thread.Node) ([]byte, error) {
	block, err := node.GetBlock(ctx, dag)
	if err != nil {
		return nil, err
	}
	event, err := DecodeEvent(block)
	if err != nil {
		return nil, err
	}
	header, err := event.GetHeader(ctx, dag, nil)
	if err != nil {
		return nil, err
	}
	body, err := event.GetBody(ctx, dag, nil)
	if err != nil {
		return nil, err
	}

	env, err := cbornode.WrapObject(&env{
		Node:   node.RawData(),
		Event:  block.RawData(),
		Header: header.RawData(),
		Body:   body.RawData(),
	}, mh.SHA2_256, -1)
	if err != nil {
		return nil, err
	}
	return env.RawData(), nil
}

func Unmarshal(data []byte, key crypto.DecryptionKey) (thread.Node, error) {
	env := new(env)
	err := cbornode.DecodeInto(data, env)
	if err != nil {
		return nil, err
	}

	rnode, err := cbornode.Decode(env.Node, mh.SHA2_256, -1)
	if err != nil {
		return nil, err
	}
	enode, err := cbornode.Decode(env.Event, mh.SHA2_256, -1)
	if err != nil {
		return nil, err
	}
	hnode, err := cbornode.Decode(env.Header, mh.SHA2_256, -1)
	if err != nil {
		return nil, err
	}
	body, err := cbornode.Decode(env.Body, mh.SHA2_256, -1)
	if err != nil {
		return nil, err
	}

	decoded, err := DecodeBlock(rnode, key)
	if err != nil {
		return nil, err
	}
	nobj := new(node)
	err = cbornode.DecodeInto(decoded.RawData(), nobj)
	if err != nil {
		return nil, err
	}

	eobj := new(event)
	err = cbornode.DecodeInto(enode.RawData(), eobj)
	if err != nil {
		return nil, err
	}
	event := &Event{
		Node: enode,
		obj:  eobj,
		header: &EventHeader{
			Node: hnode,
		},
		body: body,
	}
	return &Node{
		Node:  rnode,
		obj:   nobj,
		block: event,
	}, nil
}
