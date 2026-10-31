package classfile

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"sync"
)

// Method is the code of a method read from a class file, for the
// inliner: its instructions, exception handlers and stack map frames, and
// the constant pool they refer to.
type Method struct {
	Class     string // internal name of the class it is in
	Facade    string // the facade of a multifile part (@kotlin.Metadata xs), or ""
	Access    uint16
	Name      string
	Desc      string
	MaxStack  int
	MaxLocals int
	Code      []byte
	CodeOff   int // the offset of Code in the class file
	Handlers  []Handler
	// Frames are the StackMapTable's frames, by offset. A method without
	// branches has none.
	Frames map[int]*Frame
	// Lines is the LineNumberTable: pairs of a code offset and a line.
	Lines [][2]int
	pool  *pool
}

// Handler is an entry of a method's exception table: exceptions of class
// Catch ("" for any) thrown in [Start, End) go to Handler.
type Handler struct {
	Start, End, Handler int
	Catch               string
}

// Frame is a stack map frame: the verification types of the locals, by
// slot (Top for the second slot of a long or double and for unused
// slots), and of the values on the operand stack, one entry per value.
type Frame struct {
	Locals []VType
	Stack  []VType
}

// VType is a verification type, as frames list them.
type VType = vtype

// Verification type tags.
const (
	VTop        = tTop
	VInt        = tInt
	VFloat      = tFloat
	VDouble     = tDouble
	VLong       = tLong
	VNull       = tNull
	VUninitThis = tUninitThis
	VObject     = tObject
	VUninit     = tUninit
)

// Tag returns the type's tag: VTop, VInt, ... VUninit.
func (t vtype) Tag() byte { return t.tag }

// Class returns the internal name or array descriptor of a VObject type.
func (t vtype) Class() string { return t.name }

// Off returns the offset of the new instruction of a VUninit type.
func (t vtype) Off() int { return t.off }

// Size is the number of slots a value of the type takes.
func (t vtype) Size() int { return t.size() }

// ObjectType returns the verification type of instances of a class (an
// internal name, or an array descriptor).
func ObjectType(name string) VType { return object(name) }

// TypeOf returns the verification type of a field descriptor.
func TypeOf(desc string) VType { return typeOf(desc) }

// UninitType returns the type of an object of a class created by the new
// instruction at offset off, before its constructor is called.
func UninitType(class string, off int) VType { return vtype{tag: tUninit, name: class, off: off} }

// NullType is the type of null.
var NullType VType = nullType

var errNotFound = errors.New("method not found")

// ReadMethod reads the code of the method with a name and descriptor
// from a class file.
func ReadMethod(data []byte, name, desc string) (*Method, error) {
	r, err := NewMethodReader(data)
	if err != nil {
		return nil, err
	}
	return r.ReadMethod(name, desc)
}

