package classfile

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf16"
)

// Access flags read from class files, in addition to those used for writing.
const (
	AccProtected = 0x0004
	AccBridge    = 0x0040
	AccVarargs   = 0x0080
	AccInterface = 0x0200
	AccAbstract  = 0x0400
	AccSynthetic = 0x1000
)

// Info is the part of a class file the compiler needs to call into the
// class: its supertypes and the signatures of its fields and methods.
// Method bodies and most attributes are skipped.
type Info struct {
	Access     uint16
	Name       string // internal name, e.g. java/lang/StringBuilder
	Super      string // "" for java/lang/Object and module-info
	Interfaces []string
	Sig        string // generic signature, or "" if the class has none
	Fields     []Member
	Methods    []Member
	Kotlin     *KotlinMeta // the @kotlin.Metadata annotation, or nil
	// Retention is the value of an annotation class's
	// @java.lang.annotation.Retention: RUNTIME, CLASS or SOURCE, or ""
	// without one (CLASS).
	Retention string
	// Repeatable is the container class of a repeatable annotation
	// class, the value of its @java.lang.annotation.Repeatable, or "".
	Repeatable string
	// KotlinTargets are the values of @kotlin.annotation.Target
	// (AnnotationTarget names), and JavaTargets those of
	// @java.lang.annotation.Target (ElementType names).
	KotlinTargets, JavaTargets []string
	// Inner is the class's own entry of its InnerClasses attribute, if
	// it is a nested class: its outer class and its access as declared.
	Inner *InnerClass
	// Invisible are the descriptors of the class's annotations that are
	// kept in its class file only, such as @RestrictsSuspension.
	Invisible []string
	// StabilityInferred is the parameters argument of the class's
	// @androidx.compose.runtime.internal.StabilityInferred, which
	// Compose's compiler plugin writes, or -1 without one.
	StabilityInferred int
	// DebugMetadata holds the arguments of the @DebugMetadata of a
	// coroutine's continuation class, as strings, by name.
	DebugMetadata map[string][]string
}

// InnerClass is an entry of the InnerClasses attribute.
type InnerClass struct {
	Outer  string // "" for a local or anonymous class
	Access uint16
}

// KotlinMeta holds the arguments of a class's @kotlin.Metadata annotation,
// which describes the class's Kotlin declarations. Package kmeta decodes D1
// and D2.
type KotlinMeta struct {
	K  int      // kind: 1 class, 2 file facade, 3 synthetic class, 4 multi-file facade, 5 multi-file part
	MV []int    // metadata version
	D1 []string // protobuf message, encoded as strings
	D2 []string // strings the message refers to
	XS string   // multi-file part: the facade's internal name
	PN string   // package name, if it differs from the JVM package
	XI int      // extra flags
}

// Member is a field or method.
type Member struct {
	Access uint16
	Name   string
	Desc   string
	Sig    string // generic signature, or "" if it uses no generic types
	// LowPriority is set on a method annotated
	// @kotlin.internal.LowPriorityInOverloadResolution, which calls
	// choose only if no other function fits.
	LowPriority bool
	// Hidden is set on a method annotated @Deprecated(level = HIDDEN) or
	// @DeprecatedSinceKotlin(hiddenSince = ...), which calls can't choose.
	Hidden bool
	// Constant is a field's ConstantValue: an int32, int64, float32,
	// float64 or string, or nil.
	Constant any
}

var errTruncated = errors.New("truncated class file")

type reader struct {
	b   []byte
	pos int
	err error
}

func (r *reader) u1() byte {
	if r.pos+1 > len(r.b) {
		r.fail()
		return 0
	}
	v := r.b[r.pos]
	r.pos++
	return v
}

func (r *reader) u2() uint16 {
	if r.pos+2 > len(r.b) {
		r.fail()
		return 0
	}
	v := binary.BigEndian.Uint16(r.b[r.pos:])
	r.pos += 2
	return v
}

func (r *reader) u4() uint32 {
	if r.pos+4 > len(r.b) {
		r.fail()
		return 0
	}
	v := binary.BigEndian.Uint32(r.b[r.pos:])
	r.pos += 4
	return v
}

func (r *reader) skip(n int) {
	if n < 0 || r.pos+n > len(r.b) {
		r.fail()
		return
	}
	r.pos += n
}

func (r *reader) fail() {
	if r.err == nil {
		r.err = errTruncated
	}
	r.pos = len(r.b)
}

