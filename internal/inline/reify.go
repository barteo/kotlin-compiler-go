package inline

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	cf "github.com/barteo/kotlin-compiler-go/internal/classfile"
)

// A Template is a class that an inline function's body creates after a
// call of Intrinsics.needClassReification: a lambda or object of the
// standard library whose code operates on the function's reified type
// parameters, as filterIsInstance's { it is R }. Its class file marks
// the operations as an inline body does. kotlinc's inliner copies such a
// class for each call, with the call's type arguments in the marked
// operations; Instantiate does that.
type Template struct {
	Name string // the class's internal name
	data []byte
	// pool are the offsets of the constant pool's entries, by index (0
	// and the second slot of a long or double are unused); poolEnd is
	// where the pool ends.
	pool    []int
	poolEnd int
	ops     []reifiedOp
	// nested are the templates that the class's code creates: after
	// needClassReification calls of its own, at the calls' file offsets,
	// a lambda in a lambda, as an inline function's lambda that a lambda
	// in another inline function copies; and the anonymous classes nested
	// in it (call -1), as an object's continuations and collectors, which
	// kotlinc's inliner copies with it. Each copy creates copies of them,
	// in the order kotlinc numbers them.
	nested []nestedTemplate
	// drop are the file ranges of the suspend members' m$$forInline twins,
	// from which kotlinc's inliner regenerates m, with the lambda it
	// inlines into the class: a copy has none, so kotlinc copies m, whose
	// state machine calls the lambda object the copy holds. methodsAt is
	// the offset of the methods' count.
	drop      [][2]int
	methodsAt int
	// copied are the file offsets of the needClassReification calls
	// before the creation of a class copied already: the class itself or
	// a template that creates it, as a lambda's create() or an object's
	// INSTANCE, or one of nested that another method creates too, as
	// collect and collect$$forInline do. The copy creates the copy of the
	// class, which renaming its constants gives.
	copied []int
}

type nestedTemplate struct {
	call int // the file offset of the invokestatic
	t    *Template
}

// Instance is a copy of a template: a class file and its name.
type Instance struct {
	Name string
	Data []byte
}

// reifiedOp is a marked operation in a template's code: the marker,
// three instructions from start, and the operation, which takes a class
// constant at classAt (a class ldc's one-byte index at classAt-1 if
// ldc).
type reifiedOp struct {
	start, op int // file offsets
	kind      int
	param     string
	ldc       bool // a class ldc, whose index is one byte
}

// LoadTemplates reads the templates of the classes that the body, and
// the private methods it inlines, create after needClassReification
// calls, and those of its Copies, with read, which returns a class file.
func (b *Body) LoadTemplates(read func(class string) ([]byte, error)) error {
	for i, name := range b.reify {
		t, err := loadTemplate(name, read, nil)
		if err != nil {
			return err
		}
		if b.Templates == nil {
			b.Templates = map[int]*Template{}
		}
		b.Templates[i] = t
	}
	for i, name := range b.copy {
		// A class kc can't copy is referred to as it is.
		t, err := loadTemplate(name, read, nil)
		if err != nil || len(t.ops) > 0 {
			continue
		}
		if b.Copies == nil {
			b.Copies = map[int]*Template{}
		}
		b.Copies[i] = t
	}
	for _, sub := range b.Calls {
		if err := sub.LoadTemplates(read); err != nil {
			return err
		}
	}
	return nil
}

// loadTemplate reads a template and the templates it creates. path are
// the templates that create it, which it may create in turn.
func loadTemplate(name string, read func(class string) ([]byte, error), path []string) (*Template, error) {
	if len(path) > 8 {
		return nil, fail("class %s creates templates too deeply", name)
	}
	data, err := read(name)
	if err != nil {
		return nil, err
	}
	t, children, err := readTemplate(name, data)
	if err != nil {
		return nil, err
	}
	path = append(path, name)
	for _, c := range children {
		if slices.Contains(path, c.class) || slices.ContainsFunc(t.nested, func(n nestedTemplate) bool { return n.t.Name == c.class }) {
			t.copied = append(t.copied, c.call)
			continue
		}
		n, err := loadTemplate(c.class, read, path)
		if err != nil {
			return nil, err
		}
		t.nested = append(t.nested, nestedTemplate{call: c.call, t: n})
	}
	return t, nil
}

