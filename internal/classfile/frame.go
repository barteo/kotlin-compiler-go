package classfile

import (
	"fmt"
	"slices"
)

// Verification type tags, as in the StackMapTable attribute.
const (
	tTop        = 0
	tInt        = 1
	tFloat      = 2
	tDouble     = 3
	tLong       = 4
	tNull       = 5
	tUninitThis = 6
	tObject     = 7
	tUninit     = 8
)

// vtype is a verification type: the type of a local or stack value as the
// JVM's verifier sees it.
type vtype struct {
	tag  byte
	name string // tObject: internal name or array descriptor; tUninit: the class being created
	off  int    // tUninit: offset of the NEW instruction
}

var (
	intType    = vtype{tag: tInt}
	longType   = vtype{tag: tLong}
	floatType  = vtype{tag: tFloat}
	doubleType = vtype{tag: tDouble}
	nullType   = vtype{tag: tNull}
	stringType = object("java/lang/String")
)

func object(name string) vtype { return vtype{tag: tObject, name: name} }

// size is the number of stack or local slots a value of type t takes.
func (t vtype) size() int {
	if t.tag == tLong || t.tag == tDouble {
		return 2
	}
	return 1
}

func (t vtype) isRef() bool { return t.tag == tObject || t.tag == tNull }

func (t vtype) String() string {
	switch t.tag {
	case tInt:
		return "I"
	case tFloat:
		return "F"
	case tLong:
		return "J"
	case tDouble:
		return "D"
	case tNull:
		return "null"
	case tUninitThis:
		return "uninitialized this"
	case tObject:
		return t.name
	case tUninit:
		return fmt.Sprintf("uninitialized %s@%d", t.name, t.off)
	}
	return "top"
}

// typeOf returns the verification type of a field descriptor.
func typeOf(desc string) vtype {
	switch desc[0] {
	case 'Z', 'B', 'C', 'S', 'I':
		return intType
	case 'J':
		return longType
	case 'D':
		return doubleType
	case 'F':
		return floatType
	case 'L':
		return object(desc[1 : len(desc)-1])
	}
	return object(desc) // an array
}

// scanDesc returns the number of parameters of a method descriptor, their
// size in slots, and the return type's descriptor.
func scanDesc(desc string) (n, slots int, ret string) {
	i := 1
	for desc[i] != ')' {
		n++
		if desc[i] == 'J' || desc[i] == 'D' {
			slots += 2
		} else {
			slots++
		}
		for desc[i] == '[' {
			i++
		}
		if desc[i] == 'L' {
			for desc[i] != ';' {
				i++
			}
		}
		i++
	}
	return n, slots, desc[i+1:]
}

// paramTypes returns the verification types of a method's parameters.
func paramTypes(desc string) []vtype {
	var out []vtype
	for i := 1; desc[i] != ')'; i++ {
		start := i
		for desc[i] == '[' {
			i++
		}
		if desc[i] == 'L' {
			for desc[i] != ';' {
				i++
			}
		}
		out = append(out, typeOf(desc[start:i+1]))
	}
	return out
}

// bitset is a set of local slots.
type bitset []uint64

func (b bitset) has(i int) bool { return i/64 < len(b) && b[i/64]&(1<<(i%64)) != 0 }

func (b *bitset) set(i int) {
	for i/64 >= len(*b) {
		*b = append(*b, 0)
	}
	(*b)[i/64] |= 1 << (i % 64)
}

// and removes from b the slots not in o.
func (b bitset) and(o bitset) {
	for i := range b {
		if i < len(o) {
			b[i] &= o[i]
		} else {
			b[i] = 0
		}
	}
}

// covers reports whether b has every slot in o.
func (b bitset) covers(o bitset) bool {
	for i, w := range o {
		var have uint64
		if i < len(b) {
			have = b[i]
		}
		if w&^have != 0 {
			return false
		}
	}
	return true
}

// state is what the verifier knows at a point in the code: the operand
// stack, and which locals hold a value. A local that holds a value has
// its declared type.
type state struct {
	stack    []vtype
	assigned bitset
	// uninitThis is set in a constructor before the super constructor
	// call: this, in local 0, is still uninitialized.
	uninitThis bool
}

// frameLocals lists the locals of a frame: the declared type of each
// assigned local, Top for the others, without trailing Tops. Local 0 is
// UninitializedThis before a constructor calls its super constructor.
func (c *Code) frameLocals(assigned bitset, uninitThis bool) []vtype {
	var out []vtype
	n := 0
	for s := 0; s < len(c.locals); {
		t := c.locals[s]
		if uninitThis && (s == 0 || slices.Contains(c.thisCopies, s)) {
			t = vtype{tag: tUninitThis}
		}
		if assigned.has(s) && t.tag != tTop {
			out = append(out, t)
			n = len(out)
			s += t.size()
			continue
		}
		out = append(out, vtype{})
		s++
	}
	return out[:n]
}

func (c *Code) appendType(b []byte, t vtype) []byte {
	b = append(b, t.tag)
	switch t.tag {
	case tObject:
		b = u2(b, c.pool.Class(t.name))
	case tUninit:
		b = u2(b, uint16(t.off))
	}
	return b
}

// frame is an entry of the StackMapTable attribute.
type frame struct {
	pos           int
	locals, stack []vtype
}

// frameList returns a frame for each offset a jump targets. Where several
// labels share an offset, the last one placed has the state merged from
// all of them.
func (c *Code) frameList() []frame {
	var out []frame
	jumped := false
	for i, l := range c.placed {
		jumped = jumped || l.jumped
		if i+1 < len(c.placed) && c.placed[i+1].pos == l.pos {
			continue
		}
		if jumped {
			out = append(out, frame{l.pos, c.frameLocals(l.frame.assigned, l.frame.uninitThis), l.frame.stack})
		}
		jumped = false
	}
	return out
}

// encodeFrames builds the StackMapTable attribute's entries.
func (c *Code) encodeFrames() {
	var out []byte
	frames := c.frameList()
	prev, prevLocals := -1, c.entry
	for _, f := range frames {
		delta := f.pos - prev - 1
		same := slices.Equal(f.locals, prevLocals)
		switch {
		case same && len(f.stack) == 0 && delta < 64:
			out = append(out, byte(delta))
		case same && len(f.stack) == 0:
			out = u2(append(out, 251), uint16(delta))
		case same && len(f.stack) == 1 && delta < 64:
			out = c.appendType(append(out, byte(64+delta)), f.stack[0])
		case same && len(f.stack) == 1:
			out = c.appendType(u2(append(out, 247), uint16(delta)), f.stack[0])
		default:
			out = u2(append(out, 255), uint16(delta))
			out = u2(out, uint16(len(f.locals)))
			for _, t := range f.locals {
				out = c.appendType(out, t)
			}
			out = u2(out, uint16(len(f.stack)))
			for _, t := range f.stack {
				out = c.appendType(out, t)
			}
		}
		prev, prevLocals = f.pos, f.locals
	}
	c.frames = len(frames)
	c.smt = out
}
