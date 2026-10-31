package codegen

import (
	"math"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"

	cf "github.com/barteo/kotlin-compiler-go/internal/classfile"
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// constValue returns the value of const property p, as kotlinc inlines
// it into the code that reads it and writes it as its field's
// ConstantValue, or false if kc can't compute it: then its static
// initializer assigns it, and code reads its field.
func constValue(p *types.VarSym) (any, bool) {
	pd, ok := p.Decl.(*syntax.PropDecl)
	if !ok || !p.Const || p.Type.N {
		return nil, false
	}
	v, ok := constOf(pd.Init, 0)
	if !ok {
		return nil, false
	}
	switch t := p.Type; {
	case t.K == types.String:
		_, ok = v.(string)
	case t.K == types.Double:
		_, ok = v.(float64)
	case t.K == types.Float:
		_, ok = v.(float32)
	case t.K == types.Long || t.IsIntLike():
		_, ok = v.(int64)
	default:
		ok = false
	}
	return v, ok
}

// constPool returns the constant pool entry of the value of const
// property p, or 0 if kc can't compute it.
func constPool(c *cf.Class, p *types.VarSym) uint16 {
	v, ok := constValue(p)
	if !ok {
		return 0
	}
	switch v := v.(type) {
	case string:
		return c.String(v)
	case float64:
		return c.Double(v)
	case float32:
		return c.Float(v)
	case int64:
		if p.Type.K == types.Long {
			return c.Long(v)
		}
		return c.Int(int32(v))
	}
	return 0
}

// constProp pushes the value of const property p: the value itself, as
// kotlinc inlines it, or else its static field.
func (g *gen) constProp(p *types.VarSym) {
	v, ok := constValue(p)
	if !ok && p.Constant != nil {
		v, ok = p.Constant, true // a compiled Kotlin one
	}
	if !ok {
		owner := p.Facade
		if p.Kind != types.Global {
			owner = p.ConstOwner().Name
		}
		g.code.FieldInsn(cf.GETSTATIC, owner, p.Name, desc(p.Type))
		return
	}
	g.pushConst(v, p.Type)
}

// pushConst pushes constant v of type t, as constValue and
// types.VarSym.Constant hold it.
func (g *gen) pushConst(v any, t types.Type) {
	switch v := v.(type) {
	case string:
		g.code.Sconst(v)
	case float64:
		g.code.Dconst(v)
	case float32:
		g.code.Fconst(v)
	case int64:
		if desc(t) == "J" { // a Long or a ULong
			g.code.Lconst(v)
		} else {
			g.code.Iconst(int32(v))
		}
	}
}

// constDecls returns the consts whose fields class cd holds: an
// object's, or its companion object's.
func constDecls(cd *syntax.ClassDecl) []*syntax.PropDecl {
	var out []*syntax.PropDecl
	add := func(d *syntax.ClassDecl) {
		for _, m := range d.Members {
			if pd, ok := m.(*syntax.PropDecl); ok && pd.Sym != nil && pd.Sym.Const {
				out = append(out, pd)
			}
		}
	}
	if cd.Object && (cd.Sym.CompanionOf == nil || interfaceCompanion(cd.Sym)) {
		add(cd) // an interface's companion has them too, as kotlinc's
	}
	for _, m := range cd.Members {
		if ccd, ok := m.(*syntax.ClassDecl); ok && ccd.Sym != nil && ccd.Sym == cd.Sym.Companion {
			add(ccd)
		}
	}
	return out
}

// defaultInit reports whether init, property p's initializer, is the
// default value of p's field, which kotlinc doesn't store: a constant zero,
// false or '\u0000' for a primitive field, but not -0.0, or null.
func defaultInit(p *types.VarSym, init syntax.Expr) bool {
	if d := desc(types.FieldType(p)); len(d) != 1 {
		_, null := init.(*syntax.NullLit)
		return null
	}
	if lit, ok := init.(*syntax.IntLit); ok && lit.Unsigned {
		return lit.Val == 0 // a UInt's or ULong's
	}
	switch v, _ := constOf(init, 0); v := v.(type) {
	case int64:
		return v == 0
	case float64:
		return math.Float64bits(v) == 0
	case float32:
		return math.Float32bits(v) == 0
	}
	return false
}

// constOf computes the value of a constant expression: literals, string
// concatenation, arithmetic on Int and Long, and other const properties.
// Int, Long, Char, Byte, Short and Boolean values are int64s, Doubles
// float64s, Floats float32s and Strings strings. depth guards against a
// const that refers to itself.
func constOf(e syntax.Expr, depth int) (any, bool) {
	if depth > 32 || e == nil {
		return nil, false
	}
	switch e := e.(type) {
	case *syntax.IntLit:
		if e.Unsigned {
			return nil, false
		}
		if e.Type().K == types.Int {
			return int64(int32(e.Val)), true
		}
		return e.Val, true
	case *syntax.DoubleLit:
		return e.Val, true
	case *syntax.FloatLit:
		return e.Val, true
	case *syntax.CharLit:
		return int64(e.Val), true
	case *syntax.BoolLit:
		if e.Val {
			return int64(1), true
		}
		return int64(0), true
	case *syntax.StringLit:
		s := ""
		for _, part := range e.Parts {
			if part.X == nil {
				s += part.Lit
				continue
			}
			v, ok := constOf(part.X, depth+1)
			if !ok {
				return nil, false
			}
			str, ok := constString(v, part.X.Type())
			if !ok {
				return nil, false
			}
			s += str
		}
		return s, true
	case *syntax.Ident:
		if e.Sym == nil && e.Member != nil {
			return constOf(e.Member, depth+1) // an imported member, or this.X
		}
		return constRef(e.Sym, depth)
	case *syntax.Member:
		if e.Prop == nil && e.Name == "length" && !e.Safe && e.X.Type().K == types.String && !e.X.Type().N {
			// A constant String's length, in UTF-16 units: "abc".length.
			if v, ok := constOf(e.X, depth+1); ok {
				if s, ok := v.(string); ok {
					return int64(len(utf16.Encode([]rune(s)))), true
				}
			}
			return nil, false
		}
		if e.Builtin == types.CharCode && !e.Safe {
			return constOf(e.X, depth+1) // a constant Char's code
		}
		if e.Static != nil && e.Field != nil {
			if v, ok := builtinConsts[e.Static.Name+"."+e.Name]; ok {
				return v, true // Char.MAX_VALUE
			}
			return constRef(e.Field, depth) // Activity.RESULT_OK
		}
		return constRef(e.Prop, depth)
	case *syntax.Unary:
		v, ok := constOf(e.X, depth+1)
		if !ok || e.Call != nil {
			return nil, false
		}
		switch e.Op {
		case syntax.MINUS:
			switch v := v.(type) {
			case int64:
				return wrap(-v, e.Type()), true
			case float64:
				return -v, true
			case float32:
				return -v, true
			}
		case syntax.PLUS:
			return v, true
		}
	case *syntax.Binary:
		return constBinary(e, depth)
	case *syntax.Call:
		switch e.Builtin {
		case types.Bitwise:
			return constBitwise(e, depth)
		case types.ToInt, types.ToLong, types.ToShort, types.ToByte, types.ToChar:
			return constConvert(e, depth)
		}
		if s, ok := constTrim(e, depth); ok {
			return s, true
		}
	}
	return nil, false
}

// constTrim computes a call of the standard library's trimIndent() or
// trimMargin() on a constant String, which kotlinc evaluates at compile
// time: """...""".trimIndent() is a constant, as with a const val.
func constTrim(e *syntax.Call, depth int) (string, bool) {
	f := e.Sym
	m, ok := e.Fun.(*syntax.Member)
	if f == nil || !ok || m.Safe || f.Facade != "kotlin/text/StringsKt" || !f.Ext || f.Recv.K != types.String || f.Recv.N {
		return "", false
	}
	v, ok := constOf(m.X, depth+1)
	s, isString := v.(string)
	if !ok || !isString {
		return "", false
	}
	switch {
	case f.Name == "trimIndent" && len(e.Args) == 0:
		return trimIndent(s), true
	case f.Name == "trimMargin" && len(e.Args) <= 1:
		prefix := "|"
		if len(e.Args) == 1 {
			v, ok := constOf(e.Args[0], depth+1)
			if prefix, isString = v.(string); !ok || !isString {
				return "", false
			}
		}
		if blank(prefix) {
			return "", false // trimMargin throws
		}
		return trimMargin(s, prefix), true
	}
	return "", false
}

// trimIndent is Kotlin's String.trimIndent(): the lines without their
// common indent, and without the first and last if they are blank.
func trimIndent(s string) string {
	lines := kotlinLines(s)
	indent := -1 // in chars, of whitespace, which is all in the BMP
	for _, l := range lines {
		if !blank(l) {
			if w := utf8.RuneCountInString(l) - utf8.RuneCountInString(strings.TrimLeftFunc(l, kotlinSpace)); indent < 0 || w < indent {
				indent = w
			}
		}
	}
	return reindent(lines, func(l string) (string, bool) {
		for range indent {
			_, n := utf8.DecodeRuneInString(l)
			l = l[n:]
		}
		return l, true
	})
}

// trimMargin is Kotlin's String.trimMargin(prefix): the lines without
// the whitespace and prefix they start with, and without the first and
// last if they are blank.
func trimMargin(s, prefix string) string {
	return reindent(kotlinLines(s), func(l string) (string, bool) {
		rest := strings.TrimLeftFunc(l, kotlinSpace)
		if rest == "" || !strings.HasPrefix(rest, prefix) {
			return "", false
		}
		return rest[len(prefix):], true
	})
}

// reindent joins lines, cut by cut where it reports true, leaving out the
// first and the last if they are blank, as Kotlin's reindent.
func reindent(lines []string, cut func(string) (string, bool)) string {
	var out []string
	for i, l := range lines {
		if (i == 0 || i == len(lines)-1) && blank(l) {
			continue
		}
		if c, ok := cut(l); ok {
			l = c
		}
		out = append(out, l)
	}
	return strings.Join(out, "\n")
}

// kotlinLines splits s at \r\n, \n and \r, as Kotlin's lines().
func kotlinLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\n':
			lines = append(lines, s[start:i])
			start = i + 1
		case '\r':
			lines = append(lines, s[start:i])
			if i+1 < len(s) && s[i+1] == '\n' {
				i++
			}
			start = i + 1
		}
	}
	return append(lines, s[start:])
}