// pool records where each constant starts; strings are decoded on demand,
// since most of a large pool is never looked at.
type pool struct {
	b    []byte
	tags []byte
	offs []int // offset of each entry's data, after the tag
	// bootstrap is the class's BootstrapMethods: each a method handle and
	// its static arguments, constants of the pool.
	bootstrap [][]uint16
}

func (p *pool) utf8(i uint16) (string, error) {
	if int(i) >= len(p.tags) || p.tags[i] != tagUtf8 {
		return "", fmt.Errorf("constant %d is not a Utf8 entry", i)
	}
	off := p.offs[i]
	n := int(binary.BigEndian.Uint16(p.b[off:]))
	return decodeModifiedUTF8(p.b[off+2 : off+2+n]), nil
}

func (p *pool) int(i uint16) (int, error) {
	if int(i) >= len(p.tags) || p.tags[i] != tagInteger {
		return 0, fmt.Errorf("constant %d is not an Integer entry", i)
	}
	return int(int32(binary.BigEndian.Uint32(p.b[p.offs[i]:]))), nil
}

// constantValue returns a ConstantValue's Integer, Long, Float, Double or
// String entry, or nil.
func (p *pool) constantValue(i uint16) any {
	kind, n, f, s, err := p.constant(i)
	if err != nil {
		return nil
	}
	switch kind {
	case ConstInt:
		return int32(n)
	case ConstLong:
		return n
	case ConstFloat:
		return float32(f)
	case ConstDouble:
		return f
	case ConstString:
		return s
	}
	return nil
}

func (p *pool) class(i uint16) (string, error) {
	if int(i) >= len(p.tags) || p.tags[i] != tagClass {
		return "", fmt.Errorf("constant %d is not a Class entry", i)
	}
	return p.utf8(binary.BigEndian.Uint16(p.b[p.offs[i]:]))
}

// readPool reads the header of a class file and indexes its constant pool.
func readPool(r *reader) (*pool, error) {
	if r.u4() != 0xCAFEBABE {
		return nil, errors.New("not a class file")
	}
	r.skip(4) // minor and major version
	n := int(r.u2())
	p := &pool{b: r.b, tags: make([]byte, n), offs: make([]int, n)}
	for i := 1; i < n && r.err == nil; i++ {
		tag := r.u1()
		p.tags[i], p.offs[i] = tag, r.pos
		switch tag {
		case tagUtf8:
			r.skip(int(r.u2()))
		case tagClass, tagString, tagMethodType, 19 /* Module */, 20 /* Package */ :
			r.skip(2)
		case tagMethodHandle:
			r.skip(3)
		case tagInteger, tagFloat, tagFieldref, tagMethodref, tagInterfaceMethodref, tagNameAndType,
			tagDynamic, tagInvokeDynamic:
			r.skip(4)
		case tagLong, tagDouble:
			r.skip(8)
			i++ // takes two entries
		default:
			return nil, fmt.Errorf("unknown constant pool tag %d", tag)
		}
	}
	if r.err != nil {
		return nil, r.err
	}
	return p, nil
}

