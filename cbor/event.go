package cbor

import (
	"context"
	"fmt"

	"github.com/ipfs/go-cid"
	cbornode "github.com/ipfs/go-ipld-cbor"
	"github.com/ipfs/go-ipld-format"
	mh "github.com/multiformats/go-multihash"
	"github.com/textileio/go-threads/core/net"
	"github.com/textileio/go-threads/crypto"
	sym "github.com/textileio/go-threads/crypto/symmetric"
)

func init() {
	cbornode.RegisterCborType(event{})
	cbornode.RegisterCborType(eventHeader{})
}

// event defines the node structure of an event.
type event struct {
	Body   cid.Cid
	Header cid.Cid
}

// eventHeader defines the node structure of an event header.
type eventHeader struct {
	Key []byte `refmt:",omitempty"`
}

// CreateEvent create a new event by wrapping the body node.
func CreateEvent(ctx context.Context, dag format.DAGService, body format.Node, rkey crypto.EncryptionKey) (net.Event, error) {
	key, err := sym.NewRandom()
	if err != nil {
		return nil, err
	}
	codedBody, err := EncodeBlock(body, key)
	if err != nil {
		return nil, err
	}
	keyb, err := key.MarshalBinary()
	if err != nil {
		return nil, err
	}
	eventHeader := &eventHeader{
		Key: keyb,
	}
	header, err := cbornode.WrapObject(eventHeader, mh.SHA2_256, -1)
	if err != nil {
		return nil, err
	}
	codedHeader, err := EncodeBlock(header, rkey)
	if err != nil {
		return nil, err
	}
	obj := &event{
		Body:   codedBody.Cid(),
		Header: codedHeader.Cid(),
	}
	node, err := cbornode.WrapObject(obj, mh.SHA2_256, -1)
	if err != nil {
		return nil, err
	}

	if dag != nil {
		if err = dag.AddMany(ctx, []format.Node{node, codedHeader, codedBody}); err != nil {
			return nil, err
		}
	}

	return &Event{
		Node: node,
		obj:  obj,
		header: &EventHeader{
			Node: codedHeader,
			obj:  eventHeader,
		},
		body: codedBody,
	}, nil
}

// GetEvent returns the event node for the given cid.
func GetEvent(ctx context.Context, dag format.DAGService, id cid.Cid) (net.Event, error) {
	node, err := dag.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return EventFromNode(node)
}

// EventFromNode decodes the given node into an event.
func EventFromNode(node format.Node) (*Event, error) {
	obj := new(event)
	if err := cbornode.DecodeInto(node.RawData(), obj); err != nil {
		return nil, err
	}
	return &Event{
		Node: node,
		obj:  obj,
	}, nil
}

// EventFromRecord returns the event within the given node.
func EventFromRecord(ctx context.Context, dag format.DAGService, rec net.Record) (*Event, error) {
	block, err := rec.GetBlock(ctx, dag)
	if err != nil {
		return nil, err
	}

	event, ok := block.(*Event)
	if !ok {
		return EventFromNode(block)
	}
	return event, nil
}

// RemoveEvent removes an event from the dag service.
func RemoveEvent(ctx context.Context, dag format.DAGService, e *Event) error {
	return dag.RemoveMany(ctx, []cid.Cid{e.Cid(), e.HeaderID(), e.BodyID()})
}

// Event is a IPLD node representing an event.
type Event struct {
	format.Node

	obj    *event
	header *EventHeader
	body   format.Node
}

func (e *Event) HeaderID() cid.Cid {
	return e.obj.Header
}

func (e *Event) GetHeader(ctx context.Context, dag format.DAGService, key crypto.DecryptionKey) (net.EventHeader, error) {
	if e.header == nil {
		coded, err := dag.Get(ctx, e.obj.Header)
		if err != nil {
			return nil, err
		}
		e.header = &EventHeader{
			Node: coded,
		}
	}

	if e.header.obj != nil {
		return e.header, nil
	}

	header := new(eventHeader)
	if key != nil {
		node, err := DecodeBlock(e.header, key)
		if err != nil {
			return nil, err
		}
		if err = cbornode.DecodeInto(node.RawData(), header); err != nil {
			return nil, err
		}
		e.header.obj = header
	}
	return e.header, nil
}

func (e *Event) BodyID() cid.Cid {
	return e.obj.Body
}

func (e *Event) GetBody(ctx context.Context, dag format.DAGService, key crypto.DecryptionKey) (format.Node, error) {
	var k crypto.DecryptionKey
	if key != nil {
		header, err := e.GetHeader(ctx, dag, key)
		if err != nil {
			return nil, err
		}
		k, err = header.Key()
		if err != nil {
			return nil, err
		}
	}

	var err error
	if e.body == nil {
		e.body, err = dag.Get(ctx, e.obj.Body)
		if err != nil {
			return nil, err
		}
	}

	if k == nil {
		return e.body, nil
	} else {
		return DecodeBlock(e.body, k)
	}
}

// EventHeader is an IPLD node representing an event header.
type EventHeader struct {
	format.Node

	obj *eventHeader
}

func (h *EventHeader) Key() (crypto.DecryptionKey, error) {
	if h.obj == nil {
		return nil, fmt.Errorf("obj not loaded")
	}
	return crypto.DecryptionKeyFromBytes(h.obj.Key)
}
