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

type event struct {
	Body   cid.Cid
	Header cid.Cid
}

type eventHeader struct {
	Time int64
	Key  []byte `refmt:",omitempty"`
}

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

func DecodeEvent(node format.Node) (thread.Event, error) {
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

func GetEvent(ctx context.Context, dag format.DAGService, id cid.Cid) (thread.Event, error) {
	node, err := dag.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return DecodeEvent(node)
}

type Event struct {
	format.Node

	obj    *event
	header *EventHeader
	body   format.Node
}

func (e *Event) HeaderID() cid.Cid {
	return e.obj.Header
}

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

type EventHeader struct {
	format.Node

	obj *eventHeader
}

func (h *EventHeader) Time() (*time.Time, error) {
	if h.obj == nil {
		return nil, fmt.Errorf("obj not loaded")
	}
	t := time.Unix(h.obj.Time, 0)
	return &t, nil
}

func (h *EventHeader) Key() (crypto.DecryptionKey, error) {
	if h.obj == nil {
		return nil, fmt.Errorf("obj not loaded")
	}
	return crypto.ParseDecryptionKey(h.obj.Key)
}
