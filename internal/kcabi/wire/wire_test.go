package wire

import (
	"bytes"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	var w Writer
	w.Uint(0)
	w.Uint(1 << 40)
	w.Int(-5)
	w.Bool(true)
	w.String("héllo")
	w.Bytes([]byte{1, 2})
	r := NewReader(w.Message())
	if r.Uint() != 0 || r.Uint() != 1<<40 || r.Int() != -5 || !r.Bool() || r.String() != "héllo" ||
		!bytes.Equal(r.Bytes(), []byte{1, 2}) || r.Err() != nil {
		t.Fatal("round trip failed")
	}
	if r.Uint(); r.Err() != ErrShort {
		t.Fatal("read past the end succeeded")
	}
}

func TestMalformed(t *testing.T) {
	var w Writer
	w.Uint(10) // a length longer than the rest
	w.Bytes([]byte("abc"))
	r := NewReader(w.Message())
	if s := r.String(); s != "" || r.Err() != ErrShort {
		t.Fatalf("got %q, %v", s, r.Err())
	}
	if r.Bool() || r.Err() != ErrShort {
		t.Fatal("reader recovered after an error")
	}
}

func FuzzReader(f *testing.F) {
	f.Add([]byte{3, 'a', 'b', 'c', 1, 0x80})
	f.Fuzz(func(t *testing.T, b []byte) {
		r := NewReader(b)
		for r.Err() == nil {
			_ = r.String()
			r.Bool()
			r.Int()
		}
	})
}
