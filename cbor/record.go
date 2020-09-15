package cbor

import (
	"context"
	"fmt"

	"github.com/textileio/go-threads/core/thread"

	"github.com/ipfs/go-cid"
	cbornode "github.com/ipfs/go-ipld-cbor"
	"github.com/ipfs/go-ipld-format"
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
	Block  cid.Cid
	Sig    []byte
	PubKey []byte
	Prev   cid.Cid `refmt:",omitempty"`
}

// CreateRecordConfig wraps all the elements needed for creating a new record.
type CreateRecordConfig struct {
	Block      format.Node
	Prev       cid.Cid
	Key        ic.PrivKey
	PubKey     thread.PubKey
	ServiceKey crypto.EncryptionKey
}

// CreateRecord returns a new record from the given block and log private key.
func CreateRecord(ctx context.Context, dag format.DAGService, config CreateRecordConfig) (net.Record, error) {
	pkb, err := config.PubKey.MarshalBinary()
	if err != nil {
		return nil, err
	}
	var payload []byte
	if config.Prev.Defined() {
		payload = append(config.Block.Cid().Bytes(), config.Prev.Bytes()...)
	} else {
		payload = pkb
	}
	sig, err := config.Key.Sign(payload)
	if err != nil {
		return nil, err
	}
	obj := &record{
		Block:  config.Block.Cid(),
		Sig:    sig,
		PubKey: pkb,
		Prev:   config.Prev,
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

func (r *Record) PubKey() []byte {
	return r.obj.PubKey
}

func (r *Record) Verify(key ic.PubKey) error {
	if r.block == nil {
		return fmt.Errorf("block not loaded")
	}
	var payload []byte
	if r.PrevID().Defined() {
		payload = append(r.block.Cid().Bytes(), r.PrevID().Bytes()...)
	} else {
		payload = r.PubKey()
	}
	ok, err := key.Verify(payload, r.Sig())
	if !ok || err != nil {
		return fmt.Errorf("bad signature")
	}
	return nil
}