// blank reports whether s is all whitespace, as Kotlin's isBlank().
func blank(s string) bool { return strings.TrimLeftFunc(s, kotlinSpace) == "" }

// kotlinSpace reports whether r is whitespace to Kotlin on the JVM: to
// Character.isWhitespace or isSpaceChar.
func kotlinSpace(r rune) bool {
	return unicode.In(r, unicode.Zs, unicode.Zl, unicode.Zp) || '\t' <= r && r <= '\r' || 0x1c <= r && r <= 0x1f
}

// constConvert computes an integral constant's conversion to another
// integral type: 2654435769.toInt().
func constConvert(e *syntax.Call, depth int) (any, bool) {
	m, ok := e.Fun.(*syntax.Member)
	if !ok || m.Safe || len(e.Args) != 0 {
		return nil, false
	}
	switch m.X.Type().K {
	case types.Int, types.Long, types.Short, types.Byte, types.Char:
	default:
		return nil, false
	}
	x, ok := constOf(m.X, depth+1)
	if !ok {
		return nil, false
	}
	v := x.(int64)
	switch e.Builtin {
	case types.ToInt:
		return int64(int32(v)), true
	case types.ToShort:
		return int64(int16(v)), true
	case types.ToByte:
		return int64(int8(v)), true
	case types.ToChar:
		return int64(uint16(v)), true
	}
	return v, true
}

