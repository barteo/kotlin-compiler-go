package check

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/classfile"
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// The annotations Metro reads, of the program's declarations, as the
// checker resolved them, and of the class path's, as their class files
// keep them.

// mAnn is an annotation: its class's internal name and its arguments.
type mAnn struct {
	cls  string
	args map[string]mVal
	// typeArgs are a generic annotation's type arguments, which only the
	// program's annotations have: binding<T>().
	typeArgs []types.Type
}

// mVal is an annotation's argument: Tag is as classfile.ElementValue's.
type mVal struct {
	tag   byte
	class string // c: the class's internal name, kotlin/Nothing for Nothing::class
	str   string
	num   int64
	f     float64
	enum  [2]string // e: the enum class's internal name, the entry
	elems []mVal
	ann   *mAnn
}

func (a *mAnn) is(name string) bool { return a != nil && a.cls == name }

// key is a string that is the same for equal annotations.
func (a *mAnn) key() string {
	var b strings.Builder
	b.WriteString(a.cls)
	b.WriteByte('(')
	names := make([]string, 0, len(a.args))
	for n, v := range a.args {
		if v.tag == '[' && len(v.elems) == 0 {
			continue
		}
		names = append(names, n)
	}
	slices.Sort(names)
	for i, n := range names {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(n)
		b.WriteByte('=')
		b.WriteString(a.args[n].key())
	}
	b.WriteByte(')')
	return b.String()
}

func (v mVal) key() string {
	switch v.tag {
	case 'c':
		return "c:" + v.class
	case 's':
		return "s:" + strconv.Quote(v.str)
	case 'e':
		return "e:" + v.enum[0] + "." + v.enum[1]
	case 'D', 'F':
		return string(v.tag) + ":" + strconv.FormatFloat(v.f, 'g', -1, 64)
	case '[':
		parts := make([]string, len(v.elems))
		for i, e := range v.elems {
			parts[i] = e.key()
		}
		return "[" + strings.Join(parts, ",") + "]"
	case '@':
		return "@" + v.ann.key()
	}
	return string(v.tag) + ":" + strconv.FormatInt(v.num, 10)
}

// classes returns the classes of a class or class array argument.
func (v mVal) classes() []string {
	if v.tag == 'c' {
		return []string{v.class}
	}
	var out []string
	for _, e := range v.elems {
		out = append(out, e.classes()...)
	}
	return out
}

// fromClassfile converts a class file's annotation.
func fromClassfile(a classfile.Annotation) *mAnn {
	m := &mAnn{cls: descClass(a.Type), args: map[string]mVal{}}
	for _, p := range a.Values {
		m.args[p.Name] = fromElement(p.Value)
	}
	return m
}

func fromElement(e classfile.ElementValue) mVal {
	v := mVal{tag: e.Tag}
	switch e.Tag {
	case 'c':
		v.class = classDescName(e.Class)
	case 's':
		v.str, _ = e.Const.(string)
	case 'e':
		v.enum = [2]string{descClass(e.Enum[0]), e.Enum[1]}
	case 'D':
		f, _ := e.Const.(float64)
		v.f = f
	case 'F':
		f, _ := e.Const.(float32)
		v.f = float64(f)
	case '[':
		for _, x := range e.Elems {
			v.elems = append(v.elems, fromElement(x))
		}
	case '@':
		v.ann = fromClassfile(*e.Ann)
	default:
		n, _ := e.Const.(int64)
		v.num = n
	}
	return v
}

// descClass returns the internal name of an object type's descriptor.
func descClass(d string) string {
	if strings.HasPrefix(d, "L") && strings.HasSuffix(d, ";") {
		return d[1 : len(d)-1]
	}
	return d
}

// classDescName is the internal name of a class literal's class, from
// its return descriptor: V for Unit and the boxes' for primitives, as
// kotlinc writes them.
func classDescName(d string) string {
	switch d {
	case "V":
		return "kotlin/Unit"
	case "Ljava/lang/Void;":
		return "kotlin/Nothing"
	}
	return descClass(d)
}

