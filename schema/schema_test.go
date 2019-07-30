package schema_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/ipfs/go-cid"
	dstest "github.com/ipfs/go-merkledag/test"
	. "github.com/textileio/go-textile-thread/schema"
)

var dag = dstest.Mock()

var vars = struct {
	json    string
	jsonCid string
}{
	json: `
{
  "name": "media",
  "pin": true,
  "nodes": {
    "raw": {
      "use": ":file",
      "mill": "/blob"
    },
    "exif": {
      "use": "raw",
      "mill": "/image/exif"
    },
    "encoded": {
      "nodes": {
        "large": {
		  "use": ":file",
		  "mill": "/image/resize",
		  "opts": {
			"width": "800",
			"quality": "80"
		  }
		},
		"small": {
		  "use": ":file",
		  "mill": "/image/resize",
		  "opts": {
			"width": "320",
			"quality": "80"
		  }
		},
		"thumb": {
		  "use": "large",
		  "pin": true,
		  "mill": "/image/resize",
		  "opts": {
			"width": "100",
			"quality": "80"
		  }
		}
      }
    }
  }
}
`,
	jsonCid: "bafyreiftulgtjfjd2suc2v3f6ikssj7w6bklzg3tsoz3wp4erlarkqrloy",
}

func TestFromJSON(t *testing.T) {
	schema, err := FromJSON(bytes.NewReader([]byte(vars.json)))
	if err != nil {
		t.Fatal(err)
	}

	if schema.Cid().String() != vars.jsonCid {
		t.Fatalf("unexpected cid: %s", schema.Cid().String())
	}

	err = dag.Add(context.Background(), schema)
	if err != nil {
		t.Fatal(err)
	}
}

func TestFromCid(t *testing.T) {
	c, err := cid.Decode(vars.jsonCid)
	if err != nil {
		t.Fatal(err)
	}

	schema, err := FromCid(context.Background(), c, dag)
	if err != nil {
		t.Fatal(err)
	}

	if schema.Cid().String() != vars.jsonCid {
		t.Fatalf("unexpected cid: %s", schema.Cid().String())
	}

	subnodes := schema.Nodes()["encoded"].Nodes()
	if len(subnodes) != 3 {
		t.Fatalf("unexpected node count: %d", len(subnodes))
	}
}