// Parse reads the declarations of a class file.
func Parse(data []byte) (*Info, error) {
	r := &reader{b: data}
	p, err := readPool(r)
	if err != nil {
		return nil, err
	}

	must := func(s string, e error) string {
		if e != nil && err == nil {
			err = e
		}
		return s
	}
	info := &Info{Access: r.u2(), StabilityInferred: -1}
	info.Name = must(p.class(r.u2()))
	if super := r.u2(); super != 0 {
		info.Super = must(p.class(super))
	}
	info.Interfaces = make([]string, r.u2())
	for i := range info.Interfaces {
		info.Interfaces[i] = must(p.class(r.u2()))
	}
	members := func() []Member {
		ms := make([]Member, r.u2())
		for i := range ms {
			m := &ms[i]
			m.Access = r.u2()
			m.Name = must(p.utf8(r.u2()))
			m.Desc = must(p.utf8(r.u2()))
			for range r.u2() {
				name := must(p.utf8(r.u2()))
				size := int(r.u4())
				switch {
				case name == "Signature" && size == 2:
					m.Sig = must(p.utf8(r.u2()))
					size = 0
				case name == "ConstantValue" && size == 2:
					m.Constant = p.constantValue(r.u2())
					size = 0
				case name == "RuntimeInvisibleAnnotations":
					end := r.pos + size
					for range r.u2() {
						typ, e := p.utf8(r.u2())
						if e != nil {
							break
						}
						m.LowPriority = m.LowPriority || typ == "Lkotlin/internal/LowPriorityInOverloadResolution;"
						skipPairs(r)
					}
					size = end - r.pos
				case name == "RuntimeVisibleAnnotations":
					end := r.pos + size
					for range r.u2() {
						typ, e := p.utf8(r.u2())
						if e != nil {
							break
						}
						if typ != "Lkotlin/Deprecated;" && typ != "Lkotlin/DeprecatedSinceKotlin;" {
							skipPairs(r)
							continue
						}
						for range r.u2() {
							elem, e := p.utf8(r.u2())
							if e != nil {
								break
							}
							if elem == "hiddenSince" {
								// @DeprecatedSinceKotlin(hiddenSince = "1.6"),
								// of a version before kc's standard library.
								m.Hidden = true
							}
							if r.u1() != 'e' || elem != "level" {
								r.pos--
								skipValue(r)
								continue
							}
							r.skip(2) // the enum's type
							level, _ := p.utf8(r.u2())
							m.Hidden = m.Hidden || level == "HIDDEN"
						}
					}
					size = end - r.pos
				}
				r.skip(size)
			}
		}
		return ms
	}
	info.Fields = members()
	info.Methods = members()
	for range r.u2() {
		name := must(p.utf8(r.u2()))
		size := int(r.u4())
		switch {
		case name == "Signature" && size == 2:
			info.Sig = must(p.utf8(r.u2()))
			size = 0
		case name == "InnerClasses":
			for range r.u2() {
				inner, outer := r.u2(), r.u2()
				r.u2() // the simple name
				access := r.u2()
				if inner == 0 || must(p.class(inner)) != info.Name {
					continue
				}
				ic := &InnerClass{Access: access}
				if outer != 0 {
					ic.Outer = must(p.class(outer))
				}
				info.Inner = ic
			}
			size = 0
		case name == "RuntimeVisibleAnnotations":
			end := r.pos + size
			k, e := kotlinMeta(r, p, info)
			if e != nil && err == nil {
				err = e
			}
			info.Kotlin = k
			size = end - r.pos
		case name == "RuntimeInvisibleAnnotations":
			end := r.pos + size
			for range r.u2() {
				typ, e := p.utf8(r.u2())
				if e != nil {
					break
				}
				info.Invisible = append(info.Invisible, typ)
				if typ == "Landroidx/compose/runtime/internal/StabilityInferred;" {
					info.StabilityInferred = intArg(r, p, "parameters")
					continue
				}
				skipPairs(r)
			}
			size = end - r.pos
		}
		r.skip(size)
	}
	if r.err != nil {
		return nil, r.err
	}
	if err != nil {
		return nil, err
	}
	return info, nil
}

// kotlinMeta reads a RuntimeVisibleAnnotations attribute and returns the
// arguments of @kotlin.Metadata, if present. It records an annotation
// class's retention and targets in info. Other annotations are skipped.
func kotlinMeta(r *reader, p *pool, info *Info) (*KotlinMeta, error) {
	var meta *KotlinMeta
	for range r.u2() {
		typ, err := p.utf8(r.u2())
		if err != nil {
			return nil, err
		}
		switch typ {
		case "Ljava/lang/annotation/Retention;":
			for range r.u2() {
				r.skip(2) // value
				if v, err := enumValue(r, p); err == nil {
					info.Retention = v
				}
			}
			continue
		case "Ljava/lang/annotation/Repeatable;":
			for range r.u2() {
				r.skip(2) // value
				if r.u1() != 'c' {
					r.pos--
					skipValue(r)
					continue
				}
				if d, err := p.utf8(r.u2()); err == nil && strings.HasPrefix(d, "L") && strings.HasSuffix(d, ";") {
					info.Repeatable = d[1 : len(d)-1]
				}
			}
			continue
		case "Lkotlin/annotation/Target;", "Ljava/lang/annotation/Target;":
			var vs []string
			for range r.u2() {
				r.skip(2) // allowedTargets or value
				arrayValue(r, func() error {
					v, err := enumValue(r, p)
					vs = append(vs, v)
					return err
				})
			}
			if typ == "Lkotlin/annotation/Target;" {
				info.KotlinTargets = vs
			} else {
				info.JavaTargets = vs
			}
			continue
		}
		if typ == "Lkotlin/coroutines/jvm/internal/DebugMetadata;" {
			info.DebugMetadata = map[string][]string{}
			for range r.u2() {
				name, err := p.utf8(r.u2())
				if err != nil {
					return nil, err
				}
				if info.DebugMetadata[name], err = constValues(r, p); err != nil {
					return nil, fmt.Errorf("@DebugMetadata %s: %w", name, err)
				}
			}
			continue
		}
		if typ != "Lkotlin/Metadata;" {
			skipPairs(r)
			continue
		}
		meta = &KotlinMeta{K: 1}
		for range r.u2() {
			name, err := p.utf8(r.u2())
			if err != nil {
				return nil, err
			}
			switch name {
			case "k":
				meta.K, err = intValue(r, p)
			case "xi":
				meta.XI, err = intValue(r, p)
			case "xs":
				meta.XS, err = stringValue(r, p)
			case "pn":
				meta.PN, err = stringValue(r, p)
			case "mv":
				err = arrayValue(r, func() error {
					v, err := intValue(r, p)
					meta.MV = append(meta.MV, v)
					return err
				})
			case "d1", "d2":
				var ss []string
				err = arrayValue(r, func() error {
					v, err := stringValue(r, p)
					ss = append(ss, v)
					return err
				})
				if name == "d1" {
					meta.D1 = ss
				} else {
					meta.D2 = ss
				}
			default:
				skipValue(r)
			}
			if err != nil {
				return nil, fmt.Errorf("@kotlin.Metadata %s: %w", name, err)
			}
		}
	}
	return meta, r.err
}

