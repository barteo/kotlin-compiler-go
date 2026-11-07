package ka

import (
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/codegen"
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
	"github.com/barteo/kotlin-compiler-go/psi"
)

// ValueKind is the kind of an annotation argument's Value. The numbers
// are part of libkc's ABI: append only.
type ValueKind uint8

const (
	ValueConst      ValueKind = iota + 1
	ValueEnum                 // an enum entry: ClassID and Entry
	ValueClass                // a class literal, A::class: Type
	ValueAnnotation           // a nested annotation: Annotation
	ValueArray                // Elems
	// ValueUnsupported is a value kc didn't compute: one with errors, or
	// a constant expression kc doesn't fold.
	ValueUnsupported
)

// ConstKind is the type of a ValueConst. The numbers are part of libkc's
// ABI: append only.
type ConstKind uint8

const (
	ConstNull ConstKind = iota
	ConstBoolean
	ConstChar
	ConstByte
	ConstUByte
	ConstShort
	ConstUShort
	ConstInt
	ConstUInt
	ConstLong
	ConstULong
	ConstFloat
	ConstDouble
	ConstString
)

// Value is the value of an annotation's argument, or of an annotation
// class's parameter's default.
type Value struct {
	Kind ValueKind
	// Node is the expression it was written as; 0 if it wasn't, as the
	// array of a vararg parameter's arguments.
	Node psi.NodeID
	// Const is a ValueConst's type. Bits is a Boolean (0 or 1), a Char's
	// code, or an integer's value (an unsigned one's bits); Float is a
	// Float's or a Double's, and Str a String's.
	Const ConstKind
	Bits  int64
	Float float64
	Str   string
	// ClassID and Entry are a ValueEnum's class and entry name.
	ClassID    string
	Entry      string
	Type       *Type       // a ValueClass's type
	Annotation *Annotation // a ValueAnnotation's
	Elems      []*Value    // a ValueArray's
}

// NamedValue is an argument of an annotation: its parameter's name and
// its value.
type NamedValue struct {
	Name  string
	Value *Value
}

// args returns the arguments of annotation a, whose checked values are
// values: those written, in the order they are written, as FIR's
// argument mapping has them. A vararg parameter without arguments, whose
// value is an empty array, isn't written.
func (b *fileBuilder) args(a *syntax.Annotation, values []syntax.AnnotationValue) []NamedValue {
	var out []NamedValue
	var at []int
	next := 0 // the next positional argument
	for _, v := range values {
		if v.Array && len(v.Elems) == 0 && !named(a, v.Name) {
			continue
		}
		out = append(out, NamedValue{Name: v.Name, Value: b.checked(v)})
		pos := slices.IndexFunc(a.Args, func(arg syntax.AnnotationArg) bool { return arg.Name == v.Name })
		if pos < 0 {
			for next < len(a.Args) && a.Args[next].Name != "" {
				next++
			}
			pos = next
			next++
		}
		at = append(at, pos)
	}
	idx := make([]int, len(out))
	for i := range idx {
		idx[i] = i
	}
	slices.SortStableFunc(idx, func(x, y int) int { return at[x] - at[y] })
	sorted := make([]NamedValue, len(out))
	for i, j := range idx {
		sorted[i] = out[j]
	}
	return sorted
}

// named reports whether annotation a has an argument named name; false
// for a nested annotation, which keeps no arguments.
func named(a *syntax.Annotation, name string) bool {
	for _, arg := range a.Args {
		if arg.Name == name {
			return true
		}
	}
	return false
}

// checked returns the model of a checked argument v.
func (b *fileBuilder) checked(v syntax.AnnotationValue) *Value {
	if !v.Array {
		if len(v.Elems) == 0 {
			return &Value{Kind: ValueUnsupported}
		}
		return b.elem(v.Type, v.Elems[0])
	}
	out := &Value{Kind: ValueArray}
	et := types.VarargElem(v.Type)
	for _, e := range v.Elems {
		out.Elems = append(out.Elems, b.elem(et, e))
	}
	if len(out.Elems) > 0 {
		out.Node = b.arrayNode(out.Elems[0].Node)
	}
	return out
}

