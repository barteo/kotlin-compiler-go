package classfile

import (
	"math/rand"
	"testing"
	"unicode/utf16"
)

// decodeModifiedUTF8Ref is decodeModifiedUTF8 as it was, through UTF-16
// units, which the direct decoder must agree with.
func decodeModifiedUTF8Ref(b []byte) string {
	u := make([]uint16, 0, len(b))
	for i := 0; i < len(b); {
		c := b[i]
		switch {
		case c < 0x80:
			u = append(u, uint16(c))
			i++
		case c&0xE0 == 0xC0 && i+1 < len(b):
			u = append(u, uint16(c&0x1F)<<6|uint16(b[i+1]&0x3F))
			i += 2
		case i+2 < len(b):
			u = append(u, uint16(c&0x0F)<<12|uint16(b[i+1]&0x3F)<<6|uint16(b[i+2]&0x3F))
			i += 3
		default:
			i = len(b)
		}
	}
	return string(utf16.Decode(u))
}

func TestDecodeModifiedUTF8(t *testing.T) {
	cases := [][]byte{
		nil,
		[]byte("plain"),
		{0xC0, 0x80},                         // NUL
		{0xC3, 0xA9},                         // é
		{0xE2, 0x82, 0xAC},                   // €
		{0xED, 0xA0, 0xBD, 0xED, 0xB8, 0x80}, // U+1F600 as a surrogate pair
		{0xED, 0xA0, 0xBD},                   // a lone high surrogate
		{0xED, 0xB8, 0x80},                   // a lone low surrogate
		{0xED, 0xA0, 0xBD, 'x'},              // a high surrogate, then not a low one
		{0xED, 0xA0, 0xBD, 0xED, 0xA0, 0xBD}, // two high surrogates
		{0xC3},                               // truncated
		{0xE2, 0x82},                         // truncated
		{'a', 0xFF, 'b', 'c'},                // not modified UTF-8
		{0x00, 0x80, 0xBF},
	}
	for _, b := range cases {
		if got, want := decodeModifiedUTF8(b), decodeModifiedUTF8Ref(b); got != want {
			t.Errorf("decodeModifiedUTF8(% x) = %q, want %q", b, got, want)
		}
	}
	r := rand.New(rand.NewSource(1))
	pick := []byte{0x00, 'a', 0x7F, 0x80, 0xBF, 0xC0, 0xC3, 0xDF, 0xE0, 0xED, 0xA0, 0xB0, 0xEF, 0xF0, 0xFF}
	for range 200000 {
		b := make([]byte, r.Intn(12))
		for i := range b {
			if r.Intn(2) == 0 {
				b[i] = pick[r.Intn(len(pick))]
			} else {
				b[i] = byte(r.Intn(256))
			}
		}
		if got, want := decodeModifiedUTF8(b), decodeModifiedUTF8Ref(b); got != want {
			t.Fatalf("decodeModifiedUTF8(% x) = %q, want %q", b, got, want)
		}
	}
}

func BenchmarkDecodeModifiedUTF8(b *testing.B) {
	// As Kotlin metadata's strings are: bytes as characters, many of
	// them two-byte sequences.
	src := make([]byte, 0, 3000)
	for i := range 2000 {
		c := byte(i * 7)
		if c < 0x80 && c != 0 {
			src = append(src, c)
		} else {
			src = append(src, 0xC0|c>>6, 0x80|c&0x3F)
		}
	}
	b.Run("direct", func(b *testing.B) {
		for range b.N {
			decodeModifiedUTF8(src)
		}
	})
	b.Run("utf16", func(b *testing.B) {
		for range b.N {
			decodeModifiedUTF8Ref(src)
		}
	})
}