// constBitwise computes Int's, Long's or Boolean's bitwise function of
// constants, Booleans being 0 or 1. A
// shift's count is masked to the receiver's width, as the JVM does.
func constBitwise(e *syntax.Call, depth int) (any, bool) {
	m := e.Fun.(*syntax.Member)
	if m.Safe {
		return nil, false
	}
	x, ok := constOf(m.X, depth+1)
	if !ok {
		return nil, false
	}
	a := x.(int64)
	switch m.Name {
	case "inv":
		return wrap(^a, e.Type()), true
	case "not":
		return a ^ 1, true // Boolean's
	}
	y, ok := constOf(e.Args[0], depth+1)
	if !ok {
		return nil, false
	}
	b := y.(int64)
	t := e.Type()
	mask := int64(63)
	if t.K == types.Int {
		mask = 31
	}
	switch m.Name {
	case "and":
		return a & b, true
	case "or":
		return a | b, true
	case "xor":
		return a ^ b, true
	case "shl":
		return wrap(a<<(b&mask), t), true
	case "shr":
		return a >> (b & mask), true // a Long, or an Int sign-extended to one
	case "ushr":
		if t.K == types.Int {
			return int64(int32(uint32(a) >> (b & mask))), true
		}
		return int64(uint64(a) >> (b & mask)), true
	}
	return nil, false
}

