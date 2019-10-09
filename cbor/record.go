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
	pb "github.com/textileio/go-textile-threads/pb"
)

func init() {
	cbornode.RegisterCborType(record{})
}

type record struct {
	Block cid.Cid
	Sig   []byte
	Prev  cid.Cid `refmt:",omitempty"`
}

func NewRecord(ctx context.Context, dag format.DAGService, block format.Node, prev cid.Cid, sk ic.PrivKey, key crypto.EncryptionKey) (thread.Record, error) {
	payload := block.RawData()
	if prev.Defined() {
		payload = append(payload, prev.Bytes()...)
	}
	sig, err := sk.Sign(payload)
	if err != nil {
		return nil, err
	}
	obj := &record{
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

	return &Record{
		Node:  coded,
		obj:   obj,
		block: block,
	}, nil
}

func GetRecord(ctx context.Context, dag format.DAGService, id cid.Cid, key crypto.DecryptionKey) (thread.Record, error) {
	coded, err := dag.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return RecordFromNode(coded, key)
}

func RecordFromNode(coded format.Node, key crypto.DecryptionKey) (thread.Record, error) {
	obj := new(record)
	node, err := DecodeBlock(coded, key)
	if err != nil {
		return nil, err
	}
	err = cbornode.DecodeInto(node.RawData(), obj)
	if err != nil {
		return nil, err
	}
	return &Record{
		Node: coded,
		obj:  obj,
	}, nil
}

func RecordToProto(ctx context.Context, dag format.DAGService, rec thread.Record) (*pb.Record, error) {
	block, err := rec.GetBlock(ctx, dag)
	if err != nil {
		return nil, err
	}
	event, err := EventFromNode(block)
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

	return &pb.Record{
		Node:       rec.RawData(),
		EventNode:  block.RawData(),
		HeaderNode: header.RawData(),
		BodyNode:   body.RawData(),
	}, nil
}

// Unmarshal returns a node from a serialized version that contains link data.
func RecordFromProto(rec *pb.Record, key crypto.DecryptionKey) (thread.Record, error) {
	node, err := cbornode.Decode(rec.Node, mh.SHA2_256, -1)
	if err != nil {
		return nil, err
	}
	enode, err := cbornode.Decode(rec.EventNode, mh.SHA2_256, -1)
	if err != nil {
		return nil, err
	}
	hnode, err := cbornode.Decode(rec.HeaderNode, mh.SHA2_256, -1)
	if err != nil {
		return nil, err
	}
	body, err := cbornode.Decode(rec.BodyNode, mh.SHA2_256, -1)
	if err != nil {
		return nil, err
	}

	decoded, err := DecodeBlock(node, key)
	if err != nil {
		return nil, err
	}
	robj := new(record)
	err = cbornode.DecodeInto(decoded.RawData(), robj)
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
	return &Record{
		Node:  node,
		obj:   robj,
		block: event,
	}, nil
}

type Record struct {
	format.Node

	obj   *record
	block format.Node
}

func (r *Record) BlockID() cid.Cid {
	return r.obj.Block
}

func (r *Record) GetBlock(ctx context.Context, dag format.DAGService) (format.Node, error) {
	if r.block != nil {
		return r.block, nil
	}

	var err error
	r.block, err = dag.Get(ctx, r.obj.Block)
	if err != nil {
		return nil, err
	}
	return r.block, nil
}

func (r *Record) PrevID() cid.Cid {
	return r.obj.Prev
}

func (r *Record) Sig() []byte {
	return r.obj.Sig
}

func (r *Record) Verify(pk ic.PubKey) error {
	if r.block == nil {
		return fmt.Errorf("block not loaded")
	}
	payload := r.block.RawData()
	if r.PrevID().Defined() {
		payload = append(payload, r.PrevID().Bytes()...)
	}
	ok, err := pk.Verify(payload, r.Sig())
	if !ok || err != nil {
		return fmt.Errorf("bad signature")
	}
	return nil
}
