package roles

import (
	"context"
	"fmt"
	"io"

	"github.com/textileio/go-textile-wallet/account"

	"github.com/ipfs/go-cid"
	cbornode "github.com/ipfs/go-ipld-cbor"
	"github.com/ipfs/go-ipld-cbor/encoding"
	"github.com/ipfs/go-ipld-format"
	mh "github.com/multiformats/go-multihash"
	"github.com/polydawn/refmt/obj/atlas"
	"github.com/textileio/go-textile-core/threads"
)

type Roles struct {
	cbornode.Node
	props properties
}

type properties struct {
	Default threads.Role
	Members map[string]threads.Role
}

var propertiesAtlas = atlas.MustBuild(
	atlas.BuildEntry(properties{}).StructMap().Autogenerate().Complete(),
)

var unmarshaller = encoding.NewPooledUnmarshaller(propertiesAtlas)

func FromJSON(reader io.Reader) (threads.Roles, error) {
	node, err := cbornode.FromJSON(reader, mh.SHA2_256, -1)
	if err != nil {
		return nil, err
	}

	p := properties{}
	err = unmarshaller.Unmarshal(node.RawData(), &p)
	if err != nil {
		return nil, err
	}

	return &Roles{
		Node:  *node,
		props: p,
	}, nil
}

func FromCid(ctx context.Context, c cid.Cid, service format.DAGService) (threads.Roles, error) {
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

	return &Roles{
		Node:  *n,
		props: p,
	}, nil
}

func (s *Roles) Default() threads.Role {
	return s.props.Default
}

func (s *Roles) Members() map[account.Account]threads.Role {
	m := make(map[account.Account]threads.Role)

	for k, v := range s.props.Members {
		a, err := account.Parse(k)
		if err == nil {
			m[a] = v
		}
	}

	return m
}
