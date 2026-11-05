// Package classfile writes JVM class files (version 52, Java 8, with
// StackMapTable frames) and reads the parts of them the compiler needs.
package classfile

import (
	"encoding/binary"
	"fmt"
	"math"
	"unicode/utf16"
)

const (
	AccPublic     = 0x0001
	AccPrivate    = 0x0002
	AccStatic     = 0x0008
	AccFinal      = 0x0010
	AccEnum       = 0x4000
	AccAnnotation = 0x2000
	AccSuper      = 0x0020
	AccVolatile   = 0x0040 // a field's; a method's is AccBridge
	AccTransient  = 0x0080 // a field's; a method's is AccVarargs
	// A method's: AccSynchronized has a class's AccSuper's value.
	AccSynchronized = 0x0020
	AccNative       = 0x0100 // a method's, without code
	AccStrict       = 0x0800

	majorVersion = 52 // Java 8
)

const (
	tagUtf8               = 1
	tagInteger            = 3
	tagFloat              = 4
	tagLong               = 5
	tagDouble             = 6
	tagClass              = 7
	tagString             = 8
	tagFieldref           = 9
	tagMethodref          = 10
	tagInterfaceMethodref = 11
	tagNameAndType        = 12
	tagMethodHandle       = 15
	tagMethodType         = 16
	tagDynamic            = 17
	tagInvokeDynamic      = 18
)

// RefInvokeStatic is the method handle kind of a static method.
const RefInvokeStatic = 6

type poolKey struct {
	tag  byte
	a, b string
	n    uint64
}

// Pool is a constant pool with deduplication. It also holds the class's
// bootstrap methods, which InvokeDynamic constants refer to by index.
type Pool struct {
	buf       []byte
	count     uint16 // next free index
	index     map[poolKey]uint16
	utf8      map[string]uint16 // most entries need only a string key
	bootstrap [][]uint16        // each a method handle and its arguments
	bsmIndex  map[string]uint16
}

func (p *Pool) add(k poolKey, entry []byte, slots uint16) uint16 {
	if p.index == nil {
		p.index = map[poolKey]uint16{}
	}
	if p.count == 0 {
		p.count = 1
	}
	if i, ok := p.index[k]; ok {
		return i
	}
	if int(p.count)+int(slots) > math.MaxUint16 {
		panic("constant pool overflow")
	}
	i := p.count
	p.buf = append(p.buf, entry...)
	p.count += slots
	p.index[k] = i
	return i
}

func u2(b []byte, v uint16) []byte { return binary.BigEndian.AppendUint16(b, v) }
func u4(b []byte, v uint32) []byte { return binary.BigEndian.AppendUint32(b, v) }

// modifiedUTF8 encodes s the way class files require: NUL as two bytes and
// supplementary characters as surrogate pairs.
func modifiedUTF8(s string) []byte {
	return appendModifiedUTF8(make([]byte, 0, len(s)), s)
}

func appendModifiedUTF8(out []byte, s string) []byte {
	ascii := true
	for i := range len(s) {
		if s[i] == 0 || s[i] >= 0x80 {
			ascii = false
			break
		}
	}
	if ascii {
		return append(out, s...)
	}
	for _, r := range s {
		switch {
		case r != 0 && r < 0x80:
			out = append(out, byte(r))
		case r < 0x800:
			out = append(out, byte(0xC0|r>>6), byte(0x80|r&0x3F))
		case r < 0x10000:
			out = append(out, byte(0xE0|r>>12), byte(0x80|(r>>6)&0x3F), byte(0x80|r&0x3F))
		default:
			hi, lo := utf16.EncodeRune(r)
			for _, c := range [2]rune{hi, lo} {
				out = append(out, byte(0xE0|c>>12), byte(0x80|(c>>6)&0x3F), byte(0x80|c&0x3F))
			}
		}
	}
	return out
}

