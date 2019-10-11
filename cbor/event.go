package cbor

import (
	"context"
	"fmt"
	"time"

	"github.com/ipfs/go-cid"
	cbornode "github.com/ipfs/go-ipld-cbor"
	"github.com/ipfs/go-ipld-format"
	mh "github.com/multiformats/go-multihash"
	"github.com/textileio/go-textile-core/crypto"
	"github.com/textileio/go-textile-core/crypto/symmetric"
	"github.com/textileio/go-textile-core/thread"
	tserv "github.com/textileio/go-textile-core/threadservice"
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
	Time int64
	Key  []byte `refmt:",omitempty"`
}

// NewEvent create a new event by wrapping the body node.
func NewEvent(ctx context.Context, dag format.DAGService, body format.Node, settings *tserv.AddSettings) (thread.Event, error) {
	key, err := symmetric.CreateKey()
	if err != nil {
		return nil, err
	}
	codedBody, err := EncodeBlock(body, key)
	if err != nil {
		return nil, err
	}
	keyb, err := key.Marshal()
	if err != nil {
		return nil, err
	}
	eventHeader := &eventHeader{
		Time: settings.Time.Unix(),
		Key:  keyb,
	}
	header, err := cbornode.WrapObject(eventHeader, mh.SHA2_256, -1)
	if err != nil {
		return nil, err
	}
	codedHeader, err := EncodeBlock(header, settings.Key)
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

	err = dag.AddMany(ctx, []format.Node{node, codedHeader, codedBody})
	if err != nil {
		return nil, err
	}

	return &Event{
		Node: node,
		obj:  obj,
		header: &EventHeader{
			Node: codedHeader,
			obj:  eventHeader,
		},
	}, nil
}

// GetEvent returns the event node for the given cid.
func GetEvent(ctx context.Context, dag format.DAGService, id cid.Cid) (thread.Event, error) {
	node, err := dag.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return EventFromNode(node)
}

// EventFromNode decodes the given node into an event.
func EventFromNode(node format.Node) (thread.Event, error) {
	obj := new(event)
	err := cbornode.DecodeInto(node.RawData(), obj)
	if err != nil {
		return nil, err
	}
	return &Event{
		Node: node,
		obj:  obj,
	}, nil
}

// EventFromRecord returns the event within the given node.
func EventFromRecord(ctx context.Context, dag format.DAGService, rec thread.Record) (thread.Event, error) {
	block, err := rec.GetBlock(ctx, dag)
	if err != nil {
		return nil, err
	}

	event, ok := block.(*Event)
	if !ok {
		return nil, fmt.Errorf("invalid event")
	}
	return event, nil
}

// Event is a IPLD node representing an event.
type Event struct {
	format.Node

	obj    *event
	header *EventHeader
	body   format.Node
}

// HeaderID returns the cid of the header.
func (e *Event) HeaderID() cid.Cid {
	return e.obj.Header
}

// GetHeader returns the header node.
func (e *Event) GetHeader(ctx context.Context, dag format.DAGService, key crypto.DecryptionKey) (thread.EventHeader, error) {
	if e.header == nil {
		coded, err := dag.Get(ctx, e.obj.Header)
		if err != nil {
			return nil, err
		}
		e.header = &EventHeader{
			Node: coded,
		}
	}

	header := new(eventHeader)
	if key != nil {
		node, err := DecodeBlock(e.header, key)
		if err != nil {
			return nil, err
		}
		err = cbornode.DecodeInto(node.RawData(), header)
		if err != nil {
			return nil, err
		}
		e.header.obj = header
	}
	return e.header, nil
}

// BodyID returns the cid of the body.
func (e *Event) BodyID() cid.Cid {
	return e.obj.Body
}

// GetBody returns the body node.
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

// Time returns the wall-clock time when the event was created if it has been decoded.
func (h *EventHeader) Time() (*time.Time, error) {
	if h.obj == nil {
		return nil, fmt.Errorf("obj not loaded")
	}
	t := time.Unix(h.obj.Time, 0)
	return &t, nil
}

// Key returns the key needed to decrypt the event body if it has been decoded.
func (h *EventHeader) Key() (crypto.DecryptionKey, error) {
	if h.obj == nil {
		return nil, fmt.Errorf("obj not loaded")
	}
	return crypto.ParseDecryptionKey(h.obj.Key)
}