func intValue(r *reader, p *pool) (int, error) {
	if tag := r.u1(); tag != 'I' {
		return 0, fmt.Errorf("element value %q, want I", tag)
	}
	return p.int(r.u2())
}

// enumValue reads an enum constant's element value and returns the
// constant's name.
func enumValue(r *reader, p *pool) (string, error) {
	if tag := r.u1(); tag != 'e' {
		r.pos-- // let the caller's skipping see the tag
		skipValue(r)
		return "", fmt.Errorf("element value %q, want e", tag)
	}
	r.skip(2) // the enum's type
	return p.utf8(r.u2())
}

func stringValue(r *reader, p *pool) (string, error) {
	if tag := r.u1(); tag != 's' {
		return "", fmt.Errorf("element value %q, want s", tag)
	}
	return p.utf8(r.u2())
}

func arrayValue(r *reader, elem func() error) error {
	if tag := r.u1(); tag != '[' {
		return fmt.Errorf("element value %q, want [", tag)
	}
	for range r.u2() {
		if err := elem(); err != nil {
			return err
		}
	}
	return nil
}

// constValues reads an element value that is a string or int constant,
// or an array of them, as strings.
func constValues(r *reader, p *pool) ([]string, error) {
	switch tag := r.u1(); tag {
	case 's':
		s, err := p.utf8(r.u2())
		return []string{s}, err
	case 'I':
		v, err := p.int(r.u2())
		return []string{strconv.Itoa(v)}, err
	case '[':
		out := []string{}
		for range r.u2() {
			vs, err := constValues(r, p)
			if err != nil {
				return nil, err
			}
			out = append(out, vs...)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("element value %q, want a string or an int", tag)
	}
}

// intArg reads an annotation's element-value pairs and returns its Int
// argument name, or -1.
func intArg(r *reader, p *pool, name string) int {
	v := -1
	for range r.u2() {
		n, _ := p.utf8(r.u2())
		if r.u1() != 'I' || n != name {
			r.pos--
			skipValue(r)
			continue
		}
		if i, err := p.int(r.u2()); err == nil {
			v = i
		}
	}
	return v
}

func skipPairs(r *reader) {
	for range r.u2() {
		r.skip(2) // name
		skipValue(r)
	}
}

func skipValue(r *reader) {
	switch r.u1() {
	case 'e':
		r.skip(4)
	case '@':
		r.skip(2)
		skipPairs(r)
	case '[':
		for range r.u2() {
			skipValue(r)
		}
	default: // a constant or a class
		r.skip(2)
	}
}

// decodeModifiedUTF8 decodes the string encoding used in class files.
func decodeModifiedUTF8(b []byte) string {
	ascii := true
	for _, c := range b {
		if c == 0 || c >= 0x80 {
			ascii = false
			break
		}
	}
	if ascii {
		return string(b)
	}
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
