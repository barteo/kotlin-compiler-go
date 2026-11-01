package intbits

import (
	"math"
	"testing"
)

func TestSignExtend(t *testing.T) {
	for _, c := range []struct {
		i    int64
		bits uint
		want int64
	}{
		{300, 8, 44},
		{128, 8, -128},
		{-129, 8, 127},
		{0xFFFF, 16, -1},
		{40000, 16, -25536},
		{1 << 31, 32, math.MinInt32},
		{0xFFFFFFFF, 32, -1},
		{math.MaxInt64, 32, -1},
		{-1, 64, -1},
	} {
		if got := SignExtend(c.i, c.bits); got != c.want {
			t.Errorf("SignExtend(%d, %d) = %d, want %d", c.i, c.bits, got, c.want)
		}
		// It agrees with Go's own truncating conversions.
		var conv int64
		switch c.bits {
		case 8:
			conv = int64(int8(c.i))
		case 16:
			conv = int64(int16(c.i))
		case 32:
			conv = int64(int32(c.i))
		default:
			conv = c.i
		}
		if conv != c.want {
			t.Errorf("conversion of %d to %d bits = %d, want %d", c.i, c.bits, conv, c.want)
		}
	}
}

func TestZeroExtend(t *testing.T) {
	for _, c := range []struct {
		i    int64
		bits uint
		want int64
	}{
		{-1, 16, 0xFFFF},
		{0x12345, 16, 0x2345},
		{65, 16, 65},
		{-1, 32, 0xFFFFFFFF},
		{math.MinInt32, 32, 1 << 31},
	} {
		if got := ZeroExtend(c.i, c.bits); got != c.want {
			t.Errorf("ZeroExtend(%d, %d) = %d, want %d", c.i, c.bits, got, c.want)
		}
	}
}

func TestFromBits(t *testing.T) {
	for _, u := range []uint64{0, 1, math.MaxInt64, 1 << 63, 1<<63 + 1, math.MaxUint64, 0xDEADBEEFCAFEBABE} {
		want := int64(u)
		if got := FromBits(u); got != want {
			t.Errorf("FromBits(%#x) = %d, want %d", u, got, want)
		}
	}
}