// ReadMethods reads all the methods with code of a class file.
func ReadMethods(data []byte) ([]*Method, error) {
	r, err := NewMethodReader(data)
	if err != nil {
		return nil, err
	}
	var out []*Method
	for _, entry := range r.order {
		m, err := r.ReadMethod(entry.name, entry.desc)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
}

// MethodReader shares an indexed constant pool and lazily decoded method
// bodies. It is safe for concurrent use; callers must not modify results.
type MethodReader struct {
	data          []byte
	pool          *pool
	class, facade string
	methods       map[methodKey]*methodEntry
	order         []*methodEntry
}

type methodKey struct{ name, desc string }

type methodEntry struct {
	name, desc string
	access     uint16
	start, end int
	once       sync.Once
	method     *Method
	err        error
}

func (mr *MethodReader) ReadMethod(name, desc string) (*Method, error) {
	e := mr.methods[methodKey{name, desc}]
	if e == nil {
		return nil, fmt.Errorf("%s%s: %w", name, desc, errNotFound)
	}
	e.once.Do(func() {
		m := &Method{Class: mr.class, Facade: mr.facade, Access: e.access, Name: name, Desc: desc, pool: mr.pool}
		r := &reader{b: mr.data[:e.end], pos: e.start}
		if err := m.readCode(r, mr.pool); err != nil {
			e.err = fmt.Errorf("%s.%s%s: %w", mr.class, name, desc, err)
			return
		}
		e.method = m
	})
	return e.method, e.err
}

// NewMethodReader indexes the code attributes, leaving unrelated bodies
// undecoded. Class attributes (facade and bootstrap methods) are shared.
func NewMethodReader(data []byte) (*MethodReader, error) {
	r := &reader{b: data}
	p, err := readPool(r)
	if err != nil {
		return nil, err
	}
	var errs []error
	must := func(s string, e error) string {
		if e != nil {
			errs = append(errs, e)
		}
		return s
	}
	r.skip(2) // access
	class := must(p.class(r.u2()))
	r.skip(2) // super
	r.skip(2 * int(r.u2()))
	for range r.u2() { // fields
		r.skip(6)
		for range r.u2() {
			r.skip(2)
			r.skip(int(r.u4()))
		}
	}
	mr := &MethodReader{data: data, pool: p, class: class, methods: map[methodKey]*methodEntry{}}
	for range r.u2() {
		access := r.u2()
		name := must(p.utf8(r.u2()))
		desc := must(p.utf8(r.u2()))
		for range r.u2() {
			attr := must(p.utf8(r.u2()))
			size := int(r.u4())
			if attr == "Code" {
				e := &methodEntry{name: name, desc: desc, access: access, start: r.pos, end: r.pos + size}
				key := methodKey{name, desc}
				if mr.methods[key] != nil {
					return nil, fmt.Errorf("%s.%s%s: duplicate method code", class, name, desc)
				}
				mr.methods[key] = e
				mr.order = append(mr.order, e)
			}
			r.skip(size)
		}
	}
	if len(errs) > 0 {
		return nil, errs[0]
	}
	// The class's attributes: @kotlin.Metadata names a part's facade.
	for range r.u2() {
		name := must(p.utf8(r.u2()))
		size := int(r.u4())
		if name == "BootstrapMethods" {
			// For the invokedynamic instructions of the methods.
			end := r.pos + size
			for range r.u2() {
				bsm := []uint16{r.u2()}
				for range r.u2() {
					bsm = append(bsm, r.u2())
				}
				p.bootstrap = append(p.bootstrap, bsm)
			}
			size = end - r.pos
		}
		if name == "RuntimeVisibleAnnotations" {
			end := r.pos + size
			k, err := kotlinMeta(r, p, &Info{})
			if err != nil {
				return nil, err
			}
			if k != nil && k.K == 5 {
				mr.facade = k.XS
			}
			size = end - r.pos
		}
		r.skip(size)
	}
	if r.err != nil {
		return nil, r.err
	}
	if len(errs) > 0 {
		return nil, errs[0]
	}
	return mr, nil
}

func (m *Method) readCode(r *reader, p *pool) error {
	m.MaxStack, m.MaxLocals = int(r.u2()), int(r.u2())
	n := int(r.u4())
	if r.pos+n > len(r.b) {
		return errTruncated
	}
	m.Code, m.CodeOff = r.b[r.pos:r.pos+n], r.pos
	r.pos += n
	for range r.u2() {
		h := Handler{Start: int(r.u2()), End: int(r.u2()), Handler: int(r.u2())}
		if ct := r.u2(); ct != 0 {
			var err error
			if h.Catch, err = p.class(ct); err != nil {
				return err
			}
		}
		m.Handlers = append(m.Handlers, h)
	}
	for range r.u2() {
		name, err := p.utf8(r.u2())
		if err != nil {
			return err
		}
		size := int(r.u4())
		if name == "LineNumberTable" {
			for range r.u2() {
				pc := int(r.u2())
				m.Lines = append(m.Lines, [2]int{pc, int(r.u2())})
			}
			continue
		}
		if name != "StackMapTable" {
			r.skip(size)
			continue
		}
		end := r.pos + size
		if err := m.readFrames(r, p); err != nil {
			return fmt.Errorf("StackMapTable: %w", err)
		}
		r.pos = end
	}
	return r.err
}

// readFrames decodes the StackMapTable attribute: each frame is a delta
// from the previous one, starting from the locals the descriptor gives.
func (m *Method) readFrames(r *reader, p *pool) error {
	var locals []vtype // one entry per local, as in frames: a long takes one
	if m.Access&AccStatic == 0 {
		if m.Name == "<init>" {
			locals = append(locals, vtype{tag: tUninitThis})
		} else {
			locals = append(locals, object(m.Class))
		}
	}
	locals = append(locals, paramTypes(m.Desc)...)
	m.Frames = map[int]*Frame{}
	vt := func() (vtype, error) {
		t := vtype{tag: r.u1()}
		switch t.tag {
		case tTop, tInt, tFloat, tDouble, tLong, tNull, tUninitThis:
		case tObject:
			var err error
			if t.name, err = p.class(r.u2()); err != nil {
				return t, err
			}
		case tUninit:
			t.off = int(r.u2())
		default:
			return t, fmt.Errorf("unknown verification type %d", t.tag)
		}
		return t, nil
	}
	types := func(n int) ([]vtype, error) {
		out := make([]vtype, n)
		for i := range out {
			var err error
			if out[i], err = vt(); err != nil {
				return nil, err
			}
		}
		return out, nil
	}
	off := -1
	for range r.u2() {
		kind := r.u1()
		var stack []vtype
		var delta int
		var err error
		switch {
		case kind < 64:
			delta = int(kind)
		case kind < 128:
			delta = int(kind) - 64
			stack, err = types(1)
		case kind == 247:
			delta = int(r.u2())
			stack, err = types(1)
		case kind >= 248 && kind <= 250:
			delta = int(r.u2())
			k := 251 - int(kind)
			if k > len(locals) {
				return fmt.Errorf("chop frame removes %d of %d locals", k, len(locals))
			}
			locals = locals[:len(locals)-k]
		case kind == 251:
			delta = int(r.u2())
		case kind >= 252 && kind <= 254:
			delta = int(r.u2())
			var add []vtype
			add, err = types(int(kind) - 251)
			locals = append(locals[:len(locals):len(locals)], add...)
		case kind == 255:
			delta = int(r.u2())
			if locals, err = types(int(r.u2())); err == nil {
				stack, err = types(int(r.u2()))
			}
		default:
			return fmt.Errorf("unknown frame type %d", kind)
		}
		if err != nil {
			return err
		}
		off += delta + 1
		f := &Frame{Stack: stack}
		for _, t := range locals {
			f.Locals = append(f.Locals, t)
			if t.size() == 2 {
				f.Locals = append(f.Locals, vtype{})
			}
		}
		m.Frames[off] = f
	}
	return r.err
}

// Constants of the constant pool, for the instruction decoder.

func (p *pool) entry(i uint16, tag byte) (int, error) {
	if int(i) >= len(p.tags) || p.tags[i] != tag {
		return 0, fmt.Errorf("constant %d is not of type %d", i, tag)
	}
	return p.offs[i], nil
}

func (p *pool) u2At(off int) uint16 { return binary.BigEndian.Uint16(p.b[off:]) }

// ref reads a Fieldref, Methodref or InterfaceMethodref.
func (p *pool) ref(i uint16) (owner, name, desc string, iface bool, err error) {
	if int(i) >= len(p.tags) {
		return "", "", "", false, fmt.Errorf("constant %d out of range", i)
	}
	switch p.tags[i] {
	case tagFieldref, tagMethodref:
	case tagInterfaceMethodref:
		iface = true
	default:
		return "", "", "", false, fmt.Errorf("constant %d is not a member reference", i)
	}
	off := p.offs[i]
	if owner, err = p.class(p.u2At(off)); err != nil {
		return
	}
	nt, err := p.entry(p.u2At(off+2), tagNameAndType)
	if err != nil {
		return
	}
	if name, err = p.utf8(p.u2At(nt)); err != nil {
		return
	}
	desc, err = p.utf8(p.u2At(nt + 2))
	return
}

// invokeDynamic reads an InvokeDynamic constant: the call site's name and
// descriptor, and its bootstrap method.
func (p *pool) invokeDynamic(i uint16) (name, desc string, bsm *Bootstrap, err error) {
	off, err := p.entry(i, tagInvokeDynamic)
	if err != nil {
		return
	}
	nt, err := p.entry(p.u2At(off+2), tagNameAndType)
	if err != nil {
		return
	}
	if name, err = p.utf8(p.u2At(nt)); err != nil {
		return
	}
	if desc, err = p.utf8(p.u2At(nt + 2)); err != nil {
		return
	}
	k := int(p.u2At(off))
	if k >= len(p.bootstrap) {
		return "", "", nil, fmt.Errorf("bootstrap method %d out of range", k)
	}
	entry := p.bootstrap[k]
	h, err := p.entry(entry[0], tagMethodHandle)
	if err != nil {
		return
	}
	bsm = &Bootstrap{Kind: p.b[h]}
	if bsm.Owner, bsm.Name, bsm.Desc, bsm.Iface, err = p.ref(p.u2At(h + 1)); err != nil {
		return
	}
	for _, a := range entry[1:] {
		var c Const
		if c.Kind, c.Int, c.Float, c.Str, err = p.constant(a); err != nil {
			return
		}
		bsm.Args = append(bsm.Args, c)
	}
	return
}

// Bootstrap is the bootstrap method of an invokedynamic instruction: a
// method handle of kind Kind (RefInvokeStatic and such) to a method, and
// the static arguments.
type Bootstrap struct {
	Kind              byte
	Owner, Name, Desc string
	Iface             bool
	Args              []Const
}

// Const is a loadable constant: an int or long's value in Int, a float or
// double's in Float, a string's or class's in Str.
type Const struct {
	Kind  ConstKind
	Int   int64
	Float float64
	Str   string
}

// ConstKind is what an ldc instruction loads.
type ConstKind byte

const (
	ConstInt ConstKind = iota + 1
	ConstFloat
	ConstLong
	ConstDouble
	ConstString
	ConstClass
	ConstOther // a method handle, method type or dynamic constant
)

// constant reads a loadable constant.
func (p *pool) constant(i uint16) (kind ConstKind, n int64, f float64, s string, err error) {
	if int(i) >= len(p.tags) || i == 0 {
		return 0, 0, 0, "", fmt.Errorf("constant %d out of range", i)
	}
	off := p.offs[i]
	switch p.tags[i] {
	case tagInteger:
		return ConstInt, int64(int32(binary.BigEndian.Uint32(p.b[off:]))), 0, "", nil
	case tagFloat:
		return ConstFloat, 0, float64(math.Float32frombits(binary.BigEndian.Uint32(p.b[off:]))), "", nil
	case tagLong:
		return ConstLong, int64(binary.BigEndian.Uint64(p.b[off:])), 0, "", nil
	case tagDouble:
		return ConstDouble, 0, math.Float64frombits(binary.BigEndian.Uint64(p.b[off:])), "", nil
	case tagString:
		s, err = p.utf8(p.u2At(off))
		return ConstString, 0, 0, s, err
	case tagClass:
		s, err = p.class(i)
		return ConstClass, 0, 0, s, err
	case tagMethodHandle, tagMethodType, tagDynamic:
		return ConstOther, 0, 0, "", nil
	}
	return 0, 0, 0, "", fmt.Errorf("constant %d is not loadable", i)
}
