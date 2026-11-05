// Package wire is the byte format of libkc's requests and responses
// (internal/kcabi). A message is a sequence of values with no framing or
// tags: both sides know an operation's layout. Its JVM twin is
// io.github.barteo.kc.Wire in jvm/libkc.
//
//   - Uint is an unsigned LEB128 varint, Int a zigzag one;
//   - Bool is a byte, 0 or 1;
//   - String and Bytes are a Uint length and the bytes, UTF-8 for String.
package wire

import (
	"encoding/binary"
	"errors"
)

// Writer builds a message. The zero value is ready to use.
type Writer struct {
	buf []byte
}

// Message returns the message.
func (w *Writer) Message() []byte { return w.buf }

func (w *Writer) Uint(x uint64) { w.buf = binary.AppendUvarint(w.buf, x) }
func (w *Writer) Int(x int64)   { w.buf = binary.AppendVarint(w.buf, x) }

func (w *Writer) Bool(b bool) {
	if b {
		w.buf = append(w.buf, 1)
	} else {
		w.buf = append(w.buf, 0)
	}
}

func (w *Writer) String(s string) {
	w.Uint(uint64(len(s)))
	w.buf = append(w.buf, s...)
}

func (w *Writer) Bytes(b []byte) {
	w.Uint(uint64(len(b)))
	w.buf = append(w.buf, b...)
}

// ErrShort is the error of a Reader that ran past the end of its message
// or read a malformed value.
var ErrShort = errors.New("wire: malformed message")

// Reader reads a message. After a malformed value it returns zero values,
// and Err reports ErrShort.
type Reader struct {
	buf []byte
	err error
}

// NewReader returns a reader of message b.
func NewReader(b []byte) *Reader { return &Reader{buf: b} }

// Err returns ErrShort if a read failed.
func (r *Reader) Err() error { return r.err }

// Len returns the number of bytes left to read.
func (r *Reader) Len() int { return len(r.buf) }

func (r *Reader) fail() { r.err, r.buf = ErrShort, nil }

func (r *Reader) Uint() uint64 {
	x, n := binary.Uvarint(r.buf)
	if n <= 0 {
		r.fail()
		return 0
	}
	r.buf = r.buf[n:]
	return x
}

func (r *Reader) Int() int64 {
	x, n := binary.Varint(r.buf)
	if n <= 0 {
		r.fail()
		return 0
	}
	r.buf = r.buf[n:]
	return x
}

func (r *Reader) Bool() bool {
	if len(r.buf) == 0 || r.buf[0] > 1 {
		r.fail()
		return false
	}
	b := r.buf[0] == 1
	r.buf = r.buf[1:]
	return b
}

func (r *Reader) bytes() []byte {
	n := r.Uint()
	if r.err != nil || n > uint64(len(r.buf)) {
		r.fail()
		return nil
	}
	b := r.buf[:n:n]
	r.buf = r.buf[n:]
	return b
}

func (r *Reader) String() string { return string(r.bytes()) }

// Bytes reads a byte string, which shares the message's memory.
func (r *Reader) Bytes() []byte { return r.bytes() }
