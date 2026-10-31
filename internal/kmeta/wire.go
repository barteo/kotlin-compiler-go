package kmeta

import "fmt"

// decoder reads protobuf wire format. Malformed input panics with a
// decodeError, which the exported functions turn into an error.
type decoder struct{ b []byte }

type decodeError struct{ msg string }

func fail(format string, args ...any) {
	panic(decodeError{fmt.Sprintf(format, args...)})
}

// catch turns a decodeError panic into *err.
func catch(err *error) {
	if r := recover(); r != nil {
		de, ok := r.(decodeError)
		if !ok {
			panic(r)
		}
		*err = fmt.Errorf("kotlin metadata: %s", de.msg)
	}
}

const (
	wireVarint = 0
	wire64     = 1
	wireBytes  = 2
	wire32     = 5
)

func (d *decoder) more() bool { return len(d.b) > 0 }

func (d *decoder) varint() uint64 {
	var v uint64
	for shift := uint(0); shift < 64; shift += 7 {
		if len(d.b) == 0 {
			fail("truncated varint")
		}
		c := d.b[0]
		d.b = d.b[1:]
		v |= uint64(c&0x7f) << shift
		if c < 0x80 {
			return v
		}
	}
	fail("varint too long")
	return 0
}

// key reads a field's tag.
func (d *decoder) key() (field, wire int) {
	k := d.varint()
	return int(k >> 3), int(k & 7)
}

// int reads an int32 or enum field.
func (d *decoder) int() int { return int(int32(d.varint())) }

func (d *decoder) bool() bool { return d.varint() != 0 }

// bytes reads a length-delimited field: a string, an embedded message or a
// packed list.
func (d *decoder) bytes() []byte {
	n := d.varint()
	if n > uint64(len(d.b)) {
		fail("truncated field")
	}
	b := d.b[:n]
	d.b = d.b[n:]
	return b
}

func (d *decoder) skip(wire int) {
	switch wire {
	case wireVarint:
		d.varint()
	case wire64:
		d.fixed(8)
	case wireBytes:
		d.bytes()
	case wire32:
		d.fixed(4)
	default:
		fail("unsupported wire type %d", wire)
	}
}

func (d *decoder) fixed(n int) {
	if n > len(d.b) {
		fail("truncated field")
	}
	d.b = d.b[n:]
}

// ints appends a repeated int32 field, packed or not.
func (d *decoder) ints(wire int, dst []int) []int {
	if wire != wireBytes {
		return append(dst, d.int())
	}
	p := decoder{d.bytes()}
	for p.more() {
		dst = append(dst, p.int())
	}
	return dst
}

// find returns the last value of a length-delimited field in a message,
// without decoding the rest.
func find(msg []byte, field int) []byte {
	var found []byte
	d := decoder{msg}
	for d.more() {
		f, w := d.key()
		if f == field && w == wireBytes {
			found = d.bytes()
		} else {
			d.skip(w)
		}
	}
	return found
}
