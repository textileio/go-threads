package thread

import (
	"testing"

	mbase "github.com/multiformats/go-multibase"
)

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

	t.Logf("Variant: %s", VariantToStr[v])

	i = NewIDV1(AccessControlled, 32)

	v = i.Variant()
	if v != AccessControlled {
		t.Errorf("got wrong variant from %s: %d", i.String(), v)
	}

	t.Logf("Variant: %s", VariantToStr[v])
}
