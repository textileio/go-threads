package cbor

import (
	"context"
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
	node, err := cbornode.WrapObject(&event{
		Body:   codedBody.Cid(),
		Header: codedHeader.Cid(),
	}, mh.SHA2_256, -1)
	if err != nil {
		return nil, err
	}

	err = dag.AddMany(ctx, []format.Node{node, codedHeader, codedBody})
	if err != nil {
		return nil, err
	}

	return &Event{
		Node: node,
		header: &EventHeader{
			Node: codedHeader,
			n:    eventHeader,
			time: int(eventHeader.Time),
			key:  eventHeader.Key,
		},
	}, nil
}

func DecodeEvent(node format.Node) (thread.Event, error) {
	event := new(event)
	err := cbornode.DecodeInto(node.RawData(), event)
	if err != nil {
		return nil, err
	}
	return &Event{
		Node: node,
		n:    event,
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

	n      *event
	header *EventHeader
}

func (e *Event) Header(ctx context.Context, dag format.DAGService, key crypto.DecryptionKey) (thread.EventHeader, error) {
	if e.header != nil {
		return e.header, nil
	}
	coded, err := dag.Get(ctx, e.n.Header)
	if err != nil {
		return nil, err
	}
	node, err := DecodeBlock(coded, key)
	if err != nil {
		return nil, err
	}
	header := new(eventHeader)
	err = cbornode.DecodeInto(node.RawData(), header)
	if err != nil {
		return nil, err
	}
	e.header = &EventHeader{
		Node: coded,
		time: int(header.Time),
		key:  header.Key,
	}
	return e.header, nil
}

func (e *Event) Body(ctx context.Context, dag format.DAGService, key crypto.DecryptionKey) (format.Node, error) {
	header, err := e.Header(ctx, dag, key)
	if err != nil {
		return nil, err
	}
	coded, err := dag.Get(ctx, e.n.Body)
	if err != nil {
		return nil, err
	}
	k, err := header.Key()
	if err != nil {
		return nil, err
	}
	return DecodeBlock(coded, k)
}

type EventHeader struct {
	format.Node

	n    *eventHeader
	time int
	key  []byte
}

func (h *EventHeader) Time() time.Time {
	return time.Unix(int64(h.time), 0)
}

func (h *EventHeader) Key() (crypto.DecryptionKey, error) {
	return crypto.ParseDecryptionKey(h.key)
}