func (p *Pool) Utf8(s string) uint16 {
	if i, ok := p.utf8[s]; ok {
		return i
	}
	// Encode directly into the destination pool, avoiding a temporary
	// byte slice for every newly encountered string.
	start := len(p.buf)
	p.buf = append(p.buf, tagUtf8, 0, 0)
	p.buf = appendModifiedUTF8(p.buf, s)
	size := len(p.buf) - start - 3
	if size > math.MaxUint16 {
		p.buf = p.buf[:start]
		panic("string constant too long")
	}
	if p.count == 0 {
		p.count = 1
	}
	if p.count == math.MaxUint16 {
		p.buf = p.buf[:start]
		panic("constant pool overflow")
	}
	if p.utf8 == nil {
		p.utf8 = map[string]uint16{}
	}
	i := p.count
	p.count++
	p.utf8[s] = i
	p.buf[start+1], p.buf[start+2] = byte(size>>8), byte(size)
	return i
}

func (p *Pool) Class(name string) uint16 {
	k := poolKey{tag: tagClass, a: name}
	if i, ok := p.index[k]; ok {
		return i
	}
	n := p.Utf8(name)
	return p.add(k, u2([]byte{tagClass}, n), 1)
}

func (p *Pool) String(s string) uint16 {
	k := poolKey{tag: tagString, a: s}
	if i, ok := p.index[k]; ok {
		return i
	}
	n := p.Utf8(s)
	return p.add(k, u2([]byte{tagString}, n), 1)
}

func (p *Pool) Int(v int32) uint16 {
	return p.add(poolKey{tag: tagInteger, n: uint64(uint32(v))}, u4([]byte{tagInteger}, uint32(v)), 1)
}

func (p *Pool) Float(v float32) uint16 {
	bits := math.Float32bits(v)
	return p.add(poolKey{tag: tagFloat, n: uint64(bits)}, u4([]byte{tagFloat}, bits), 1)
}

func (p *Pool) Long(v int64) uint16 {
	e := binary.BigEndian.AppendUint64([]byte{tagLong}, uint64(v))
	return p.add(poolKey{tag: tagLong, n: uint64(v)}, e, 2)
}

func (p *Pool) Double(v float64) uint16 {
	bits := math.Float64bits(v)
	e := binary.BigEndian.AppendUint64([]byte{tagDouble}, bits)
	return p.add(poolKey{tag: tagDouble, n: bits}, e, 2)
}

func (p *Pool) NameAndType(name, desc string) uint16 {
	k := poolKey{tag: tagNameAndType, a: name, b: desc}
	if i, ok := p.index[k]; ok {
		return i
	}
	n, d := p.Utf8(name), p.Utf8(desc)
	return p.add(k, u2(u2([]byte{tagNameAndType}, n), d), 1)
}

func (p *Pool) Field(owner, name, desc string) uint16 {
	k := poolKey{tag: tagFieldref, a: owner, b: name + ":" + desc}
	if i, ok := p.index[k]; ok {
		return i
	}
	c, nt := p.Class(owner), p.NameAndType(name, desc)
	return p.add(k, u2(u2([]byte{tagFieldref}, c), nt), 1)
}

func (p *Pool) Method(owner, name, desc string) uint16 {
	k := poolKey{tag: tagMethodref, a: owner, b: name + desc}
	if i, ok := p.index[k]; ok {
		return i
	}
	c, nt := p.Class(owner), p.NameAndType(name, desc)
	return p.add(k, u2(u2([]byte{tagMethodref}, c), nt), 1)
}

func (p *Pool) InterfaceMethod(owner, name, desc string) uint16 {
	k := poolKey{tag: tagInterfaceMethodref, a: owner, b: name + desc}
	if i, ok := p.index[k]; ok {
		return i
	}
	c, nt := p.Class(owner), p.NameAndType(name, desc)
	return p.add(k, u2(u2([]byte{tagInterfaceMethodref}, c), nt), 1)
}

// MethodHandle returns a method handle constant of the given kind for a
// Methodref or InterfaceMethodref.
func (p *Pool) MethodHandle(kind byte, ref uint16) uint16 {
	return p.add(poolKey{tag: tagMethodHandle, n: uint64(kind)<<16 | uint64(ref)}, u2([]byte{tagMethodHandle, kind}, ref), 1)
}

