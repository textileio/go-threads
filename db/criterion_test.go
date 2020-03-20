package db

import (
	"fmt"
	"math/big"
	"testing"
	"time"
)

// Most meaningful tests for queries are in query_test.go
// making full coverage for criterion needs a more synthetic
// testing like done here because of exploding combination of
// Go types and < > == operations.

func TestCompare(t *testing.T) {
	t.Parallel()
	tests := []struct {
		// a < b < c
		a interface{}
		b interface{}
		c interface{}
	}{
		{a: time.Now().Add(-time.Hour), b: time.Now(), c: time.Now().Add(time.Hour)},
		{a: *big.NewFloat(1.01), b: *big.NewFloat(2.2), c: *big.NewFloat(5.4)},
		{a: *big.NewInt(1), b: *big.NewInt(2), c: *big.NewInt(5)},
		{a: *big.NewRat(1, 3), b: *big.NewRat(1, 2), c: *big.NewRat(3, 2)},
		{a: 0, b: 1, c: 2},
		{a: int8(-2), b: int8(1), c: int8(2)},
		{a: int16(-2), b: int16(1), c: int16(2)},
		{a: int32(-2), b: int32(1), c: int32(2)},
		{a: int64(-2), b: int64(1), c: int64(2)},
		{a: uint(0), b: uint(1), c: uint(2)},
		{a: uint8(0), b: uint8(1), c: uint8(2)},
		{a: uint16(0), b: uint16(1), c: uint16(2)},
		{a: uint32(0), b: uint32(1), c: uint32(2)},
		{a: uint64(0), b: uint64(1), c: uint64(2)},
		{a: float32(-0.3), b: float32(0.6), c: float32(2.21)},
		{a: float64(-0.3), b: float64(0.6), c: float64(2.21)},
		{a: "a", b: "b", c: "c"},
		{a: &tstComparer{Val: -1}, b: &tstComparer{Val: 2}, c: &tstComparer{Val: 3}},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(fmt.Sprintf("%T", tc.a), func(t *testing.T) {
			t.Parallel()
			if compareTst(t, tc.a, tc.b) != -1 ||
				compareTst(t, tc.a, tc.c) != -1 ||
				compareTst(t, tc.b, tc.c) != -1 ||
				compareTst(t, tc.c, tc.a) != 1 ||
				compareTst(t, tc.c, tc.b) != 1 ||
				compareTst(t, tc.b, tc.a) != 1 ||
				compareTst(t, tc.a, tc.a) != 0 ||
				compareTst(t, tc.b, tc.b) != 0 ||
				compareTst(t, tc.c, tc.c) != 0 {
				t.Fatalf("compare incorrect result: %v", tc)
			}
		})
	}
}

func compareTst(t *testing.T, v1, v2 interface{}) int {
	r, err := compare(v1, v2)
	if err != nil {
		t.Fatalf("error when calling compare: %v", err)
	}
	return r
}

type tstComparer struct {
	Val int
}

func (t *tstComparer) Compare(other interface{}) (int, error) {
	v := other.(*tstComparer)
	if t.Val == v.Val {
		return 0, nil
	}
	if t.Val < v.Val {
		return -1, nil
	}
	return 1, nil
}
