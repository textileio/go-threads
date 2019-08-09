package roles_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/ipfs/go-cid"
	dstest "github.com/ipfs/go-merkledag/test"
	. "github.com/textileio/go-textile-thread/roles"
)

var dag = dstest.Mock()

var vars = struct {
	json    string
	jsonCid string
}{
	json: `
{
    "default": 1,
    "accounts": {
        "P7isTTVuZXSwyak3eFeNSX9MUueJFT8iX38dpFi28U1ME8Zd": 3,
        "P8xM7BwAXCyLpMH8Mnj78MBi1Eg64Zu36A7DJhoC97aws8RV": 0
    }
}
`,
	jsonCid: "bafyreiftulgtjfjd2suc2v3f6ikssj7w6bklzg3tsoz3wp4erlarkqrloy",
}

func TestFromJSON(t *testing.T) {
	roles, err := FromJSON(bytes.NewReader([]byte(vars.json)))
	if err != nil {
		t.Fatal(err)
	}

	if roles.Cid().String() != vars.jsonCid {
		t.Fatalf("unexpected cid: %s", roles.Cid().String())
	}

	err = dag.Add(context.Background(), roles)
	if err != nil {
		t.Fatal(err)
	}
}

func TestFromCid(t *testing.T) {
	c, err := cid.Decode(vars.jsonCid)
	if err != nil {
		t.Fatal(err)
	}

	roles, err := FromCid(context.Background(), c, dag)
	if err != nil {
		t.Fatal(err)
	}

	if roles.Cid().String() != vars.jsonCid {
		t.Fatalf("unexpected cid: %s", roles.Cid().String())
	}

	subnodes := roles.Members()
	if len(subnodes) != 2 {
		t.Fatalf("unexpected node count: %d", len(subnodes))
	}
}