func (p *Pool) MethodType(desc string) uint16 {
	d := p.Utf8(desc)
	return p.add(poolKey{tag: tagMethodType, a: desc}, u2([]byte{tagMethodType}, d), 1)
}

// Bootstrap adds a bootstrap method, a method handle and its static
// arguments (constants), and returns its index.
func (p *Pool) Bootstrap(handle uint16, args ...uint16) uint16 {
	entry := append([]uint16{handle}, args...)
	key := fmt.Sprint(entry)
	if i, ok := p.bsmIndex[key]; ok {
		return i
	}
	if p.bsmIndex == nil {
		p.bsmIndex = map[string]uint16{}
	}
	i := uint16(len(p.bootstrap))
	p.bootstrap = append(p.bootstrap, entry)
	p.bsmIndex[key] = i
	return i
}

func (p *Pool) InvokeDynamic(bsm uint16, name, desc string) uint16 {
	nt := p.NameAndType(name, desc)
	return p.add(poolKey{tag: tagInvokeDynamic, a: name, b: desc, n: uint64(bsm)}, u2(u2([]byte{tagInvokeDynamic}, bsm), nt), 1)
}

type member struct {
	access, name, desc uint16
	nameStr            string // the name, for AddDelegates and HasMethod
	descriptor         string
	excNames           []string // the Exceptions, by internal name
	code               *Code
	constant           uint16   // a field's ConstantValue, or 0
	exceptions         []uint16 // a method's Exceptions
	anns               annotations
	params             []annotations // the parameters', by index
	synthetic          int           // leading parameters without annotations: see SyntheticParams
	deflt              *ElementValue // an annotation method's AnnotationDefault
	deprecated         bool          // the Deprecated attribute
	signature          string        // the Signature attribute, if any
}

// annotations are the annotations of a class, member or parameter: those
// visible to reflection and those that aren't.
type annotations struct{ visible, invisible []Annotation }

func (a *annotations) add(an Annotation, visible bool) {
	if visible {
		a.visible = append(a.visible, an)
	} else {
		a.invisible = append(a.invisible, an)
	}
}

// Class is a class file being built.
type Class struct {
	Pool
	Access     uint16
	Name       string
	Super      string
	SourceFile string
	Interfaces []string
	// InnerClasses are the entries of the InnerClasses attribute: the
	// class itself if it is nested, and the nested classes it declares
	// or refers to.
	InnerClasses []InnerClassEntry
	// Enclosing is set on a local or anonymous class declared in a
	// method: the EnclosingMethod attribute's class, method name and
	// descriptor.
	Enclosing *EnclosingMethod
	// Line is the line of the class's declaration, which methods the
	// compiler generates for it, without lines of their own, get.
	Line int
	// Signature is the class's generic signature, its Signature
	// attribute, if it has one.
	Signature string
	fields    []*member
	methods   []*member
	anns      annotations
}

// EnclosingMethod is the method a local or anonymous class is declared
// in: Name and Desc are empty if it is not in a method.
type EnclosingMethod struct {
	Class, Name, Desc string
}

// InnerClassEntry is an entry of the InnerClasses attribute: a nested
// class, the class it is declared in and its simple name, which are
// empty for an anonymous class, and its access flags as declared.
type InnerClassEntry struct {
	Inner, Outer, Simple string
	Access               uint16
}

func NewClass(access uint16, name, super string) *Class {
	return &Class{Access: access, Name: name, Super: super}
}

func (c *Class) AddField(access uint16, name, desc string) {
	c.fields = append(c.fields, &member{access: access, name: c.Utf8(name), desc: c.Utf8(desc), descriptor: desc})
}

// AddConstField adds a static final field whose ConstantValue is value,
// an Integer, Long, Float, Double or String constant of the pool.
func (c *Class) AddConstField(access uint16, name, desc string, value uint16) {
	c.fields = append(c.fields, &member{access: access, name: c.Utf8(name), desc: c.Utf8(desc), descriptor: desc, constant: value})
}

