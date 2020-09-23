package jsonpatcher

import (
	"bytes"
	"encoding/gob"
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
	gob.Register(map[string]interface {}{})
}

func TestJsonPatcher_Migration(t *testing.T) {
	in1 := patchEventOld{Timestamp: time.Now(), ID: "123", CollectionName: "abc"}
	node1, err := cbornode.WrapObject(in1, mh.SHA2_256, -1)
	if err != nil {
		t.Fatal(err)
	}
	b := bytes.Buffer{}
	e := gob.NewEncoder(&b)
	if err := e.Encode(in1); err != nil {
		t.Errorf("failed to gob encode event with time.Time timestamp: %v", err)
	}

	out := new(patchEvent)
	err = cbornode.DecodeInto(node1.RawData(), &out)
	if err != nil {
		t.Errorf("failed to decode event with time.Time timestamp: %v", err)
	}
	if !out.time().Equal(time.Time{}) {
		t.Error("non-encodable time should have zero-value")
	}
	b.Reset()
	e = gob.NewEncoder(&b)
	if err := e.Encode(out); err != nil {
		t.Errorf("failed to gob encode event with empty time.Time timestamp: %v", err)
	}

	ts := time.Now()
	in2 := patchEvent{Timestamp: ts.UnixNano(), ID: "123", CollectionName: "abc"}
	node2, err := cbornode.WrapObject(in2, mh.SHA2_256, -1)
	if err != nil {
		t.Fatal(err)
	}

	b.Reset()
	e = gob.NewEncoder(&b)
	if err := e.Encode(in2); err != nil {
		t.Errorf("failed to gob encode event with int64 timestamp: %v", err)
	}

	err = cbornode.DecodeInto(node2.RawData(), &out)
	if err != nil {
		t.Errorf("failed to decode event with int64 timestamp: %v", err)
	}
	tsout := out.time()

	b.Reset()
	e = gob.NewEncoder(&b)
	if err := e.Encode(out); err != nil {
		t.Errorf("failed to gob encode event with int64 timestamp: %v", err)
	}

	if !tsout.Equal(ts) {
		t.Error("encodable time should be equal to input")
	}
}
