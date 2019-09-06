package cbor

import (
	"time"

	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/ipfs/go-ipld-format"
	mh "github.com/multiformats/go-multihash"
	"github.com/textileio/go-textile-core/crypto"
	"github.com/textileio/go-textile-core/thread"
)

func init() {
	cbor.RegisterCborType(event{})
	cbor.RegisterCborType(eventHeader{})
}

type event struct {
	Body   cid.Cid
	Header cid.Cid
}

type eventHeader struct {
	Time int64
	Key  []byte `refmt:",omitempty"`
}

func NewEvent(body format.Node) (thread.Event, error) {
	key, err := crypto.GenerateAESKey()
	if err != nil {
		return nil, err
	}

	cipherbody, err := EncodeBlock(body, key)
	if err != nil {
		return nil, err
	}
	header := &eventHeader{
		Time: time.Now().UnixNano(),
		Key:  key,
	}

	hnode, err := cbor.WrapObject(header, mh.SHA2_256, -1)
	if err != nil {
		return nil, err
	}
	enode, err := cbor.WrapObject(&event{
		Body:   cipherbody.Cid(),
		Header: hnode.Cid(),
	}, mh.SHA2_256, -1)
	if err != nil {
		return nil, err
	}

	return &Event{
		Node: *enode,
		header: &EventHeader{
			Node: *hnode,
			time: int(header.Time),
			key:  header.Key,
		},
		body: cipherbody,
	}, nil
}

type Event struct {
	cbor.Node

	header *EventHeader
	body   format.Node
}

func (e *Event) Header() thread.EventHeader {
	return e.header
}

func (e *Event) Body() format.Node {
	return e.body
}

type EventHeader struct {
	cbor.Node

	time int
	key  []byte
}

func (e *EventHeader) Time() int {
	return e.time
}

func (e *EventHeader) Key() []byte {
	return e.key
}
