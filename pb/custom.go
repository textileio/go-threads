package thread_pb

import (
	"encoding/json"

	proto "github.com/gogo/protobuf/proto"
	peer "github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/textileio/go-textile-core/thread"
)

// customGogoType aggregates the interfaces that custom Gogo types need to implement.
// it is only used for type assertions.
type customGogoType interface {
	proto.Marshaler
	proto.Unmarshaler
	json.Marshaler
	json.Unmarshaler
	proto.Sizer
}

// ProtoThreadID is a custom type used by gogo to serde raw thread IDs into the thread.ID type, and back.
type ProtoThreadID struct {
	thread.ID
}

var _ customGogoType = (*ProtoThreadID)(nil)

func (id ProtoThreadID) Marshal() ([]byte, error) {
	return id.Bytes(), nil
}

func (id ProtoThreadID) MarshalTo(data []byte) (n int, err error) {
	return copy(data, id.Bytes()), nil
}

func (id ProtoThreadID) MarshalJSON() ([]byte, error) {
	m, _ := id.Marshal()
	return json.Marshal(m)
}

func (id *ProtoThreadID) Unmarshal(data []byte) (err error) {
	id.ID, _ = thread.Cast(data)
	return nil
}

func (id *ProtoThreadID) UnmarshalJSON(data []byte) error {
	var v []byte
	err := json.Unmarshal(data, &v)
	if err != nil {
		return err
	}
	return id.Unmarshal(v)
}

func (id ProtoThreadID) Size() int {
	return len(id.ID.Bytes())
}

// ProtoPeerID is a custom type used by gogo to serde raw peer IDs into the peer.ID type, and back.
type ProtoPeerID struct {
	peer.ID
}

var _ customGogoType = (*ProtoPeerID)(nil)

func (id ProtoPeerID) Marshal() ([]byte, error) {
	return []byte(id.ID), nil
}

func (id ProtoPeerID) MarshalTo(data []byte) (n int, err error) {
	return copy(data, []byte(id.ID)), nil
}

func (id ProtoPeerID) MarshalJSON() ([]byte, error) {
	m, _ := id.Marshal()
	return json.Marshal(m)
}

func (id *ProtoPeerID) Unmarshal(data []byte) (err error) {
	id.ID = peer.ID(string(data))
	return nil
}

func (id *ProtoPeerID) UnmarshalJSON(data []byte) error {
	var v []byte
	err := json.Unmarshal(data, &v)
	if err != nil {
		return err
	}
	return id.Unmarshal(v)
}

func (id ProtoPeerID) Size() int {
	return len([]byte(id.ID))
}

// ProtoAddr is a custom type used by gogo to serde raw multiaddresses into the ma.Multiaddr type, and back.
type ProtoAddr struct {
	ma.Multiaddr
}

var _ customGogoType = (*ProtoAddr)(nil)

func (a ProtoAddr) Marshal() ([]byte, error) {
	return a.Bytes(), nil
}

func (a ProtoAddr) MarshalTo(data []byte) (n int, err error) {
	return copy(data, a.Bytes()), nil
}

func (a ProtoAddr) MarshalJSON() ([]byte, error) {
	m, _ := a.Marshal()
	return json.Marshal(m)
}

func (a *ProtoAddr) Unmarshal(data []byte) (err error) {
	a.Multiaddr, err = ma.NewMultiaddrBytes(data)
	return err
}

func (a *ProtoAddr) UnmarshalJSON(data []byte) error {
	v := new([]byte)
	err := json.Unmarshal(data, v)
	if err != nil {
		return err
	}
	return a.Unmarshal(*v)
}

func (a ProtoAddr) Size() int {
	return len(a.Bytes())
}
