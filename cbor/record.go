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

func DecodeRecord(coded format.Node, key crypto.DecryptionKey) (thread.Record, error) {
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

func GetRecord(ctx context.Context, dag format.DAGService, id cid.Cid, key crypto.DecryptionKey) (thread.Record, error) {
	coded, err := dag.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return DecodeRecord(coded, key)
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
