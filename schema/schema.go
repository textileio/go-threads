package schema

import (
	"context"
	"fmt"
	"io"

	"github.com/ipfs/go-ipld-cbor/encoding"

	"github.com/ipfs/go-cid"
	cbornode "github.com/ipfs/go-ipld-cbor"
	"github.com/ipfs/go-ipld-format"
	mh "github.com/multiformats/go-multihash"
	"github.com/polydawn/refmt/obj/atlas"
	"github.com/textileio/go-textile-core/threads"
)

type Schema struct {
	cbornode.Node
	props properties
}

type properties struct {
	Name       string
	Use        string
	Pin        bool
	Plaintext  bool
	Mill       string
	Opts       map[string]string
	Nodes      map[string]properties
	JSONSchema map[string]interface{}
}

var propertiesAtlas = atlas.MustBuild(
	atlas.BuildEntry(properties{}).StructMap().Autogenerate().Complete(),
)

var marshaller = encoding.NewPooledMarshaller(propertiesAtlas)
var unmarshaller = encoding.NewPooledUnmarshaller(propertiesAtlas)

func FromJSON(reader io.Reader) (threads.Schema, error) {
	node, err := cbornode.FromJSON(reader, mh.SHA2_256, -1)
	if err != nil {
		return nil, err
	}

	p := properties{}
	err = unmarshaller.Unmarshal(node.RawData(), &p)
	if err != nil {
		return nil, err
	}

	return &Schema{
		Node:  *node,
		props: p,
	}, nil
}

func FromCid(ctx context.Context, c cid.Cid, service format.DAGService) (threads.Schema, error) {
	node, err := service.Get(ctx, c)
	if err != nil {
		return nil, err
	}
	if node.Cid().Type() != cid.DagCBOR {
		return nil, fmt.Errorf("invalid node type")
	}

	var p properties
	err = unmarshaller.Unmarshal(node.RawData(), &p)
	if err != nil {
		return nil, err
	}

	n, ok := node.(*cbornode.Node)
	if !ok {
		return nil, fmt.Errorf("invalid node")
	}

	return &Schema{
		Node:  *n,
		props: p,
	}, nil
}

func (s *Schema) Name() string {
	return s.props.Name
}

func (s *Schema) Pin() bool {
	return s.props.Pin
}

func (s *Schema) Plaintext() bool {
	return s.props.Plaintext
}

func (s *Schema) Mill() string {
	return s.props.Mill
}

func (s *Schema) Opts() map[string]string {
	return s.props.Opts
}

func (s *Schema) Nodes() map[string]threads.Schema {
	m := make(map[string]threads.Schema)

	for k, v := range s.props.Nodes {
		b, err := marshaller.Marshal(v)
		if err == nil {
			n, err := cbornode.Decode(b, mh.SHA2_256, -1)
			if err == nil {
				m[k] = &Schema{
					Node:  *n,
					props: v,
				}
			}
		}
	}

	return m
}

func (s *Schema) JSONSchema() map[string]interface{} {
	return s.props.JSONSchema
}