// NewMethod adds a method and returns its code buffer.
func (c *Class) NewMethod(access uint16, name, desc string) *Code {
	code := newCode(&c.Pool, c.Name, access, name, desc)
	c.methods = append(c.methods, &member{access: access, name: c.Utf8(name), nameStr: name, desc: c.Utf8(desc), descriptor: desc, code: code})
	return code
}

// AddAbstractMethod adds a method without code.
func (c *Class) AddAbstractMethod(access uint16, name, desc string) {
	c.methods = append(c.methods, &member{access: access, name: c.Utf8(name), nameStr: name, desc: c.Utf8(desc), descriptor: desc})
}

// HasMethod reports whether the class has a method with a name and
// descriptor.
func (c *Class) HasMethod(name, desc string) bool {
	for _, m := range c.methods {
		if m.nameStr == name && m.descriptor == desc {
			return true
		}
	}
	return false
}

// NumMethods returns the number of methods the class has so far.
func (c *Class) NumMethods() int { return len(c.methods) }

// Throws gives the method added last an Exceptions attribute: the
// checked exceptions it declares, by internal name.
func (c *Class) Throws(classes ...string) {
	m := c.methods[len(c.methods)-1]
	for _, e := range classes {
		m.exceptions = append(m.exceptions, c.Class(e))
	}
	m.excNames = append(m.excNames, classes...)
}

// MemberRef is a field or method of a class being built, to give attributes
// once it is added: annotations, which codegen writes after the code,
// which may add other methods.
type MemberRef struct{ m *member }

// LastMethod returns the method added last.
func (c *Class) LastMethod() MemberRef { return MemberRef{c.methods[len(c.methods)-1]} }

// LastField returns the field added last.
func (c *Class) LastField() MemberRef { return MemberRef{c.fields[len(c.fields)-1]} }

// Annotate adds an annotation to the class: a RuntimeVisibleAnnotations
// one if visible, else a RuntimeInvisibleAnnotations one.
func (c *Class) Annotate(a Annotation, visible bool) { c.anns.add(a, visible) }

// Annotate adds an annotation to the member.
func (m MemberRef) Annotate(a Annotation, visible bool) { m.m.anns.add(a, visible) }

// AnnotateParam adds an annotation to the method's parameter i, counted
// in its descriptor.
func (m MemberRef) AnnotateParam(i int, a Annotation, visible bool) {
	for len(m.m.params) <= i {
		m.m.params = append(m.m.params, annotations{})
	}
	m.m.params[i].add(a, visible)
}

// SyntheticParams marks the method's first n parameters as synthetic,
// as an inner class constructor's outer instance or an enum
// constructor's name and ordinal: as with kotlinc and javac, its
// parameter annotations attributes leave them out.
func (m MemberRef) SyntheticParams(n int) { m.m.synthetic = n }

// SetDefault gives an annotation class's method its default value.
func (m MemberRef) SetDefault(v ElementValue) { m.m.deflt = &v }

// Deprecate gives the member the Deprecated attribute.
// AddAccess adds access flags to the member.
func (m MemberRef) AddAccess(flags uint16) { m.m.access |= flags }

func (m MemberRef) Deprecate() { m.m.deprecated = true }

// SetSignature gives the member a generic signature, its Signature
// attribute.
func (m MemberRef) SetSignature(sig string) { m.m.signature = sig }

// MethodSignature returns the generic signature of the class's method of
// a name and descriptor, if it has one.
func (c *Class) MethodSignature(name, desc string) string {
	for _, m := range c.methods {
		if m.nameStr == name && m.descriptor == desc {
			return m.signature
		}
	}
	return ""
}

// attr is an attribute: its name's constant and its contents.
type attr struct {
	name uint16
	data []byte
}

func appendAttrs(out []byte, attrs []attr) []byte {
	out = u2(out, uint16(len(attrs)))
	for _, a := range attrs {
		out = u2(out, a.name)
		out = u4(out, uint32(len(a.data)))
		out = append(out, a.data...)
	}
	return out
}