// arrayNode returns the node of the array literal whose first element is
// node first: [a, b] or arrayOf(a, b); 0 for the arguments of a vararg
// parameter.
func (b *fileBuilder) arrayNode(first psi.NodeID) psi.NodeID {
	if first == 0 || b.snap == nil {
		return 0
	}
	p := b.snap.Node(first).Parent()
	switch p.NumericKind() {
	case psi.KindCOLLECTION_LITERAL_EXPRESSION:
		return p.ID()
	case psi.KindVALUE_ARGUMENT:
		call := p.Parent().Parent()
		if call.NumericKind() != psi.KindCALL_EXPRESSION {
			return 0
		}
		callee := call.FirstChild()
		if name := callee.Text(); name == "arrayOf" || strings.HasSuffix(name, "ArrayOf") {
			return call.ID()
		}
	}
	return 0
}

// elem returns the model of e, a value of type t.
func (b *fileBuilder) elem(t types.Type, e syntax.AnnotationElem) *Value {
	if e.Nested != nil {
		return &Value{Kind: ValueAnnotation, Node: b.prov.Annotation(e.Nested.P), Annotation: b.nested(e.Nested)}
	}
	if e.X == nil {
		return &Value{Kind: ValueUnsupported}
	}
	v := b.constant(t, e.X)
	v.Node = b.exprNode(e.X)
	return v
}

// nested returns the model of a nested annotation, one in another's
// arguments.
func (b *fileBuilder) nested(a *syntax.Annotation) *Annotation {
	cls := a.Class
	if cls == nil {
		cls = b.annots[a.P]
	}
	return &Annotation{ClassID: classID(cls), Node: b.prov.Annotation(a.P), Args: b.args(a, a.Values)}
}

// constant returns the model of x, a checked constant, enum entry or
// class literal of type t, without its node; ValueUnsupported if kc
// doesn't compute it.
func (b *fileBuilder) constant(t types.Type, x syntax.Expr) (out *Value) {
	unsupported := &Value{Kind: ValueUnsupported}
	defer func() {
		if recover() != nil {
			out = unsupported
		}
	}()
	t = t.NonNull()
	if cl, ok := x.(*syntax.ClassLit); ok {
		if cl.Class.K == types.Invalid { // an error type, as FIR keeps it
			return &Value{Kind: ValueClass, Type: &Type{Kind: TypeError, Name: classLitName(cl)}}
		}
		t := b.typ(cl.Class, nil)
		if t.Kind == TypeClass && cl.Class.K == types.Class {
			if id := b.javaWritten(cl.Class.C, classLitName(cl)); id != "" {
				t.ClassID = id
			}
		}
		return &Value{Kind: ValueClass, Type: t}
	}
	if t.K == types.Class && t.C != nil && isEnumClass(t.C) {
		name := ""
		switch x := x.(type) {
		case *syntax.Member:
			name = x.Name
		case *syntax.Ident:
			name = x.Name
			if x.Member != nil {
				name = x.Member.Name
			}
		}
		if name == "" {
			return unsupported
		}
		return &Value{Kind: ValueEnum, ClassID: classID(t.C), Entry: name}
	}
	if t.K == types.Class && t.C != nil && types.IsValue(t) {
		bits, ok := codegen.UnsignedConst(x)
		if !ok {
			return unsupported
		}
		switch t.C.Name {
		case "kotlin/UByte":
			return &Value{Kind: ValueConst, Const: ConstUByte, Bits: int64(uint8(bits))}
		case "kotlin/UShort":
			return &Value{Kind: ValueConst, Const: ConstUShort, Bits: int64(uint16(bits))}
		case "kotlin/UInt":
			return &Value{Kind: ValueConst, Const: ConstUInt, Bits: int64(uint32(bits))}
		case "kotlin/ULong":
			return &Value{Kind: ValueConst, Const: ConstULong, Bits: bits}
		}
		return unsupported
	}
	c, ok := codegen.ConstValue(x)
	if !ok {
		return unsupported
	}
	num := func() (int64, bool) {
		switch c := c.(type) {
		case int64:
			return c, true
		case float64:
			return int64(c), true
		case float32:
			return int64(c), true
		}
		return 0, false
	}
	flt := func() (float64, bool) {
		switch c := c.(type) {
		case float64:
			return c, true
		case float32:
			return float64(c), true
		case int64:
			return float64(c), true
		}
		return 0, false
	}
	integer := func(k ConstKind, conv func(int64) int64) *Value {
		n, ok := num()
		if !ok {
			return unsupported
		}
		return &Value{Kind: ValueConst, Const: k, Bits: conv(n)}
	}
	same := func(n int64) int64 { return n }
	switch t.K {
	case types.String:
		s, ok := c.(string)
		if !ok {
			return unsupported
		}
		return &Value{Kind: ValueConst, Const: ConstString, Str: s}
	case types.Bool:
		return integer(ConstBoolean, same)
	case types.Char:
		return integer(ConstChar, func(n int64) int64 { return int64(uint16(n)) })
	case types.Byte:
		return integer(ConstByte, func(n int64) int64 { return int64(int8(n)) })
	case types.Short:
		return integer(ConstShort, func(n int64) int64 { return int64(int16(n)) })
	case types.Int:
		return integer(ConstInt, func(n int64) int64 { return int64(int32(n)) })
	case types.Long:
		return integer(ConstLong, same)
	case types.Float:
		f, ok := flt()
		if !ok {
			return unsupported
		}
		return &Value{Kind: ValueConst, Const: ConstFloat, Float: float64(float32(f))}
	case types.Double:
		f, ok := flt()
		if !ok {
			return unsupported
		}
		return &Value{Kind: ValueConst, Const: ConstDouble, Float: f}
	}
	return unsupported
}

