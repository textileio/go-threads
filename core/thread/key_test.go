package thread

import (
	"bytes"
	"testing"
)

func TestNewRandomKey(t *testing.T) {
	k := NewRandomKey()
	if k.sk == nil {
		t.Fatal("service key should not be nil")
	}
	if k.rk == nil {
		t.Fatal("read key should not be nil")
	}
}

func TestNewRandomServiceKey(t *testing.T) {
	k := NewRandomServiceKey()
	if k.sk == nil {
		t.Fatal("service key should not be nil")
	}
	if k.rk != nil {
		t.Fatal("read key should be nil")
	}
}

func TestKey_FromBytes(t *testing.T) {
	t.Run("full", func(t *testing.T) {
		k1 := NewRandomKey()
		b := k1.Bytes()
		k2, err := KeyFromBytes(b)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(k2.sk.Bytes(), k1.sk.Bytes()) {
			t.Fatal("service keys are not equal")
		}
		if !bytes.Equal(k2.rk.Bytes(), k1.rk.Bytes()) {
			t.Fatal("read keys are not equal")
		}
	})
	t.Run("service", func(t *testing.T) {
		k1 := NewRandomServiceKey()
		b := k1.Bytes()
		k2, err := KeyFromBytes(b)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(k2.sk.Bytes(), k1.sk.Bytes()) {
			t.Fatal("service keys are not equal")
		}
		if k2.rk != nil {
			t.Fatal("read key should be nil")
		}
	})
}

func TestKey_FromString(t *testing.T) {
	t.Run("full", func(t *testing.T) {
		k1 := NewRandomKey()
		s := k1.String()
		k2, err := KeyFromString(s)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(k2.sk.Bytes(), k1.sk.Bytes()) {
			t.Fatal("service keys are not equal")
		}
		if !bytes.Equal(k2.rk.Bytes(), k1.rk.Bytes()) {
			t.Fatal("read keys are not equal")
		}
	})
	t.Run("service", func(t *testing.T) {
		k1 := NewRandomServiceKey()
		s := k1.String()
		k2, err := KeyFromString(s)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(k2.sk.Bytes(), k1.sk.Bytes()) {
			t.Fatal("service keys are not equal")
		}
		if k2.rk != nil {
			t.Fatal("read key should be nil")
		}
	})
}
