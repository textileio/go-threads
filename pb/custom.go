package threads_pb

import (
	"encoding/binary"
	"encoding/json"

	"github.com/gogo/protobuf/proto"
	"github.com/ipfs/go-cid"
	ic "github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	pt "github.com/libp2p/go-libp2p-core/test"
	ma "github.com/multiformats/go-multiaddr"
	mh "github.com/multiformats/go-multihash"
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
	MarshalTo(data []byte) (n int, err error)
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

// ProtoCid is a custom type used by gogo to serde raw CIDs into the cid.CID type, and back.
type ProtoCid struct {
	cid.Cid
}

var _ customGogoType = (*ProtoCid)(nil)

func (c ProtoCid) Marshal() ([]byte, error) {
	return c.Bytes(), nil
}

func (c ProtoCid) MarshalTo(data []byte) (n int, err error) {
	return copy(data, c.Bytes()), nil
}

func (c ProtoCid) MarshalJSON() ([]byte, error) {
	m, _ := c.Marshal()
	return json.Marshal(m)
}

func (c *ProtoCid) Unmarshal(data []byte) (err error) {
	c.Cid, err = cid.Cast(data)
	if err == cid.ErrVarintBuffSmall {
		c.Cid = cid.Undef
		return nil
	}
	return err
}

func (c *ProtoCid) UnmarshalJSON(data []byte) error {
	v := new([]byte)
	err := json.Unmarshal(data, v)
	if err != nil {
		return err
	}
	return c.Unmarshal(*v)
}

func (c ProtoCid) Size() int {
	return len(c.Bytes())
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
	id.ID, err = thread.Cast(data)
	return err
}

func (id *ProtoThreadID) UnmarshalJSON(data []byte) error {
	v := new([]byte)
	err := json.Unmarshal(data, v)
	if err != nil {
		return err
	}
	return id.Unmarshal(*v)
}

func (id ProtoThreadID) Size() int {
	return len(id.Bytes())
}

// ProtoPubKey is a custom type used by gogo to serde raw public keys into the PubKey type, and back.
type ProtoPubKey struct {
	ic.PubKey
}

var _ customGogoType = (*ProtoPubKey)(nil)

func (k ProtoPubKey) Marshal() ([]byte, error) {
	return ic.MarshalPublicKey(k)
}

func (k ProtoPubKey) MarshalTo(data []byte) (n int, err error) {
	b, err := ic.MarshalPublicKey(k)
	return copy(data, b), err
}

func (k ProtoPubKey) MarshalJSON() ([]byte, error) {
	m, _ := k.Marshal()
	return json.Marshal(m)
}

func (k *ProtoPubKey) Unmarshal(data []byte) (err error) {
	k.PubKey, err = ic.UnmarshalPublicKey(data)
	return err
}

func (k *ProtoPubKey) UnmarshalJSON(data []byte) error {
	v := new([]byte)
	err := json.Unmarshal(data, v)
	if err != nil {
		return err
	}
	return k.Unmarshal(*v)
}

func (k ProtoPubKey) Size() int {
	b, _ := k.Marshal()
	return len(b)
}

// ProtoPrivKey is a custom type used by gogo to serde raw private keys into the PrivKey type, and back.
type ProtoPrivKey struct {
	ic.PrivKey
}

var _ customGogoType = (*ProtoPrivKey)(nil)

func (k ProtoPrivKey) Marshal() ([]byte, error) {
	return ic.MarshalPrivateKey(k)
}

func (k ProtoPrivKey) MarshalTo(data []byte) (n int, err error) {
	b, err := ic.MarshalPrivateKey(k)
	return copy(data, b), err
}

func (k ProtoPrivKey) MarshalJSON() ([]byte, error) {
	m, _ := k.Marshal()
	return json.Marshal(m)
}

func (k *ProtoPrivKey) Unmarshal(data []byte) (err error) {
	k.PrivKey, err = ic.UnmarshalPrivateKey(data)
	return err
}

func (k *ProtoPrivKey) UnmarshalJSON(data []byte) error {
	v := new([]byte)
	err := json.Unmarshal(data, v)
	if err != nil {
		return err
	}
	return k.Unmarshal(*v)
}

func (k ProtoPrivKey) Size() int {
	b, _ := k.Marshal()
	return len(b)
}

// HeadCID is a custom type used by gogo to serde raw log heads into a CIDtype, and back.
type HeadCid struct {
	cid.Cid
}

var _ customGogoType = (*HeadCid)(nil)

func (hc HeadCid) Marshal() ([]byte, error) {
	return hc.Bytes(), nil
}

func (hc *HeadCid) Unmarshal(data []byte) (err error) {
	hc.Cid, err = cid.Cast(data)
	return err
}

func (hc HeadCid) MarshalJSON() ([]byte, error) {
	m, _ := hc.Marshal()
	return json.Marshal(m)
}

func (hc *HeadCid) UnmarshalJSON(data []byte) error {
	var v []byte
	err := json.Unmarshal(data, &v)
	if err != nil {
		return err
	}
	return hc.Unmarshal(v)
}

func (hc *HeadCid) Size() int {
	return len(hc.Bytes())
}

func (hc HeadCid) MarshalTo(data []byte) (n int, err error) {
	return copy(data, []byte(hc.Bytes())), nil
}

// NewPopulatedHeadCid generates a populated instance of the custom gogo type HeadCid.
// It is required by gogo-generated tests.
func NewPopulatedHeadCid(r randyThreads) *HeadCid {
	var b []byte
	_ = binary.PutVarint(b, r.Int63())
	m, _ := mh.Sum(b, mh.SHA2_256, -1)
	cid := cid.NewCidV1(cid.Raw, m)
	return &HeadCid{Cid: cid}
}

// NewPopulatedProtoPeerID generates a populated instance of the custom gogo type ProtoPeerID.
// It is required by gogo-generated tests.
func NewPopulatedProtoPeerID(r randyThreads) *ProtoPeerID {
	id, _ := pt.RandPeerID()
	return &ProtoPeerID{ID: id}
}

// NewPopulatedProtoAddr generates a populated instance of the custom gogo type ProtoAddr.
// It is required by gogo-generated tests.
func NewPopulatedProtoAddr(r randyThreads) *ProtoAddr {
	a, _ := ma.NewMultiaddr("/ip4/123.123.123.123/tcp/7001")
	return &ProtoAddr{Multiaddr: a}
}

// NewPopulatedProtoCid generates a populated instance of the custom gogo type ProtoCid.
// It is required by gogo-generated tests.
func NewPopulatedProtoCid(r randyThreads) *ProtoCid {
	hash, _ := mh.Encode([]byte("hashy"), mh.SHA2_256)
	c := cid.NewCidV1(cid.DagCBOR, hash)
	return &ProtoCid{Cid: c}
}

// NewPopulatedProtoThreadID generates a populated instance of the custom gogo type ProtoThreadID.
// It is required by gogo-generated tests.
func NewPopulatedProtoThreadID(r randyThreads) *ProtoThreadID {
	id := thread.NewIDV1(thread.Raw, 16)
	return &ProtoThreadID{ID: id}
}

// NewPopulatedProtoPubKey generates a populated instance of the custom gogo type ProtoPubKey.
// It is required by gogo-generated tests.
func NewPopulatedProtoPubKey(r randyThreads) *ProtoPubKey {
	_, k, _ := ic.GenerateKeyPair(ic.RSA, 512)
	return &ProtoPubKey{PubKey: k}
}

// NewPopulatedProtoPrivKey generates a populated instance of the custom gogo type ProtoPrivKey.
// It is required by gogo-generated tests.
func NewPopulatedProtoPrivKey(r randyThreads) *ProtoPrivKey {
	k, _, _ := ic.GenerateKeyPair(ic.RSA, 512)
	return &ProtoPrivKey{PrivKey: k}
}
