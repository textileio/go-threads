// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: tstore.proto

package thread_pb

import (
	fmt "fmt"
	_ "github.com/gogo/protobuf/gogoproto"
	proto "github.com/gogo/protobuf/proto"
	io "io"
	math "math"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.GoGoProtoPackageIsVersion2 // please upgrade the proto package

// AddrBookRecord represents a record for a log in the address book.
type AddrBookRecord struct {
	// Thread ID
	ThreadID *ProtoThreadID `protobuf:"bytes,1,opt,name=threadID,proto3,customtype=ProtoThreadID" json:"threadID,omitempty"`
	// The peer ID.
	PeerID *ProtoPeerID `protobuf:"bytes,2,opt,name=peerID,proto3,customtype=ProtoPeerID" json:"peerID,omitempty"`
	// The multiaddresses. This is a sorted list where element 0 expires the soonest.
	Addrs []*AddrBookRecord_AddrEntry `protobuf:"bytes,3,rep,name=addrs,proto3" json:"addrs,omitempty"`
}

func (m *AddrBookRecord) Reset()         { *m = AddrBookRecord{} }
func (m *AddrBookRecord) String() string { return proto.CompactTextString(m) }
func (*AddrBookRecord) ProtoMessage()    {}
func (*AddrBookRecord) Descriptor() ([]byte, []int) {
	return fileDescriptor_74b294050729ac9e, []int{0}
}
func (m *AddrBookRecord) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *AddrBookRecord) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_AddrBookRecord.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalTo(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *AddrBookRecord) XXX_Merge(src proto.Message) {
	xxx_messageInfo_AddrBookRecord.Merge(m, src)
}
func (m *AddrBookRecord) XXX_Size() int {
	return m.Size()
}
func (m *AddrBookRecord) XXX_DiscardUnknown() {
	xxx_messageInfo_AddrBookRecord.DiscardUnknown(m)
}

var xxx_messageInfo_AddrBookRecord proto.InternalMessageInfo

func (m *AddrBookRecord) GetAddrs() []*AddrBookRecord_AddrEntry {
	if m != nil {
		return m.Addrs
	}
	return nil
}

// AddrEntry represents a single multiaddress.
type AddrBookRecord_AddrEntry struct {
	Addr *ProtoAddr `protobuf:"bytes,1,opt,name=addr,proto3,customtype=ProtoAddr" json:"addr,omitempty"`
	// The point in time when this address expires.
	Expiry int64 `protobuf:"varint,2,opt,name=expiry,proto3" json:"expiry,omitempty"`
	// The original TTL of this address.
	Ttl int64 `protobuf:"varint,3,opt,name=ttl,proto3" json:"ttl,omitempty"`
}

func (m *AddrBookRecord_AddrEntry) Reset()         { *m = AddrBookRecord_AddrEntry{} }
func (m *AddrBookRecord_AddrEntry) String() string { return proto.CompactTextString(m) }
func (*AddrBookRecord_AddrEntry) ProtoMessage()    {}
func (*AddrBookRecord_AddrEntry) Descriptor() ([]byte, []int) {
	return fileDescriptor_74b294050729ac9e, []int{0, 0}
}
func (m *AddrBookRecord_AddrEntry) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *AddrBookRecord_AddrEntry) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_AddrBookRecord_AddrEntry.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalTo(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *AddrBookRecord_AddrEntry) XXX_Merge(src proto.Message) {
	xxx_messageInfo_AddrBookRecord_AddrEntry.Merge(m, src)
}
func (m *AddrBookRecord_AddrEntry) XXX_Size() int {
	return m.Size()
}
func (m *AddrBookRecord_AddrEntry) XXX_DiscardUnknown() {
	xxx_messageInfo_AddrBookRecord_AddrEntry.DiscardUnknown(m)
}

var xxx_messageInfo_AddrBookRecord_AddrEntry proto.InternalMessageInfo

func (m *AddrBookRecord_AddrEntry) GetExpiry() int64 {
	if m != nil {
		return m.Expiry
	}
	return 0
}

func (m *AddrBookRecord_AddrEntry) GetTtl() int64 {
	if m != nil {
		return m.Ttl
	}
	return 0
}

func init() {
	proto.RegisterType((*AddrBookRecord)(nil), "thread.pb.AddrBookRecord")
	proto.RegisterType((*AddrBookRecord_AddrEntry)(nil), "thread.pb.AddrBookRecord.AddrEntry")
}

func init() { proto.RegisterFile("tstore.proto", fileDescriptor_74b294050729ac9e) }