// builtinConsts are the constants of the built-in types' companion
// objects, which constant expressions can use.
var builtinConsts = map[string]any{
	"kotlin/jvm/internal/IntCompanionObject.MIN_VALUE":           int64(-1 << 31),
	"kotlin/jvm/internal/IntCompanionObject.MAX_VALUE":           int64(1<<31 - 1),
	"kotlin/jvm/internal/IntCompanionObject.SIZE_BYTES":          int64(4),
	"kotlin/jvm/internal/IntCompanionObject.SIZE_BITS":           int64(32),
	"kotlin/jvm/internal/LongCompanionObject.MIN_VALUE":          int64(-1 << 63),
	"kotlin/jvm/internal/LongCompanionObject.MAX_VALUE":          int64(1<<63 - 1),
	"kotlin/jvm/internal/LongCompanionObject.SIZE_BYTES":         int64(8),
	"kotlin/jvm/internal/LongCompanionObject.SIZE_BITS":          int64(64),
	"kotlin/jvm/internal/ShortCompanionObject.MIN_VALUE":         int64(-1 << 15),
	"kotlin/jvm/internal/ShortCompanionObject.MAX_VALUE":         int64(1<<15 - 1),
	"kotlin/jvm/internal/ShortCompanionObject.SIZE_BYTES":        int64(2),
	"kotlin/jvm/internal/ShortCompanionObject.SIZE_BITS":         int64(16),
	"kotlin/jvm/internal/ByteCompanionObject.MIN_VALUE":          int64(-1 << 7),
	"kotlin/jvm/internal/ByteCompanionObject.MAX_VALUE":          int64(1<<7 - 1),
	"kotlin/jvm/internal/ByteCompanionObject.SIZE_BYTES":         int64(1),
	"kotlin/jvm/internal/ByteCompanionObject.SIZE_BITS":          int64(8),
	"kotlin/jvm/internal/CharCompanionObject.MIN_VALUE":          int64(0),
	"kotlin/jvm/internal/CharCompanionObject.MAX_VALUE":          int64(0xffff),
	"kotlin/jvm/internal/CharCompanionObject.SIZE_BYTES":         int64(2),
	"kotlin/jvm/internal/CharCompanionObject.SIZE_BITS":          int64(16),
	"kotlin/jvm/internal/CharCompanionObject.MIN_HIGH_SURROGATE": int64(0xd800),
	"kotlin/jvm/internal/CharCompanionObject.MAX_HIGH_SURROGATE": int64(0xdbff),
	"kotlin/jvm/internal/CharCompanionObject.MIN_LOW_SURROGATE":  int64(0xdc00),
	"kotlin/jvm/internal/CharCompanionObject.MAX_LOW_SURROGATE":  int64(0xdfff),
	"kotlin/jvm/internal/CharCompanionObject.MIN_SURROGATE":      int64(0xd800),
	"kotlin/jvm/internal/CharCompanionObject.MAX_SURROGATE":      int64(0xdfff),
}

// constRef returns the value of const property p, or of a constant
// field from the class path.
func constRef(p *types.VarSym, depth int) (any, bool) {
	if p != nil && p.Constant != nil {
		return p.Constant, true // a Java constant, or a compiled Kotlin one
	}
	if p == nil || !p.Const {
		return nil, false
	}
	pd, ok := p.Decl.(*syntax.PropDecl)
	if !ok {
		return nil, false
	}
	return constOf(pd.Init, depth+1)
}

func constBinary(e *syntax.Binary, depth int) (any, bool) {
	if e.Call != nil {
		return nil, false
	}
	x, ok := constOf(e.X, depth+1)
	if !ok {
		return nil, false
	}
	y, ok := constOf(e.Y, depth+1)
	if !ok {
		return nil, false
	}
	if e.Op == syntax.PLUS && e.X.Type().K == types.String {
		s, ok := constString(y, e.Y.Type())
		if !ok {
			return nil, false
		}
		return x.(string) + s, true
	}
	a, ok1 := x.(int64)
	b, ok2 := y.(int64)
	t := e.Type()
	if !ok1 || !ok2 || t.K != types.Int && t.K != types.Long {
		return nil, false
	}
	switch e.Op {
	case syntax.PLUS:
		return wrap(a+b, t), true
	case syntax.MINUS:
		return wrap(a-b, t), true
	case syntax.STAR:
		return wrap(a*b, t), true
	case syntax.SLASH:
		if b != 0 {
			return wrap(a/b, t), true // Go truncates, and wraps MIN_VALUE / -1, as the JVM does
		}
	case syntax.PERCENT:
		if b != 0 {
			return a % b, true
		}
	}
	return nil, false
}

// wrap truncates an Int's value to 32 bits.
func wrap(v int64, t types.Type) int64 {
	if t.K == types.Int {
		return int64(int32(v))
	}
	return v
}

// constString returns a constant as a string template shows it, for the
// types whose text kc computes as the JVM does.
func constString(v any, t types.Type) (string, bool) {
	switch t.K {
	case types.String:
		s, ok := v.(string)
		return s, ok
	case types.Int, types.Long, types.Short, types.Byte:
		i, ok := v.(int64)
		return strconv.FormatInt(i, 10), ok
	case types.Char:
		i, ok := v.(int64)
		return string(rune(i)), ok && (i < 0xD800 || i > 0xDFFF)
	case types.Bool:
		i, ok := v.(int64)
		return strconv.FormatBool(i != 0), ok
	}
	return "", false
}
