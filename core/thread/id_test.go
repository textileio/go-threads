package thread

import (
	"crypto/rand"
	"encoding/binary"
	"testing"

	mbase "github.com/multiformats/go-multibase"
)

func TestCast(t *testing.T) {
	i := NewIDV1(Raw, 32)
	j, err := Cast(i.Bytes())
	if err != nil {
		t.Errorf("failed to cast ID %s: %s", i.String(), err)
	}
	if i != j {
		t.Errorf("id %v not equal to id %v", i.String(), j.String())
	}
}

func TestDecode(t *testing.T) {
	i := NewIDV1(Raw, 32)
	t.Logf("New ID: %s", i.String())

	j, err := Decode(i.String())
	if err != nil {
		t.Errorf("failed to decode ID %s: %s", i.String(), err)
	}

	t.Logf("Decoded ID: %s", j.String())
}

func TestExtractEncoding(t *testing.T) {
	i := NewIDV1(Raw, 16)

	e, err := ExtractEncoding(i.String())
	if err != nil {
		t.Errorf("failed to extract encoding from %s: %s", i.String(), err)
	}

	t.Logf("Encoding: %s", mbase.EncodingToStr[e])
}

func TestID_Version(t *testing.T) {
	i := NewIDV1(Raw, 16)

	v := i.Version()
	if v != V1 {
		t.Errorf("got wrong version from %s: %d", i.String(), v)
	}

	t.Logf("Version: %d", v)
}

func TestID_Variant(t *testing.T) {
	i := NewIDV1(Raw, 16)

	v := i.Variant()
	if v != Raw {
		t.Errorf("got wrong variant from %s: %d", i.String(), v)
	}

	t.Logf("Variant: %s", v)

	i = NewIDV1(AccessControlled, 32)

	v = i.Variant()
	if v != AccessControlled {
		t.Errorf("got wrong variant from %s: %d", i.String(), v)
	}

	t.Logf("Variant: %s", v)
}

func TestID_Valid(t *testing.T) {
	i := NewIDV1(Raw, 16)
	if err := i.Validate(); err != nil {
		t.Errorf("id %s is invalid", i.String())
	}
}

func TestID_Invalid(t *testing.T) {
	i := makeID(t, 5, int64(Raw), 16)
	if err := i.Validate(); err == nil {
		t.Errorf("id %s is valid but it has an invalid version", i.String())
	}

	i = makeID(t, V1, 50, 16)
	if err := i.Validate(); err == nil {
		t.Errorf("id %s is valid but it has an invalid variant", i.String())
	}

	i = makeID(t, V1, int64(Raw), 0)
	if err := i.Validate(); err == nil {
		t.Errorf("id %s is valid but it has no random bytes", i.String())
	}
}

func makeID(t *testing.T, version uint64, variant int64, size uint8) ID {
	num := make([]byte, size)
	_, err := rand.Read(num)
	if err != nil {
		t.Errorf("failed to generate random data: %v", err)
	}

	numlen := len(num)
	// two 8 bytes (max) numbers plus num
	buf := make([]byte, 2*binary.MaxVarintLen64+numlen)
	n := binary.PutUvarint(buf, version)
	n += binary.PutUvarint(buf[n:], uint64(variant))
	cn := copy(buf[n:], num)
	if cn != numlen {
		t.Errorf("copy length is inconsistent")
	}

	return ID(buf[:n+numlen])
}