// annotationAttrs returns the attributes of annotations as.
func (c *Class) annotationAttrs(attrs []attr, as annotations) []attr {
	enc := func(name string, list []Annotation) {
		if len(list) == 0 {
			return
		}
		b := u2(nil, uint16(len(list)))
		for _, a := range list {
			b = c.annotation(b, a)
		}
		attrs = append(attrs, attr{c.Utf8(name), b})
	}
	enc("RuntimeVisibleAnnotations", as.visible)
	enc("RuntimeInvisibleAnnotations", as.invisible)
	return attrs
}

// memberAttrs returns the annotation attributes of a member.
func (c *Class) memberAttrs(attrs []attr, m *member) []attr {
	if m.deprecated {
		attrs = append(attrs, attr{c.Utf8("Deprecated"), nil})
	}
	if m.signature != "" {
		attrs = append(attrs, attr{c.Utf8("Signature"), u2(nil, c.Utf8(m.signature))})
	}
	attrs = c.annotationAttrs(attrs, m.anns)
	if len(m.params) > 0 {
		k := min(m.synthetic, len(m.params))
		n := len(paramTypes(m.descriptor)) - m.synthetic
		params := func(name string, visible bool) {
			any := false
			for _, p := range m.params[k:] {
				if visible && len(p.visible) > 0 || !visible && len(p.invisible) > 0 {
					any = true
				}
			}
			if !any {
				return
			}
			b := []byte{byte(n)}
			for i := m.synthetic; i < m.synthetic+n; i++ {
				var list []Annotation
				if i < len(m.params) {
					list = m.params[i].invisible
					if visible {
						list = m.params[i].visible
					}
				}
				b = u2(b, uint16(len(list)))
				for _, a := range list {
					b = c.annotation(b, a)
				}
			}
			attrs = append(attrs, attr{c.Utf8(name), b})
		}
		params("RuntimeVisibleParameterAnnotations", true)
		params("RuntimeInvisibleParameterAnnotations", false)
	}
	if m.deflt != nil {
		attrs = append(attrs, attr{c.Utf8("AnnotationDefault"), c.elementValue(nil, *m.deflt)})
	}
	return attrs
}