// expandContainers replaces the containers of repeated annotations with
// the annotations they hold.
func expandContainers(as []*mAnn) []*mAnn {
	var out []*mAnn
	for _, a := range as {
		if v, ok := a.args["value"]; ok && v.tag == '[' && strings.HasSuffix(a.cls, "$Container") && len(v.elems) > 0 && v.elems[0].tag == '@' {
			for _, e := range v.elems {
				out = append(out, e.ann)
			}
			continue
		}
		out = append(out, a)
	}
	return out
}

// fromSource converts the program's annotations that the checker
// resolved.
func (c *Checker) fromSource(as []syntax.Annotation, scope *types.ClassSym) []*mAnn {
	var out []*mAnn
	for i := range as {
		if a := c.fromSourceAnn(&as[i], scope); a != nil {
			out = append(out, a)
		}
	}
	return out
}

func (c *Checker) fromSourceAnn(a *syntax.Annotation, scope *types.ClassSym) *mAnn {
	if a.Class == nil {
		return nil
	}
	m := &mAnn{cls: a.Class.Name, args: map[string]mVal{}}
	for _, v := range a.Values {
		var elems []mVal
		for _, e := range v.Elems {
			elems = append(elems, c.fromSourceElem(e, scope))
		}
		if v.Array {
			m.args[v.Name] = mVal{tag: '[', elems: elems}
		} else if len(elems) == 1 {
			m.args[v.Name] = elems[0]
		}
	}
	if len(a.TypeArgs) > 0 {
		saved := c.lookupCls
		c.lookupCls = scope
		for _, ta := range a.TypeArgs {
			if ta.Type == nil {
				m.typeArgs = append(m.typeArgs, types.InvalidType)
				continue
			}
			m.typeArgs = append(m.typeArgs, c.resolveType(ta.Type))
		}
		c.lookupCls = saved
	}
	return m
}

func (c *Checker) fromSourceElem(e syntax.AnnotationElem, scope *types.ClassSym) mVal {
	if e.Nested != nil {
		if e.Nested.Class == nil {
			// An annotation the checker didn't keep, as binding<T>():
			// its class is still known.
			cls := c.metroAnnotationClass(e.Nested, scope)
			if cls == nil {
				return mVal{tag: '@', ann: &mAnn{args: map[string]mVal{}}}
			}
			e.Nested.Class = cls
			defer func() { e.Nested.Class = nil }()
		}
		return mVal{tag: '@', ann: c.fromSourceAnn(e.Nested, scope)}
	}
	return metroConst(e.X)
}

// metroAnnotationClass resolves the class of an annotation by its name.
func (c *Checker) metroAnnotationClass(a *syntax.Annotation, scope *types.ClassSym) *types.ClassSym {
	saved := c.lookupCls
	c.lookupCls = scope
	defer func() { c.lookupCls = saved }()
	t := c.resolveType(&syntax.TypeRef{P: a.P, Name: a.Name})
	if t.K != types.Class {
		return nil
	}
	return t.C
}

// metroConst returns the value of an annotation argument's checked
// expression.
func metroConst(x syntax.Expr) mVal {
	switch x := x.(type) {
	case *syntax.ClassLit:
		return mVal{tag: 'c', class: typeClassName(x.Class)}
	case *syntax.StringLit:
		s, _ := constString(x, 0)
		return mVal{tag: 's', str: s}
	case *syntax.IntLit:
		if x.Long {
			return mVal{tag: 'J', num: x.Val}
		}
		return mVal{tag: 'I', num: x.Val}
	case *syntax.BoolLit:
		n := int64(0)
		if x.Val {
			n = 1
		}
		return mVal{tag: 'Z', num: n}
	case *syntax.DoubleLit:
		return mVal{tag: 'D', f: x.Val}
	case *syntax.Unary:
		v := metroConst(x.X)
		if x.Op == syntax.MINUS {
			v.num, v.f = -v.num, -v.f
		}
		return v
	case *syntax.Ident:
		if v := x.Sym; v != nil {
			if pd, ok := v.Decl.(*syntax.PropDecl); ok && v.Const && pd.Init != nil {
				return metroConst(pd.Init)
			}
			if v.Kind == types.Static && v.Owner != nil && v.Type.K == types.Class && v.Type.C == v.Owner {
				return mVal{tag: 'e', enum: [2]string{v.Owner.Name, v.Name}}
			}
			return metroConstant(v.Constant)
		}
	case *syntax.Member:
		if v := x.Prop; v != nil {
			if v.Kind == types.Static && v.Owner != nil && v.Type.K == types.Class && v.Type.C == v.Owner {
				return mVal{tag: 'e', enum: [2]string{v.Owner.Name, v.Name}}
			}
			if pd, ok := v.Decl.(*syntax.PropDecl); ok && v.Const && pd.Init != nil {
				return metroConst(pd.Init)
			}
			return metroConstant(v.Constant)
		}
	}
	return mVal{tag: '?'}
}

