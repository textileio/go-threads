package cbor

import (
	"context"

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

type event struct {
	Body   cid.Cid
	Header cid.Cid
}

type eventHeader struct {
	Time int64
	Key  []byte `refmt:",omitempty"`
}

func NewEvent(body format.Node, settings *tserv.PutSettings) (thread.Event, error) {
	key, err := symmetric.CreateKey()
	if err != nil {
		return nil, err
	}
	coded, err := EncodeBlock(body, key)
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
	node, err := cbornode.WrapObject(&event{
		Body:   coded.Cid(),
		Header: codedHeader.Cid(),
	}, mh.SHA2_256, -1)
	if err != nil {
		return nil, err
	}
	return &Event{
		Node: node,
		header: &EventHeader{
			Node: codedHeader,
			time: int(eventHeader.Time),
			key:  eventHeader.Key,
		},
		body: coded,
	}, nil
}

func EncodeEvent(event thread.Event, key crypto.EncryptionKey) (format.Node, error) {
	return EncodeBlock(event, key)
}

func DecodeEvent(ctx context.Context, dag format.DAGService, id cid.Cid, key crypto.DecryptionKey) (thread.Event, error) {
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
	codedHeader, err := dag.Get(ctx, event.Header)
	if err != nil {
		return nil, err
	}
	header, err := DecodeBlock(codedHeader, key)
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
		Node: coded,
		header: &EventHeader{
			Node: codedHeader,
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
	key, err := crypto.ParseDecryptionKey(e.header.key)
	if err != nil {
		return nil, err
	}
	return DecodeBlock(e.body, key)
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