// nestedRef is a needClassReification call in a template's code, at
// file offset call, before the creation of class.
type nestedRef struct {
	call  int
	class string
}

// readTemplate reads the class file of a class to copy for each call, and
// returns the templates its code creates.
func readTemplate(name string, data []byte) (*Template, []nestedRef, error) {
	t := &Template{Name: name, data: data}
	if len(data) < 10 {
		return nil, nil, fail("class %s is truncated", name)
	}
	n := int(binary.BigEndian.Uint16(data[8:]))
	t.pool = make([]int, n)
	pos := 10
	for i := 1; i < n; i++ {
		if pos >= len(data) {
			return nil, nil, fail("class %s is truncated", name)
		}
		t.pool[i] = pos
		switch tag := data[pos]; tag {
		case 1: // Utf8
			pos += 3 + int(binary.BigEndian.Uint16(data[pos+1:]))
		case 3, 4, 9, 10, 11, 12, 17, 18: // Integer, Float, refs, NameAndType, Dynamic
			pos += 5
		case 5, 6: // Long, Double
			pos += 9
			i++
		case 7, 8, 16, 19, 20: // Class, String, MethodType, Module, Package
			pos += 3
		case 15: // MethodHandle
			pos += 4
		default:
			return nil, nil, fail("class %s has a constant of tag %d", name, tag)
		}
	}
	t.poolEnd = pos
	methods, err := cf.ReadMethods(data)
	if err != nil {
		return nil, nil, err
	}
	if err := t.findTwins(); err != nil {
		return nil, nil, err
	}
	// kotlinc numbers the copies of the classes nested in a template in
	// the order its code creates them, reading a suspend member's
	// m$$forInline in place of m: the methods in that order, each twin
	// before its member.
	var order []*cf.Method
	for _, m := range methods {
		if strings.HasSuffix(m.Name, "$$forInline") {
			continue
		}
		if j := slices.IndexFunc(methods, func(f *cf.Method) bool { return f.Name == m.Name+"$$forInline" && f.Desc == m.Desc }); j >= 0 {
			order = append(order, methods[j])
		}
		order = append(order, m)
	}
	for _, m := range methods {
		if !slices.Contains(order, m) {
			order = append(order, m)
		}
	}
	var nested []nestedRef
	for _, m := range order {
		insns, err := m.Decode()
		if err != nil {
			return nil, nil, err
		}
		for i, in := range insns {
			// An anonymous class nested in the template, which kotlinc's
			// inliner copies with it: the copy's code creates the copy.
			if c := created(&in); isNested(name, c) && !slices.ContainsFunc(nested, func(r nestedRef) bool { return r.class == c }) {
				nested = append(nested, nestedRef{call: -1, class: c})
				continue
			}
			if in.Op == cf.INVOKESTATIC && in.Owner == "kotlin/jvm/internal/Intrinsics" && in.Name == "needClassReification" && in.Desc == "()V" {
				var class string
				switch next := &insns[min(i+1, len(insns)-1)]; {
				case i+1 < len(insns) && next.Op == cf.NEW:
					class = next.Class
				case i+1 < len(insns) && next.Op == cf.GETSTATIC && next.Name == "INSTANCE" && next.Desc == "L"+next.Owner+";":
					class = next.Owner
				default:
					return nil, nil, fail("class %s's needClassReification call doesn't precede an object's creation", name)
				}
				nested = append(nested, nestedRef{call: m.CodeOff + in.Off, class: class})
				continue
			}
			if in.Op != cf.INVOKESTATIC || in.Owner != "kotlin/jvm/internal/Intrinsics" || in.Name != "reifiedOperationMarker" {
				continue
			}
			if i < 2 || i+1 >= len(insns) || !isConst(&insns[i-2]) || insns[i-1].Op != cf.LDC || insns[i-1].Const != cf.ConstString {
				return nil, nil, fail("class %s marks a reified operation kc doesn't read", name)
			}
			kind, op := constInt(&insns[i-2]), insns[i+1]
			if want, ok := reifiedOps[kind]; !ok || op.Op != want && !(want == cf.LDC && op.Op == cf.LDC_W) {
				return nil, nil, fail("class %s has a reified operation kc doesn't support (%d)", name, kind)
			}
			t.ops = append(t.ops, reifiedOp{
				start: m.CodeOff + insns[i-2].Off, op: m.CodeOff + op.Off, kind: kind,
				param: trimNullable(insns[i-1].Str), ldc: op.Op == cf.LDC,
			})
		}
	}
	return t, nested, nil
}

