package cbor

import (
	"context"
	"fmt"

	"github.com/ipfs/go-cid"
	cbornode "github.com/ipfs/go-ipld-cbor"
	format "github.com/ipfs/go-ipld-format"
	ic "github.com/libp2p/go-libp2p-core/crypto"
	mh "github.com/multiformats/go-multihash"
	"github.com/textileio/go-threads/core/net"
	"github.com/textileio/go-threads/crypto"
	pb "github.com/textileio/go-threads/net/pb"
)

func init() {
	cbornode.RegisterCborType(record{})
}

// record defines the node structure of a record.
type record struct {
	Block     cid.Cid
	Sig       []byte
	AuthorSig []byte
	Prev      cid.Cid `refmt:",omitempty"`
}

type CreateRecordConfig struct {
	Block      format.Node
	Prev       cid.Cid
	Key        ic.PrivKey
	AuthorKey  ic.PrivKey
	ServiceKey crypto.EncryptionKey
}

// CreateRecord returns a new record from the given block and log private key.
func CreateRecord(ctx context.Context, dag format.DAGService, config CreateRecordConfig) (net.Record, error) {
	payload := config.Block.Cid().Bytes()
	if config.Prev.Defined() {
		payload = append(payload, config.Prev.Bytes()...)
	}
	sig, err := config.Key.Sign(payload)
	if err != nil {
		return nil, err
	}
	asig, err := config.AuthorKey.Sign(payload)
	if err != nil {
		return nil, err
	}
	obj := &record{
		Block:     config.Block.Cid(),
		Sig:       sig,
		AuthorSig: asig,
		Prev:      config.Prev,
	}
	node, err := cbornode.WrapObject(obj, mh.SHA2_256, -1)
	if err != nil {
		return nil, err
	}
	coded, err := EncodeBlock(node, config.ServiceKey)
	if err != nil {
		return nil, err
	}

	if dag != nil {
		if err = dag.Add(ctx, coded); err != nil {
			return nil, err
		}
	}

	return &Record{
		Node:  coded,
		obj:   obj,
		block: config.Block,
	}, nil
}

// GetRecord returns a record from the given cid.
func GetRecord(ctx context.Context, dag format.DAGService, id cid.Cid, key crypto.DecryptionKey) (net.Record, error) {
	coded, err := dag.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return RecordFromNode(coded, key)
}

// RecordFromNode decodes a record from a node using the given key.
func RecordFromNode(coded format.Node, key crypto.DecryptionKey) (net.Record, error) {
	obj := new(record)
	node, err := DecodeBlock(coded, key)
	if err != nil {
		return nil, err
	}
	if err = cbornode.DecodeInto(node.RawData(), obj); err != nil {
		return nil, err
	}
	return &Record{
		Node: coded,
		obj:  obj,
	}, nil
}

// RemoveRecord removes a record from the dag service.
func RemoveRecord(ctx context.Context, dag format.DAGService, rec net.Record) error {
	return dag.Remove(ctx, rec.Cid())
}

// RecordToProto returns a proto version of a record for transport.
// Nodes are sent encrypted.
func RecordToProto(ctx context.Context, dag format.DAGService, rec net.Record) (*pb.Log_Record, error) {
	block, err := rec.GetBlock(ctx, dag)
	if err != nil {
		return nil, err
	}
	event, ok := block.(*Event)
	if !ok {
		event, err = EventFromNode(block)
		if err != nil {
			return nil, err
		}
	}
	header, err := event.GetHeader(ctx, dag, nil)
	if err != nil {
		return nil, err
	}
	body, err := event.GetBody(ctx, dag, nil)
	if err != nil {
		return nil, err
	}

	return &pb.Log_Record{
		RecordNode: rec.RawData(),
		EventNode:  block.RawData(),
		HeaderNode: header.RawData(),
		BodyNode:   body.RawData(),
	}, nil
}

// Unmarshal returns a node from a serialized version that contains link data.
func RecordFromProto(rec *pb.Log_Record, key crypto.DecryptionKey) (net.Record, error) {
	if key == nil {
		return nil, fmt.Errorf("decryption key is required")
	}

	rnode, err := cbornode.Decode(rec.RecordNode, mh.SHA2_256, -1)
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

	decoded, err := DecodeBlock(rnode, key)
	if err != nil {
		return nil, err
	}
	robj := new(record)
	if err = cbornode.DecodeInto(decoded.RawData(), robj); err != nil {
		return nil, err
	}

	eobj := new(event)
	if err = cbornode.DecodeInto(enode.RawData(), eobj); err != nil {
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
		Node:  rnode,
		obj:   robj,
		block: event,
	}, nil
}

// Record is an IPLD node representing a record.
type Record struct {
	format.Node

	obj   *record
	block format.Node
}

// BlockID returns the cid of the block.
func (r *Record) BlockID() cid.Cid {
	return r.obj.Block
}

// GetBlock returns the block.
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

// PrevID returns the cid of the previous linked record.
func (r *Record) PrevID() cid.Cid {
	return r.obj.Prev
}

// Sig returns the node's log key signature.
func (r *Record) Sig() []byte {
	return r.obj.Sig
}

// AuthorSig returns the node's author key signature.
func (r *Record) AuthorSig() []byte {
	return r.obj.AuthorSig
}

// Verify returns a nil error if the node signature is valid.
func (r *Record) Verify(key ic.PubKey, sig []byte) error {
	if r.block == nil {
		return fmt.Errorf("block not loaded")
	}
	payload := r.block.Cid().Bytes()
	if r.PrevID().Defined() {
		payload = append(payload, r.PrevID().Bytes()...)
	}
	ok, err := key.Verify(payload, sig)
	if !ok || err != nil {
		return fmt.Errorf("bad signature")
	}
	return nil
}