var fileDescriptor_74b294050729ac9e = []byte{
	// 276 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0xe2, 0x29, 0x29, 0x2e, 0xc9,
	0x2f, 0x4a, 0xd5, 0x2b, 0x28, 0xca, 0x2f, 0xc9, 0x17, 0xe2, 0x2c, 0xc9, 0x28, 0x4a, 0x4d, 0x4c,
	0xd1, 0x2b, 0x48, 0x92, 0xd2, 0x4d, 0xcf, 0x2c, 0xc9, 0x28, 0x4d, 0xd2, 0x4b, 0xce, 0xcf, 0xd5,
	0x4f, 0xcf, 0x4f, 0xcf, 0xd7, 0x07, 0xab, 0x48, 0x2a, 0x4d, 0x03, 0xf3, 0xc0, 0x1c, 0x30, 0x0b,
	0xa2, 0x53, 0xe9, 0x2f, 0x23, 0x17, 0x9f, 0x63, 0x4a, 0x4a, 0x91, 0x53, 0x7e, 0x7e, 0x76, 0x50,
	0x6a, 0x72, 0x7e, 0x51, 0x8a, 0x90, 0x2e, 0x17, 0x07, 0xc4, 0x38, 0x4f, 0x17, 0x09, 0x46, 0x05,
	0x46, 0x0d, 0x1e, 0x27, 0xc1, 0x5b, 0xf7, 0xe4, 0x79, 0x03, 0x40, 0xea, 0x43, 0xa0, 0x12, 0x41,
	0x70, 0x25, 0x42, 0xea, 0x5c, 0x6c, 0x05, 0xa9, 0xa9, 0x45, 0x9e, 0x2e, 0x12, 0x4c, 0x60, 0xc5,
	0xfc, 0xb7, 0xee, 0xc9, 0x73, 0x83, 0x15, 0x07, 0x80, 0x85, 0x83, 0xa0, 0xd2, 0x42, 0x96, 0x5c,
	0xac, 0x89, 0x29, 0x29, 0x45, 0xc5, 0x12, 0xcc, 0x0a, 0xcc, 0x1a, 0xdc, 0x46, 0xca, 0x7a, 0x70,
	0x47, 0xeb, 0xa1, 0xba, 0x00, 0xcc, 0x75, 0xcd, 0x2b, 0x29, 0xaa, 0x0c, 0x82, 0xe8, 0x90, 0x8a,
	0xe0, 0xe2, 0x84, 0x8b, 0x09, 0x29, 0x72, 0xb1, 0x80, 0x44, 0xa1, 0x6e, 0xe3, 0xbd, 0x75, 0x4f,
	0x9e, 0x13, 0x6c, 0x1d, 0x48, 0x45, 0x10, 0x58, 0x4a, 0x48, 0x8c, 0x8b, 0x2d, 0xb5, 0xa2, 0x20,
	0xb3, 0xa8, 0x12, 0xec, 0x26, 0xe6, 0x20, 0x28, 0x4f, 0x48, 0x80, 0x8b, 0xb9, 0xa4, 0x24, 0x47,
	0x82, 0x19, 0x2c, 0x08, 0x62, 0x3a, 0x49, 0x9c, 0x78, 0x24, 0xc7, 0x78, 0xe1, 0x91, 0x1c, 0xe3,
	0x83, 0x47, 0x72, 0x8c, 0x13, 0x1e, 0xcb, 0x31, 0x5c, 0x78, 0x2c, 0xc7, 0x70, 0xe3, 0xb1, 0x1c,
	0x43, 0x12, 0x1b, 0x38, 0x80, 0x8c, 0x01, 0x01, 0x00, 0x00, 0xff, 0xff, 0x57, 0x46, 0x62, 0xf3,
	0x6a, 0x01, 0x00, 0x00,
}

func (m *AddrBookRecord) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalTo(dAtA)
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *AddrBookRecord) MarshalTo(dAtA []byte) (int, error) {
	var i int
	_ = i
	var l int
	_ = l
	if m.ThreadID != nil {
		dAtA[i] = 0xa
		i++
		i = encodeVarintTstore(dAtA, i, uint64(m.ThreadID.Size()))
		n1, err := m.ThreadID.MarshalTo(dAtA[i:])
		if err != nil {
			return 0, err
		}
		i += n1
	}
	if m.PeerID != nil {
		dAtA[i] = 0x12
		i++
		i = encodeVarintTstore(dAtA, i, uint64(m.PeerID.Size()))
		n2, err := m.PeerID.MarshalTo(dAtA[i:])
		if err != nil {
			return 0, err
		}
		i += n2
	}
	if len(m.Addrs) > 0 {
		for _, msg := range m.Addrs {
			dAtA[i] = 0x1a
			i++
			i = encodeVarintTstore(dAtA, i, uint64(msg.Size()))
			n, err := msg.MarshalTo(dAtA[i:])
			if err != nil {
				return 0, err
			}
			i += n
		}
	}
	return i, nil
}

func (m *AddrBookRecord_AddrEntry) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalTo(dAtA)
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *AddrBookRecord_AddrEntry) MarshalTo(dAtA []byte) (int, error) {
	var i int
	_ = i
	var l int
	_ = l
	if m.Addr != nil {
		dAtA[i] = 0xa
		i++
		i = encodeVarintTstore(dAtA, i, uint64(m.Addr.Size()))
		n3, err := m.Addr.MarshalTo(dAtA[i:])
		if err != nil {
			return 0, err
		}
		i += n3
	}
	if m.Expiry != 0 {
		dAtA[i] = 0x10
		i++
		i = encodeVarintTstore(dAtA, i, uint64(m.Expiry))
	}
	if m.Ttl != 0 {
		dAtA[i] = 0x18
		i++
		i = encodeVarintTstore(dAtA, i, uint64(m.Ttl))
	}
	return i, nil
}