// findTwins sets t.methodsAt and t.drop: the ranges of the method_info
// structures of the methods named m$$forInline whose member m the class
// has.
func (t *Template) findTwins() error {
	data, pos := t.data, t.poolEnd
	u2 := func() int {
		if pos+2 > len(data) {
			pos = len(data) + 1
			return 0
		}
		v := int(binary.BigEndian.Uint16(data[pos:]))
		pos += 2
		return v
	}
	attrs := func() {
		for range u2() {
			pos += 2
			if pos+4 > len(data) {
				pos = len(data) + 1
				return
			}
			pos += 4 + int(binary.BigEndian.Uint32(data[pos:]))
		}
	}
	utf8 := func(i int) string {
		if i <= 0 || i >= len(t.pool) || t.pool[i] == 0 || data[t.pool[i]] != 1 {
			return ""
		}
		at := t.pool[i]
		return string(data[at+3 : at+3+int(binary.BigEndian.Uint16(data[at+1:]))])
	}
	pos += 6 // access, this, super
	pos += 2 * u2()
	for range u2() { // fields
		pos += 6
		attrs()
	}
	t.methodsAt = pos
	type info struct {
		name, desc string
		start, end int
	}
	var ms []info
	for range u2() {
		start := pos
		pos += 2
		name, desc := utf8(u2()), utf8(u2())
		attrs()
		ms = append(ms, info{name, desc, start, pos})
	}
	if pos > len(data) {
		return fail("class %s is truncated", t.Name)
	}
	for _, m := range ms {
		member, ok := strings.CutSuffix(m.name, "$$forInline")
		if ok && slices.ContainsFunc(ms, func(o info) bool { return o.name == member && o.desc == m.desc }) {
			t.drop = append(t.drop, [2]int{m.start, m.end})
		}
	}
	return nil
}

// created returns the class whose object in creates, by new or as its
// INSTANCE, or "".
func created(in *cf.Insn) string {
	switch {
	case in.Op == cf.NEW:
		return in.Class
	case in.Op == cf.GETSTATIC && in.Name == "INSTANCE" && in.Desc == "L"+in.Owner+";":
		return in.Owner
	}
	return ""
}

// isNested reports whether class is an anonymous class nested in the
// class outer, as kotlinc's inliner tells: outer$...$N, as a member's
// continuation outer$emit$1 too.
func isNested(outer, class string) bool {
	rest, ok := strings.CutPrefix(class, outer+"$")
	if !ok {
		return false
	}
	_, err := strconv.Atoi(rest[strings.LastIndex(rest, "$")+1:])
	return err == nil
}

func trimNullable(s string) string {
	if len(s) > 0 && s[len(s)-1] == '?' {
		return s[:len(s)-1]
	}
	return s
}

// Params returns the reified type parameters that the template's
// operations use.
func (t *Template) Params() []string {
	var out []string
	for _, op := range t.ops {
		out = append(out, op.param)
	}
	for _, n := range t.nested {
		out = append(out, n.t.Params()...)
	}
	return out
}

// Instantiate returns a copy of the template's class file named name,
// whose marked operations use the classes of the type arguments, by the
// type parameters' names (internal names, or descriptors of arrays). The
// markers become nops, so the code keeps its offsets and its frames.
func (t *Template) Instantiate(name string, classes map[string]string) ([]Instance, error) {
	return t.instantiate(name, classes, map[string]string{t.Name: name})
}