// Bytes serializes the class.
func (c *Class) Bytes() []byte {
	this, super := c.Class(c.Name), c.Class(c.Super)
	ifaces := make([]uint16, len(c.Interfaces))
	for i, name := range c.Interfaces {
		ifaces[i] = c.Class(name)
	}
	// The attributes add constants, so they are all encoded before the
	// pool is written.
	fieldAttrs := make([][]attr, len(c.fields))
	for i, f := range c.fields {
		var attrs []attr
		if f.constant != 0 {
			attrs = append(attrs, attr{c.Utf8("ConstantValue"), u2(nil, f.constant)})
		}
		fieldAttrs[i] = c.memberAttrs(attrs, f)
	}
	methodAttrs := make([][]attr, len(c.methods))
	for i, m := range c.methods {
		var attrs []attr
		if m.code != nil {
			attrs = append(attrs, attr{c.Utf8("Code"), c.codeAttr(m)})
		}
		if len(m.exceptions) > 0 {
			b := u2(nil, uint16(len(m.exceptions)))
			for _, e := range m.exceptions {
				b = u2(b, e)
			}
			attrs = append(attrs, attr{c.Utf8("Exceptions"), b})
		}
		methodAttrs[i] = c.memberAttrs(attrs, m)
	}
	var classAttrs []attr
	if c.SourceFile != "" {
		classAttrs = append(classAttrs, attr{c.Utf8("SourceFile"), u2(nil, c.Utf8(c.SourceFile))})
	}
	if len(c.InnerClasses) > 0 {
		// A class is listed once, which the JVM requires: the code
		// generator may add it from both the class and its outer class.
		var entries []InnerClassEntry
		seen := map[string]bool{}
		for _, e := range c.InnerClasses {
			if !seen[e.Inner] {
				seen[e.Inner] = true
				entries = append(entries, e)
			}
		}
		b := u2(nil, uint16(len(entries)))
		for _, e := range entries {
			var outer, simple uint16
			if e.Outer != "" {
				outer = c.Class(e.Outer)
			}
			if e.Simple != "" {
				simple = c.Utf8(e.Simple)
			}
			b = u2(u2(u2(u2(b, c.Class(e.Inner)), outer), simple), e.Access)
		}
		classAttrs = append(classAttrs, attr{c.Utf8("InnerClasses"), b})
	}
	if c.Signature != "" {
		classAttrs = append(classAttrs, attr{c.Utf8("Signature"), u2(nil, c.Utf8(c.Signature))})
	}
	if e := c.Enclosing; e != nil {
		var m uint16
		if e.Name != "" {
			m = c.NameAndType(e.Name, e.Desc)
		}
		classAttrs = append(classAttrs, attr{c.Utf8("EnclosingMethod"), u2(u2(nil, c.Class(e.Class)), m)})
	}
	classAttrs = c.annotationAttrs(classAttrs, c.anns)
	if len(c.bootstrap) > 0 {
		// After the code, which adds bootstrap methods.
		b := u2(nil, uint16(len(c.bootstrap)))
		for _, m := range c.bootstrap {
			b = u2(b, m[0])
			b = u2(b, uint16(len(m)-1))
			for _, a := range m[1:] {
				b = u2(b, a)
			}
		}
		classAttrs = append(classAttrs, attr{c.Utf8("BootstrapMethods"), b})
	}

	out := u4(nil, 0xCAFEBABE)
	out = u2(out, 0)
	out = u2(out, majorVersion)
	out = u2(out, c.count)
	out = append(out, c.buf...)
	out = u2(out, c.Access)
	out = u2(out, this)
	out = u2(out, super)
	out = u2(out, uint16(len(ifaces)))
	for _, i := range ifaces {
		out = u2(out, i)
	}
	out = u2(out, uint16(len(c.fields)))
	for i, f := range c.fields {
		out = u2(u2(u2(out, f.access), f.name), f.desc)
		out = appendAttrs(out, fieldAttrs[i])
	}
	out = u2(out, uint16(len(c.methods)))
	for i, m := range c.methods {
		out = u2(u2(u2(out, m.access), m.name), m.desc)
		out = appendAttrs(out, methodAttrs[i])
	}
	return appendAttrs(out, classAttrs)
}

// codeAttr finishes a method's code and returns its Code attribute's
// contents, with its frames and debug information.
func (c *Class) codeAttr(m *member) []byte {
	code := m.code
	if err := code.finish(); err != nil {
		panic(err)
	}
	if c.Line > 0 && len(code.lines) == 0 && len(code.buf) > 0 && m.access&(AccSynthetic|AccBridge) == 0 {
		code.lines = []lineEntry{{0, c.Line}}
	}
	code.encodeFrames()
	var attrs []attr
	if code.frames > 0 {
		attrs = append(attrs, attr{c.Utf8("StackMapTable"), append(u2(nil, uint16(code.frames)), code.smt...)})
	}
	lines, vars, nlines, nvars := code.debugAttrs()
	if nlines > 0 {
		attrs = append(attrs, attr{c.Utf8("LineNumberTable"), append(u2(nil, uint16(nlines)), lines...)})
	}
	if nvars > 0 {
		attrs = append(attrs, attr{c.Utf8("LocalVariableTable"), append(u2(nil, uint16(nvars)), vars...)})
	}
	b := u2(nil, uint16(code.max))
	b = u2(b, uint16(code.maxLocals))
	b = u4(b, uint32(len(code.buf)))
	b = append(b, code.buf...)
	b = u2(b, uint16(len(code.handlers)))
	for _, h := range code.handlers {
		b = u2(u2(u2(u2(b, uint16(h.start)), uint16(h.end)), uint16(h.handler.pos)), h.catchType)
	}
	return appendAttrs(b, attrs)
}
