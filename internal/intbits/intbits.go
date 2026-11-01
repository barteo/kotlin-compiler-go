// Package intbits narrows and reinterprets integer values the way the JVM
// does: by keeping their low bits. It does so with int64 arithmetic and
// range-checked conversions, so no value is truncated by an unchecked Go
// conversion.
package intbits

import "math"

// SignExtend returns the low bits of i, sign-extended: the value Java's
// narrowing conversion to a signed type of that width gives, as (byte) 300
// is 44 and (int) 0xFFFFFFFFL is -1. bits is between 1 and 64.
func SignExtend(i int64, bits uint) int64 {
	s := 64 - bits
	return i << s >> s
}

// ZeroExtend returns the low bits of i, zero-extended: the value Java's
// narrowing conversion to char gives, as (char) -1 is 65535. bits is
// between 1 and 63.
func ZeroExtend(i int64, bits uint) int64 {
	return i & (1<<bits - 1)
}

// FromBits returns the int64 whose two's-complement bits are u, as the
// literal 0xFFFFFFFFFFFFFFFFL is -1.
func FromBits(u uint64) int64 {
	if u <= math.MaxInt64 {
		return int64(u)
	}
	// The top bit is set, so the complement fits in an int64, and
	// complementing it back gives the negative value with u's bits.
	if c := ^u; c <= math.MaxInt64 {
		return ^int64(c)
	}
	panic("unreachable")
}