func encodeVarintTstore(dAtA []byte, offset int, v uint64) int {
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return offset + 1
}
func (m *AddrBookRecord) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.ThreadID != nil {
		l = m.ThreadID.Size()
		n += 1 + l + sovTstore(uint64(l))
	}
	if m.PeerID != nil {
		l = m.PeerID.Size()
		n += 1 + l + sovTstore(uint64(l))
	}
	if len(m.Addrs) > 0 {
		for _, e := range m.Addrs {
			l = e.Size()
			n += 1 + l + sovTstore(uint64(l))
		}
	}
	return n
}

func (m *AddrBookRecord_AddrEntry) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.Addr != nil {
		l = m.Addr.Size()
		n += 1 + l + sovTstore(uint64(l))
	}
	if m.Expiry != 0 {
		n += 1 + sovTstore(uint64(m.Expiry))
	}
	if m.Ttl != 0 {
		n += 1 + sovTstore(uint64(m.Ttl))
	}
	return n
}

func sovTstore(x uint64) (n int) {
	for {
		n++
		x >>= 7
		if x == 0 {
			break
		}
	}
	return n
}
func sozTstore(x uint64) (n int) {
	return sovTstore(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (m *AddrBookRecord) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowTstore
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: AddrBookRecord: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: AddrBookRecord: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field ThreadID", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowTstore
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				byteLen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if byteLen < 0 {
				return ErrInvalidLengthTstore
			}
			postIndex := iNdEx + byteLen
			if postIndex < 0 {
				return ErrInvalidLengthTstore
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			var v ProtoThreadID
			m.ThreadID = &v
			if err := m.ThreadID.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field PeerID", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowTstore
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				byteLen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if byteLen < 0 {
				return ErrInvalidLengthTstore
			}
			postIndex := iNdEx + byteLen
			if postIndex < 0 {
				return ErrInvalidLengthTstore
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			var v ProtoPeerID
			m.PeerID = &v
			if err := m.PeerID.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 3:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Addrs", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowTstore
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthTstore
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthTstore
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Addrs = append(m.Addrs, &AddrBookRecord_AddrEntry{})
			if err := m.Addrs[len(m.Addrs)-1].Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipTstore(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthTstore
			}
			if (iNdEx + skippy) < 0 {
				return ErrInvalidLengthTstore
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *AddrBookRecord_AddrEntry) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowTstore
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: AddrEntry: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: AddrEntry: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Addr", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowTstore
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				byteLen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if byteLen < 0 {
				return ErrInvalidLengthTstore
			}
			postIndex := iNdEx + byteLen
			if postIndex < 0 {
				return ErrInvalidLengthTstore
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			var v ProtoAddr
			m.Addr = &v
			if err := m.Addr.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 2:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Expiry", wireType)
			}
			m.Expiry = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowTstore
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.Expiry |= int64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 3:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Ttl", wireType)
			}
			m.Ttl = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowTstore
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.Ttl |= int64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		default:
			iNdEx = preIndex
			skippy, err := skipTstore(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthTstore
			}
			if (iNdEx + skippy) < 0 {
				return ErrInvalidLengthTstore
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func skipTstore(dAtA []byte) (n int, err error) {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowTstore
			}
			if iNdEx >= l {
				return 0, io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		wireType := int(wire & 0x7)
		switch wireType {
		case 0:
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowTstore
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				iNdEx++
				if dAtA[iNdEx-1] < 0x80 {
					break
				}
			}
			return iNdEx, nil
		case 1:
			iNdEx += 8
			return iNdEx, nil
		case 2:
			var length int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowTstore
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				length |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if length < 0 {
				return 0, ErrInvalidLengthTstore
			}
			iNdEx += length
			if iNdEx < 0 {
				return 0, ErrInvalidLengthTstore
			}
			return iNdEx, nil
		case 3:
			for {
				var innerWire uint64
				var start int = iNdEx
				for shift := uint(0); ; shift += 7 {
					if shift >= 64 {
						return 0, ErrIntOverflowTstore
					}
					if iNdEx >= l {
						return 0, io.ErrUnexpectedEOF
					}
					b := dAtA[iNdEx]
					iNdEx++
					innerWire |= (uint64(b) & 0x7F) << shift
					if b < 0x80 {
						break
					}
				}
				innerWireType := int(innerWire & 0x7)
				if innerWireType == 4 {
					break
				}
				next, err := skipTstore(dAtA[start:])
				if err != nil {
					return 0, err
				}
				iNdEx = start + next
				if iNdEx < 0 {
					return 0, ErrInvalidLengthTstore
				}
			}
			return iNdEx, nil
		case 4:
			return iNdEx, nil
		case 5:
			iNdEx += 4
			return iNdEx, nil
		default:
			return 0, fmt.Errorf("proto: illegal wireType %d", wireType)
		}
	}
	panic("unreachable")
}

var (
	ErrInvalidLengthTstore = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowTstore   = fmt.Errorf("proto: integer overflow")
)
