package cbor

import (
	"context"
	"time"

	"github.com/ipfs/go-cid"
	cbornode "github.com/ipfs/go-ipld-cbor"
	"github.com/ipfs/go-ipld-format"
	mh "github.com/multiformats/go-multihash"
	"github.com/textileio/go-textile-core/crypto"
	"github.com/textileio/go-textile-core/thread"
)

func init() {
	cbornode.RegisterCborType(event{})
	cbornode.RegisterCborType(eventHeader{})
}

type event struct {
	Body   cid.Cid
	Header cid.Cid
}

type eventHeader struct {
	Time int64
	Key  []byte `refmt:",omitempty"`
}

func NewEvent(body format.Node, time time.Time) (thread.Event, error) {
	key, err := crypto.GenerateAESKey()
	if err != nil {
		return nil, err
	}
	coded, err := EncodeBlock(body, key)
	if err != nil {
		return nil, err
	}
	eventHeader := &eventHeader{
		Time: time.UnixNano(),
		Key:  key,
	}
	// @todo: header needs to be encrypted with a read key
	header, err := cbornode.WrapObject(eventHeader, mh.SHA2_256, -1)
	if err != nil {
		return nil, err
	}
	node, err := cbornode.WrapObject(&event{
		Body:   coded.Cid(),
		Header: header.Cid(),
	}, mh.SHA2_256, -1)
	if err != nil {
		return nil, err
	}
	return &Event{
		Node: node,
		header: &EventHeader{
			Node: header,
			time: int(eventHeader.Time),
			key:  eventHeader.Key,
		},
		body: coded,
	}, nil
}

func EncodeEvent(event thread.Event, key []byte) (format.Node, error) {
	return EncodeBlock(event, key)
}

func DecodeEvent(ctx context.Context, dag format.DAGService, id cid.Cid, key []byte) (thread.Event, error) {
	coded, err := dag.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	node, err := DecodeBlock(coded, key)
	if err != nil {
		return nil, err
	}
	event := new(event)
	err = cbornode.DecodeInto(node.RawData(), event)
	if err != nil {
		return nil, err
	}
	header, err := dag.Get(ctx, event.Header)
	if err != nil {
		return nil, err
	}
	eventHeader := new(eventHeader)
	err = cbornode.DecodeInto(header.RawData(), eventHeader)
	if err != nil {
		return nil, err
	}
	body, err := dag.Get(ctx, event.Body)
	if err != nil {
		return nil, err
	}
	return &Event{
		Node: node,
		header: &EventHeader{
			Node: header,
			time: int(eventHeader.Time),
			key:  eventHeader.Key,
		},
		body: body,
	}, nil
}

type Event struct {
	format.Node

	header *EventHeader
	body   format.Node
}

func (e *Event) Header() thread.EventHeader {
	return e.header
}

func (e *Event) Body() format.Node {
	return e.body
}

func (e *Event) Decrypt() (format.Node, error) {
	return DecodeBlock(e.body, e.header.key)
}

type EventHeader struct {
	format.Node

	time int
	key  []byte
}

func (h *EventHeader) Time() int {
	return h.time
}

func (h *EventHeader) Key() []byte {
	return h.key
}