// isEnumClass reports whether cls is an enum class, of the program or of
// the class path.
func isEnumClass(cls *types.ClassSym) bool {
	if cls.Entries != nil {
		return true
	}
	cls.Ensure()
	return cls.Super != nil && cls.Super.Name == "java/lang/Enum"
}

// defaultValue returns the model of x, the default value of an annotation
// class's parameter of type t, which the checker checked as an ordinary
// expression: arrayOf(..) or [..] for an array, a constructor call for a
// nested annotation.
func (b *fileBuilder) defaultValue(t types.Type, x syntax.Expr, depth int) *Value {
	if x == nil || depth > 16 {
		return &Value{Kind: ValueUnsupported}
	}
	node := b.qualified(b.prov.Node(x))
	if node == 0 {
		node = b.exprNode(x)
	}
	if node != 0 && b.snap != nil {
		// A negative number the checker folded into a literal: -2, whose
		// node is the PREFIX_EXPRESSION.
		if p := b.snap.Node(node).Parent(); p.NumericKind() == psi.KindPREFIX_EXPRESSION && p.Range().End == b.snap.Node(node).Range().End {
			switch b.snap.Node(node).NumericKind() {
			case psi.KindINTEGER_CONSTANT, psi.KindFLOAT_CONSTANT:
				node = p.ID()
			}
		}
	}
	if t.IsArray() || t.K == types.Class && types.IsUnsignedArray(t.C) {
		out := &Value{Kind: ValueArray, Node: node}
		elems, ok := syntax.ArrayElems(x)
		if !ok {
			elems = []syntax.Expr{x}
		}
		for _, e := range syntax.FlattenSpreads(elems) {
			out.Elems = append(out.Elems, b.defaultValue(types.VarargElem(t), e, depth+1))
		}
		return out
	}
	if call, ok := x.(*syntax.Call); ok && call.Ctor != nil && call.Ctor.Retention != types.NoRetention {
		return &Value{Kind: ValueAnnotation, Node: node, Annotation: b.nestedDefault(call, node, depth)}
	}
	v := b.constant(t, x)
	v.Node = node
	return v
}