// instantiate copies the template as name, with the classes that renames
// maps renamed in its constants, and its nested templates, which it
// creates instead, as name$1, name$2 and so on.
func (t *Template) instantiate(name string, classes map[string]string, renames map[string]string) ([]Instance, error) {
	renames = maps.Clone(renames)
	renames[t.Name] = name
	for i, n := range t.nested {
		renames[n.t.Name] = name + "$" + strconv.Itoa(i+1)
	}
	var out []Instance
	for _, n := range t.nested {
		ins, err := n.t.instantiate(renames[n.t.Name], classes, renames)
		if err != nil {
			return nil, err
		}
		out = append(out, ins...)
	}
	data, err := t.instantiate1(name, classes, renames)
	if err != nil {
		return nil, err
	}
	return append([]Instance{{Name: name, Data: data}}, out...), nil
}

func (t *Template) instantiate1(name string, classes map[string]string, renames map[string]string) ([]byte, error) {
	data := t.data
	dotted := map[string]string{}
	for old, n := range renames {
		dotted[strings.ReplaceAll(old, "/", ".")] = strings.ReplaceAll(n, "/", ".")
	}
	var pool bytes.Buffer
	for i := 1; i < len(t.pool); i++ {
		start := t.pool[i]
		if start == 0 {
			continue // the second slot of a long or double
		}
		end := t.poolEnd
		for j := i + 1; j < len(t.pool); j++ {
			if t.pool[j] != 0 {
				end = t.pool[j]
				break
			}
		}
		e := data[start:end]
		if e[0] == 1 {
			s := e[3:]
			if n, ok := renames[string(s)]; ok {
				s = []byte(n)
			} else if n, ok := dotted[string(s)]; ok {
				s = []byte(n) // a suspend lambda's @DebugMetadata(c = ...)
			} else {
				// Descriptors and signatures, generic ones too.
				for old, n := range renames {
					for _, end := range []string{";", "<"} {
						if oldDesc := []byte("L" + old + end); bytes.Contains(s, oldDesc) {
							s = bytes.ReplaceAll(s, oldDesc, []byte("L"+n+end))
						}
					}
				}
			}
			e = append([]byte{1, byte(len(s) >> 8), byte(len(s))}, s...)
		}
		pool.Write(e)
	}
	// The classes of the operations, as new constants.
	count := len(t.pool)
	index := map[string]int{}
	for _, op := range t.ops {
		cls, ok := classes[op.param]
		if !ok {
			return nil, fmt.Errorf("no type argument for reified type parameter %s", op.param)
		}
		if _, ok := index[cls]; ok {
			continue
		}
		pool.Write([]byte{1, byte(len(cls) >> 8), byte(len(cls))})
		pool.WriteString(cls)
		pool.Write([]byte{7, byte(count >> 8), byte(count)})
		index[cls] = count + 1
		count += 2
	}
	if count > 0xFFFF {
		return nil, fmt.Errorf("class %s: constant pool overflow", name)
	}
	out := append([]byte(nil), data[:8]...)
	out = binary.BigEndian.AppendUint16(out, uint16(count))
	out = append(out, pool.Bytes()...)
	shift := len(out) - t.poolEnd
	out = append(out, data[t.poolEnd:]...)
	for _, op := range t.ops {
		idx := index[classes[op.param]]
		start, at := op.start+shift, op.op+shift
		for k := start; k < at; k++ {
			out[k] = cf.NOP
		}
		if op.ldc {
			// ldc's index is one byte: an ldc_w in its place takes the
			// marker's last byte.
			at--
			out[at] = cf.LDC_W
		}
		binary.BigEndian.PutUint16(out[at+1:], uint16(idx))
	}
	// The needClassReification calls, whose classes are copied.
	calls := slices.Clone(t.copied)
	for _, n := range t.nested {
		calls = append(calls, n.call)
	}
	for _, call := range calls {
		if call < 0 {
			continue // a nested class created without a call
		}
		for k := call + shift; k < call+shift+3; k++ {
			out[k] = cf.NOP
		}
	}
	// The twins, last first.
	if len(t.drop) > 0 {
		at := t.methodsAt + shift
		binary.BigEndian.PutUint16(out[at:], binary.BigEndian.Uint16(out[at:])-uint16(len(t.drop)))
		for _, r := range slices.Backward(t.drop) {
			out = slices.Delete(out, r[0]+shift, r[1]+shift)
		}
	}
	return out, nil
}