func metroConstant(c any) mVal {
	switch c := c.(type) {
	case string:
		return mVal{tag: 's', str: c}
	case int32:
		return mVal{tag: 'I', num: int64(c)}
	case int64:
		return mVal{tag: 'J', num: c}
	case float64:
		return mVal{tag: 'D', f: c}
	}
	return mVal{tag: '?'}
}

// typeClassName is the internal name of a class literal's class.
func typeClassName(t types.Type) string {
	switch t.K {
	case types.Class:
		return t.C.Name
	case types.Nothing:
		return "kotlin/Nothing"
	case types.Unit:
		return "kotlin/Unit"
	case types.Any:
		return "java/lang/Object"
	case types.String:
		return "java/lang/String"
	}
	return "?" + t.String()
}

// kotlinAnnSrc writes an annotation as Kotlin.
func (m *metroAnalysis) kotlinAnnSrc(a *mAnn) (string, error) {
	cls := m.class(a.cls)
	if cls == nil {
		return "", fmt.Errorf("unknown annotation class %s", a.cls)
	}
	names := make([]string, 0, len(a.args))
	for n := range a.args {
		names = append(names, n)
	}
	slices.Sort(names)
	var args []string
	for _, n := range names {
		v, err := m.kotlinValSrc(a.args[n])
		if err != nil {
			return "", err
		}
		args = append(args, "`"+n+"` = "+v)
	}
	return "@" + metroClassName(cls) + "(" + strings.Join(args, ", ") + ")", nil
}

// kotlinValSrc writes an annotation's argument as Kotlin.
func (m *metroAnalysis) kotlinValSrc(v mVal) (string, error) {
	switch v.tag {
	case 'c':
		cls := m.class(v.class)
		switch {
		case v.class == "kotlin/Nothing":
			return "kotlin.Nothing::class", nil
		case cls == nil:
			return "", fmt.Errorf("unknown class %s", v.class)
		}
		return metroClassName(cls) + "::class", nil
	case 's':
		return kotlinString(v.str), nil
	case 'I', 'S', 'B':
		return strconv.FormatInt(v.num, 10), nil
	case 'J':
		return strconv.FormatInt(v.num, 10) + "L", nil
	case 'Z':
		return strconv.FormatBool(v.num != 0), nil
	case 'C':
		return "'\\u" + fmt.Sprintf("%04x", v.num) + "'", nil
	case 'D':
		return strconv.FormatFloat(v.f, 'g', -1, 64) + metroFloatSuffix(v.f, ""), nil
	case 'F':
		return strconv.FormatFloat(v.f, 'g', -1, 32) + metroFloatSuffix(v.f, "f"), nil
	case 'e':
		cls := m.class(v.enum[0])
		if cls == nil {
			return "", fmt.Errorf("unknown enum class %s", v.enum[0])
		}
		return metroClassName(cls) + "." + v.enum[1], nil
	case '[':
		var parts []string
		for _, e := range v.elems {
			s, err := m.kotlinValSrc(e)
			if err != nil {
				return "", err
			}
			parts = append(parts, s)
		}
		return "[" + strings.Join(parts, ", ") + "]", nil
	case '@':
		s, err := m.kotlinAnnSrc(v.ann)
		return strings.TrimPrefix(s, "@"), err
	}
	return "", fmt.Errorf("an annotation argument kc can't write")
}

func metroFloatSuffix(f float64, suffix string) string {
	s := strconv.FormatFloat(f, 'g', -1, 64)
	if !strings.ContainsAny(s, ".eE") {
		return ".0" + suffix
	}
	return suffix
}

// kotlinString writes a string literal.
func kotlinString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"', '\\', '$':
			b.WriteByte('\\')
			b.WriteRune(r)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 0x20 {
				fmt.Fprintf(&b, `\u%04x`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}