// nestedDefault returns the annotation call creates in a default value:
// its arguments, in the order written.
func (b *fileBuilder) nestedDefault(call *syntax.Call, node psi.NodeID, depth int) *Annotation {
	a := &Annotation{ClassID: classID(call.Ctor), Node: node}
	if call.Sym == nil {
		return a
	}
	params := call.Sym.Params
	args := make([][]syntax.Expr, len(params))
	var order []int // parameters by their first argument
	for i, arg := range call.Args {
		j := i
		if call.ArgMap != nil {
			j = call.ArgMap[i]
		}
		if j < 0 || j >= len(params) {
			return a
		}
		if args[j] == nil {
			order = append(order, j)
		}
		if sp, ok := arg.(*syntax.Spread); ok && params[j].Vararg {
			if elems, ok := syntax.ArrayElems(sp.X); ok {
				args[j] = append(args[j], elems...)
				continue
			}
		}
		args[j] = append(args[j], arg)
	}
	for _, j := range order {
		p := params[j]
		switch {
		case p.Vararg:
			v := &Value{Kind: ValueArray}
			for _, x := range args[j] {
				v.Elems = append(v.Elems, b.defaultValue(types.VarargElem(p.Type), x, depth+1))
			}
			a.Args = append(a.Args, NamedValue{Name: p.Name, Value: v})
		default:
			a.Args = append(a.Args, NamedValue{Name: p.Name, Value: b.defaultValue(p.Type, args[j][0], depth+1)})
		}
	}
	return a
}

// qualified returns the qualified expression, a.B, whose selector is
// node n, else n: the checker's provenance of a resolved a.B is B's.
func (b *fileBuilder) qualified(n psi.NodeID) psi.NodeID {
	if n == 0 || b.snap == nil {
		return n
	}
	ref := b.snap.Node(n)
	for {
		p := ref.Parent()
		if p.NumericKind() != psi.KindDOT_QUALIFIED_EXPRESSION || p.FirstChild().ID() == ref.ID() {
			return ref.ID()
		}
		ref = p
	}
}

// exprNode returns the psi node of x, an expression re-parsed from an
// annotation argument's tokens, which has no provenance: the outermost
// expression that starts where x does, within its argument.
func (b *fileBuilder) exprNode(x syntax.Expr) psi.NodeID {
	if b.snap == nil {
		return 0
	}
	off, ok := b.offset(startPos(x, 0))
	if !ok {
		return 0
	}
	n := b.snap.NodeAt(off)
	if !n.Valid() || n.Range().Start != off {
		return 0
	}
	for {
		p := n.Parent()
		if !p.Valid() || p.Range().Start != off {
			break
		}
		switch p.NumericKind() {
		case psi.KindVALUE_ARGUMENT, psi.KindVALUE_ARGUMENT_LIST, psi.KindCOLLECTION_LITERAL_EXPRESSION,
			psi.KindANNOTATION_ENTRY, psi.KindVALUE_PARAMETER, psi.KindFILE:
			return expression(n)
		}
		n = p
	}
	return expression(n)
}

// expression returns the node n, if it is an expression, else 0: a leaf
// token, as an operator, is not.
func expression(n psi.NodeRef) psi.NodeID {
	if n.IsLeaf() {
		return 0
	}
	return n.ID()
}

// startPos returns the position x starts at: its leftmost operand's.
func startPos(x syntax.Expr, depth int) syntax.Pos {
	if depth > 64 {
		return x.Pos()
	}
	var in syntax.Expr
	switch x := x.(type) {
	case *syntax.Binary:
		in = x.X
	case *syntax.Member:
		if x.X != nil {
			in = x.X
		}
	case *syntax.Call:
		if x.Fun != nil && !x.Literal {
			in = x.Fun
		}
	case *syntax.Index:
		in = x.X
	case *syntax.ClassLit:
		if x.X != nil {
			in = x.X
		} else if x.Ref != nil {
			if q := x.Ref.P; before(q, x.Pos()) {
				return q
			}
		}
	}
	p := x.Pos()
	if in != nil {
		if q := startPos(in, depth+1); before(q, p) {
			return q
		}
	}
	return p
}

func before(a, b syntax.Pos) bool { return a.Line < b.Line || a.Line == b.Line && a.Col < b.Col }

// offset returns the byte offset of position p, line and column from 1.
func (b *fileBuilder) offset(p syntax.Pos) (int, bool) {
	if b.lines == nil {
		src := b.snap.Source()
		b.lines = []int{0}
		for i := 0; i < len(src); i++ {
			if src[i] == '\n' {
				b.lines = append(b.lines, i+1)
			}
		}
	}
	if p.Line < 1 || p.Line > len(b.lines) || p.Col < 1 {
		return 0, false
	}
	return b.lines[p.Line-1] + p.Col - 1, true
}

