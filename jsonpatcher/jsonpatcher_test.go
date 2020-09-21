package jsonpatcher

import (
	"testing"
	"time"

	cbornode "github.com/ipfs/go-ipld-cbor"
	mh "github.com/multiformats/go-multihash"
	core "github.com/textileio/go-threads/core/db"
)

type patchEventOld struct {
	Timestamp      time.Time
	ID             core.InstanceID
	CollectionName string
	Patch          operation
}

func init() {
	cbornode.RegisterCborType(patchEventOld{})
	cbornode.RegisterCborType(time.Time{})
}

func TestJsonPatcher_Migration(t *testing.T) {
	in1 := patchEventOld{Timestamp: time.Now(), ID: "123", CollectionName: "abc"}
	node1, err := cbornode.WrapObject(in1, mh.SHA2_256, -1)
	if err != nil {
		t.Fatal(err)
	}
	out := new(patchEvent)
	err = cbornode.DecodeInto(node1.RawData(), &out)
	if err != nil {
		t.Errorf("failed to decode event with time.Time timestamp: %v", err)
	}
	if !out.time().Equal(time.Time{}) {
		t.Error("non-encodable time should have zero-value")
	}

	ts := time.Now()
	in2 := patchEvent{Timestamp: ts.UnixNano(), ID: "123", CollectionName: "abc"}
	node2, err := cbornode.WrapObject(in2, mh.SHA2_256, -1)
	if err != nil {
		t.Fatal(err)
	}
	err = cbornode.DecodeInto(node2.RawData(), &out)
	if err != nil {
		t.Errorf("failed to decode event with int64 timestamp: %v", err)
	}
	tsout := out.time()
	if !tsout.Equal(ts) {
		t.Error("encodable time should be equal to input")
	}
}
