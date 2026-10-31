package classfile

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestPoolUTF8Encoding(t *testing.T) {
	for _, tc := range []struct {
		text    string
		encoded []byte
	}{
		{"", nil},
		{"ASCII", []byte("ASCII")},
		{"a\x00b", []byte{0x61, 0xc0, 0x80, 0x62}},
		{"żółw", []byte("żółw")},
		{"日本語", []byte("日本語")},
		{"😀", []byte{0xed, 0xa0, 0xbd, 0xed, 0xb8, 0x80}},
	} {
		var p Pool
		if got := p.Utf8(tc.text); got != 1 {
			t.Fatalf("Utf8(%q) index = %d, want 1", tc.text, got)
		}
		if p.buf[0] != tagUtf8 || int(binary.BigEndian.Uint16(p.buf[1:3])) != len(tc.encoded) || !bytes.Equal(p.buf[3:], tc.encoded) {
			t.Errorf("Utf8(%q) encoded as %x, want %x", tc.text, p.buf, tc.encoded)
		}
		if got := p.utf8At(1); got != tc.text {
			t.Errorf("utf8At(1) = %q, want %q", got, tc.text)
		}
	}
}

func TestPoolRepeatedConstants(t *testing.T) {
	var p Pool
	for _, add := range []func() uint16{
		func() uint16 { return p.Utf8("repeated") },
		func() uint16 { return p.Class("demo/A") },
		func() uint16 { return p.String("repeated") },
		func() uint16 { return p.NameAndType("method", "()V") },
		func() uint16 { return p.Field("demo/A", "field", "I") },
		func() uint16 { return p.Method("demo/A", "method", "()V") },
		func() uint16 { return p.InterfaceMethod("demo/I", "method", "()V") },
	} {
		first := add()
		size, count := len(p.buf), p.count
		if got := add(); got != first || len(p.buf) != size || p.count != count {
			t.Fatalf("repeated constant changed pool: index %d -> %d, size %d -> %d, count %d -> %d", first, got, size, len(p.buf), count, p.count)
		}
	}
	if n := testing.AllocsPerRun(100, func() { p.Utf8("repeated") }); n != 0 {
		t.Errorf("repeated UTF8 constant allocated %g times, want 0", n)
	}
}