// String returns v as Kotlin writes it, for tests and diagnostics.
func (v *Value) String() string {
	if v == nil {
		return "<none>"
	}
	switch v.Kind {
	case ValueConst:
		switch v.Const {
		case ConstBoolean:
			return map[bool]string{true: "true", false: "false"}[v.Bits != 0]
		case ConstChar:
			return "'" + string(rune(v.Bits)) + "'"
		case ConstString:
			return `"` + v.Str + `"`
		case ConstFloat:
			return trimFloat(v.Float) + "f"
		case ConstDouble:
			return trimFloat(v.Float)
		case ConstLong:
			return itoa(v.Bits) + "L"
		case ConstUByte, ConstUShort, ConstUInt:
			return itoa(v.Bits) + "u"
		case ConstULong:
			return utoa(uint64(v.Bits)) + "uL"
		case ConstByte:
			return "b" + itoa(v.Bits)
		case ConstShort:
			return "s" + itoa(v.Bits)
		}
		return itoa(v.Bits)
	case ValueEnum:
		return v.ClassID + "." + v.Entry
	case ValueClass:
		return typeText(v.Type) + "::class"
	case ValueAnnotation:
		return v.Annotation.String()
	case ValueArray:
		parts := make([]string, len(v.Elems))
		for i, e := range v.Elems {
			parts[i] = e.String()
		}
		return "[" + strings.Join(parts, ", ") + "]"
	}
	return "<unsupported>"
}

// String returns a as Kotlin writes it: @a/B(x = 1).
func (a *Annotation) String() string {
	parts := make([]string, len(a.Args))
	for i, arg := range a.Args {
		parts[i] = arg.Name + " = " + arg.Value.String()
	}
	return "@" + a.ClassID + "(" + strings.Join(parts, ", ") + ")"
}

func typeText(t *Type) string {
	if t == nil {
		return "?"
	}
	if t.Kind == TypeClass {
		return t.ClassID
	}
	return t.Name
}

func trimFloat(f float64) string {
	if math.Trunc(f) == f && math.Abs(f) < 1e15 {
		return itoa(int64(f)) + ".0"
	}
	return strconv.FormatFloat(f, 'g', -1, 64)
}

func itoa(n int64) string  { return strconv.FormatInt(n, 10) }
func utoa(n uint64) string { return strconv.FormatUint(n, 10) }

// classLitName returns the class a class literal names as written,
// a.b.C for a.b.C::class; "" for the class of a value.
func classLitName(cl *syntax.ClassLit) string {
	if cl.Ref != nil {
		return cl.Ref.Name
	}
	var parts []string
	for x := cl.X; ; {
		switch e := x.(type) {
		case *syntax.Member:
			parts = append([]string{e.Name}, parts...)
			x = e.X
			continue
		case *syntax.Ident:
			return strings.Join(append([]string{e.Name}, parts...), ".")
		}
		return ""
	}
}

// javaWritten returns the class ID of Java class c by its own name,
// java/lang/Throwable, if name, as written in the file, names it so
// (import java.lang.Throwable, or java.util.Map.Entry spelled out) and
// kc knows it by a Kotlin name; else "". FIR keeps such a class as the
// Java class, which KSP loads by that name.
func (b *fileBuilder) javaWritten(c *types.ClassSym, name string) string {
	if c == nil || name == "" {
		return ""
	}
	pkg, rel := "", c.Name
	if i := strings.LastIndexByte(c.Name, '/'); i >= 0 {
		pkg, rel = c.Name[:i+1], c.Name[i+1:]
	}
	java := pkg + strings.ReplaceAll(rel, "$", ".")
	if java == classID(c) {
		return ""
	}
	fq := name
	first, rest, _ := strings.Cut(name, ".")
	for _, im := range b.f.Imports {
		n := im.Alias
		if n == "" {
			n = im.Path[strings.LastIndexByte(im.Path, '.')+1:]
		}
		if !im.Star && n == first {
			fq = im.Path
			if rest != "" {
				fq += "." + rest
			}
		}
	}
	if fq != strings.ReplaceAll(java, "/", ".") {
		return ""
	}
	return java
}
