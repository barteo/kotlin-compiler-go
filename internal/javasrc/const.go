package javasrc

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/intbits"
)

// A cval is the value of a constant expression, of a primitive type or
// String: its kind is the type's descriptor, or 'T' for a String.
type cval struct {
	k byte
	i int64 // Z, C, B, S, I and J
	f float64
	s string
}

// constant returns a static final field's constant value as its
// ConstantValue attribute holds it: an int32, int64, float32, float64 or
// string, or nil if its initializer isn't a constant expression. seen
// holds the fields whose values are being computed, which a cycle of
// references isn't constant through.
func (s *Set) constant(c *Class, f *Field, seen map[*Field]bool) any {
	v, ok := s.fieldValue(c, f, seen)
	if !ok {
		return nil
	}
	return v.attr()
}

func (s *Set) fieldValue(c *Class, f *Field, seen map[*Field]bool) (cval, bool) {
	k := fieldKind(f.Type)
	if k == 0 || seen[f] || len(f.Init) == 0 {
		return cval{}, false
	}
	if seen == nil {
		seen = map[*Field]bool{}
	}
	seen[f] = true
	defer delete(seen, f)
	e := &evaluator{s: s, r: &resolver{s: s, c: c}, toks: joinOps(f.Init), seen: seen}
	v, ok := e.expr()
	if !ok || e.pos != len(e.toks) {
		return cval{}, false
	}
	return assign(v, k)
}

// fieldKind returns the kind of a field's value, if it can be a constant:
// a primitive type's or String's.
func fieldKind(t *Type) byte {
	if t.Dims > 0 {
		return 0
	}
	if t.Prim != "" {
		return primDesc[t.Prim][0]
	}
	if n := len(t.Segs); n == 1 && t.Segs[0].Name == "String" ||
		n == 3 && t.Segs[0].Name == "java" && t.Segs[1].Name == "lang" && t.Segs[2].Name == "String" {
		return 'T'
	}
	return 0
}

// attr returns the value as a ConstantValue attribute holds it.
func (v cval) attr() any {
	switch v.k {
	case 'Z', 'C', 'B', 'S', 'I':
		// convert and fromAttr keep these within an int.
		if v.i < math.MinInt32 || v.i > math.MaxInt32 {
			panic(fmt.Sprintf("javasrc: %c constant %d is out of int range", v.k, v.i))
		}
		return int32(v.i)
	case 'J':
		return v.i
	case 'F':
		return float32(v.f)
	case 'D':
		return v.f
	case 'T':
		return v.s
	}
	return nil
}

// fromAttr returns the value of a field with descriptor desc whose
// ConstantValue is a.
func fromAttr(a any, desc string) (cval, bool) {
	switch v := a.(type) {
	case int32:
		if len(desc) == 1 && strings.Contains("ZCBSI", desc) {
			return cval{k: desc[0], i: int64(v)}, true
		}
	case int64:
		return cval{k: 'J', i: v}, true
	case float32:
		return cval{k: 'F', f: float64(v)}, true
	case float64:
		return cval{k: 'D', f: v}, true
	case string:
		return cval{k: 'T', s: v}, true
	}
	return cval{}, false
}

func integral(k byte) bool { return strings.IndexByte("CBSIJ", k) >= 0 }
func numeric(k byte) bool  { return strings.IndexByte("CBSIJFD", k) >= 0 }

// convert casts a value to a primitive type or String.
func convert(v cval, k byte) (cval, bool) {
	if k == 'T' || v.k == 'T' || k == 'Z' || v.k == 'Z' {
		return v, v.k == k
	}
	var i int64
	var f float64
	if v.k == 'F' || v.k == 'D' {
		f = v.f
		switch {
		case math.IsNaN(f):
			i = 0
		case k == 'J' && f >= math.MaxInt64:
			i = math.MaxInt64
		case k == 'J' && f <= math.MinInt64:
			i = math.MinInt64
		case k != 'J' && f >= math.MaxInt32:
			i = math.MaxInt32
		case k != 'J' && f <= math.MinInt32:
			i = math.MinInt32
		default:
			i = int64(f)
		}
	} else {
		i, f = v.i, float64(v.i)
	}
	switch k {
	case 'B':
		return cval{k: k, i: intbits.SignExtend(i, 8)}, true
	case 'S':
		return cval{k: k, i: intbits.SignExtend(i, 16)}, true
	case 'C':
		return cval{k: k, i: intbits.ZeroExtend(i, 16)}, true
	case 'I':
		return cval{k: k, i: intbits.SignExtend(i, 32)}, true
	case 'J':
		return cval{k: k, i: i}, true
	case 'F':
		return cval{k: k, f: float64(float32(f))}, true
	case 'D':
		return cval{k: k, f: f}, true
	}
	return cval{}, false
}

// assign converts a value to a field's type, as an initializer does: a
// constant int narrows to a byte, short or char.
func assign(v cval, k byte) (cval, bool) {
	if v.k == 'T' || k == 'T' || v.k == 'Z' || k == 'Z' {
		return v, v.k == k
	}
	if (v.k == 'F' || v.k == 'D') && integral(k) || v.k == 'D' && k == 'F' || v.k == 'J' && k != 'J' && integral(k) {
		return cval{}, false
	}
	return convert(v, k)
}

// promote returns the type two operands of a numeric operator are
// converted to.
func promote(a, b byte) byte {
	switch {
	case a == 'D' || b == 'D':
		return 'D'
	case a == 'F' || b == 'F':
		return 'F'
	case a == 'J' || b == 'J':
		return 'J'
	}
	return 'I'
}

// str returns a value's text in a string concatenation, or false for a
// floating-point value, whose Java text isn't Go's.
func (v cval) str() (string, bool) {
	switch v.k {
	case 'T':
		return v.s, true
	case 'Z':
		return strconv.FormatBool(v.i != 0), true
	case 'C':
		// convert keeps a char within 16 bits.
		if v.i < 0 || v.i > math.MaxUint16 {
			return "", false
		}
		return string(rune(v.i)), true
	case 'B', 'S', 'I', 'J':
		return strconv.FormatInt(v.i, 10), true
	}
	return "", false
}

// joinOps joins the one-character punctuation tokens of an operator that
// are next to each other: << and >>> and the like.
func joinOps(toks []token) []token {
	ops := []string{">>>", "<<", ">>", "<=", ">=", "==", "!=", "&&", "||"}
	var out []token
	for i := 0; i < len(toks); i++ {
		t := toks[i]
		if t.kind == tPunct {
			for _, op := range ops {
				n := len(op)
				if i+n > len(toks) {
					continue
				}
				match := true
				for j := range n {
					u := toks[i+j]
					if u.kind != tPunct || u.text != op[j:j+1] || u.line != t.line || u.col != t.col+j {
						match = false
						break
					}
				}
				if match {
					t.text = op
					i += n - 1
					break
				}
			}
		}
		out = append(out, t)
	}
	return out
}

// evaluator computes a constant expression, as javac folds them.
type evaluator struct {
	s    *Set
	r    *resolver
	toks []token
	pos  int
	seen map[*Field]bool
}

func (e *evaluator) tok() token {
	if e.pos < len(e.toks) {
		return e.toks[e.pos]
	}
	return token{kind: tEOF}
}

func (e *evaluator) is(s string) bool { t := e.tok(); return t.kind == tPunct && t.text == s }

var precedence = map[string]int{
	"||": 1, "&&": 2, "|": 3, "^": 4, "&": 5, "==": 6, "!=": 6,
	"<": 7, ">": 7, "<=": 7, ">=": 7, "<<": 8, ">>": 8, ">>>": 8,
	"+": 9, "-": 9, "*": 10, "/": 10, "%": 10,
}

func (e *evaluator) expr() (cval, bool) {
	c, ok := e.binary(1)
	if !ok || !e.is("?") {
		return c, ok
	}
	e.pos++
	a, ok1 := e.expr()
	if !e.is(":") {
		return cval{}, false
	}
	e.pos++
	b, ok2 := e.expr()
	if !ok1 || !ok2 || c.k != 'Z' {
		return cval{}, false
	}
	// The result has the operands' type, promoted if numeric.
	if a.k != b.k {
		if !numeric(a.k) || !numeric(b.k) {
			return cval{}, false
		}
		k := promote(a.k, b.k)
		a, _ = convert(a, k)
		b, _ = convert(b, k)
	}
	if c.i != 0 {
		return a, true
	}
	return b, true
}

func (e *evaluator) binary(min int) (cval, bool) {
	x, ok := e.unary()
	if !ok {
		return cval{}, false
	}
	for {
		t := e.tok()
		prec := precedence[t.text]
		if t.kind != tPunct || prec < min || prec == 0 {
			return x, true
		}
		e.pos++
		y, ok := e.binary(prec + 1)
		if !ok {
			return cval{}, false
		}
		if x, ok = binop(t.text, x, y); !ok {
			return cval{}, false
		}
	}
}

func (e *evaluator) unary() (cval, bool) {
	t := e.tok()
	if t.kind == tPunct {
		switch t.text {
		case "+", "-", "~", "!":
			e.pos++
			x, ok := e.unary()
			if !ok {
				return cval{}, false
			}
			return unop(t.text, x)
		case "(":
			// A cast to a primitive type or String, or parentheses.
			if n := e.toks[e.pos+1:]; len(n) >= 2 && n[0].kind == tIdent && n[1].text == ")" {
				k := byte(0)
				if d, ok := primDesc[n[0].text]; ok {
					k = d[0]
				} else if n[0].text == "String" {
					k = 'T'
				}
				if k != 0 {
					e.pos += 3
					x, ok := e.unary()
					if !ok {
						return cval{}, false
					}
					return convert(x, k)
				}
			}
			e.pos++
			x, ok := e.expr()
			if !ok || !e.is(")") {
				return cval{}, false
			}
			e.pos++
			return x, true
		}
		return cval{}, false
	}
	e.pos++
	switch t.kind {
	case tNumber:
		return number(t.text)
	case tString:
		return cval{k: 'T', s: t.text}, true
	case tChar:
		n, _ := strconv.Atoi(t.text)
		return cval{k: 'C', i: int64(n)}, true
	case tIdent:
		switch t.text {
		case "true":
			return cval{k: 'Z', i: 1}, true
		case "false":
			return cval{k: 'Z'}, true
		}
		names := []string{t.text}
		for e.is(".") && e.pos+1 < len(e.toks) && e.toks[e.pos+1].kind == tIdent {
			names = append(names, e.toks[e.pos+1].text)
			e.pos += 2
		}
		return e.name(names)
	}
	return cval{}, false
}

// name returns the value of a constant field named X or C.X or p.C.X.
func (e *evaluator) name(names []string) (cval, bool) {
	field := names[len(names)-1]
	if len(names) == 1 {
		// A field of the class or an enclosing one, or one a static
		// import names.
		for k := e.r.c; k != nil; k = k.Outer {
			if v, ok, found := e.field(internalName(k), field); found {
				return v, ok
			}
		}
		for _, imp := range e.r.c.file.Imports {
			if !imp.Static {
				continue
			}
			if !imp.Star && strings.HasSuffix(imp.Name, "."+field) {
				if v, ok, found := e.field(e.s.qualified(imp.Name[:len(imp.Name)-len(field)-1]), field); found {
					return v, ok
				}
			}
			if imp.Star {
				if v, ok, found := e.field(e.s.qualified(imp.Name), field); found {
					return v, ok
				}
			}
		}
		return cval{}, false
	}
	t := &Type{}
	for _, n := range names[:len(names)-1] {
		t.Segs = append(t.Segs, Seg{Name: n})
	}
	class, tp := e.r.classType(t)
	if tp != nil || class == "" {
		return cval{}, false
	}
	v, ok, _ := e.field(class, field)
	return v, ok
}

// field returns the value of a class's constant field, and reports
// whether the class has the field.
func (e *evaluator) field(class, name string) (cval, bool, bool) {
	return e.inheritedField(class, name, 0)
}

// inheritedField returns the value of a constant field that a class
// declares or inherits.
func (e *evaluator) inheritedField(class, name string, depth int) (cval, bool, bool) {
	if depth > 16 {
		return cval{}, false, false
	}
	if v, ok, found := e.declaredField(class, name); found {
		return v, ok, true
	}
	var supers []string
	if c := e.s.classes[class]; c != nil {
		sr := superResolver(e.s, c)
		for _, t := range append(append([]*Type{}, c.Extends...), c.Implements...) {
			if n, tp := sr.classType(t); tp == nil {
				supers = append(supers, n)
			}
		}
	} else if e.s.Outside != nil {
		supers, _ = e.s.Outside.Supers(class)
	}
	for _, super := range supers {
		if v, ok, found := e.inheritedField(super, name, depth+1); found {
			return v, ok, true
		}
	}
	return cval{}, false, false
}

func (e *evaluator) declaredField(class, name string) (cval, bool, bool) {
	if c := e.s.classes[class]; c != nil {
		for _, f := range c.Fields {
			if f.Name != name {
				continue
			}
			if f.Flags&0x0018 != 0x0018 && c.Kind != KindInterface && c.Kind != KindAnnotation {
				return cval{}, false, true // not static final
			}
			v, ok := e.s.fieldValue(c, f, e.seen)
			return v, ok, true
		}
		return cval{}, false, false
	}
	if e.s.Outside == nil {
		return cval{}, false, false
	}
	a, desc := e.s.Outside.Constant(class, name)
	if desc == "" {
		return cval{}, false, false
	}
	v, ok := fromAttr(a, desc)
	return v, ok, true
}

func unop(op string, x cval) (cval, bool) {
	switch op {
	case "!":
		if x.k != 'Z' {
			return cval{}, false
		}
		return cval{k: 'Z', i: 1 - x.i}, true
	case "~":
		if !integral(x.k) {
			return cval{}, false
		}
		x, _ = convert(x, promote(x.k, 'I'))
		return convert(cval{k: x.k, i: ^x.i}, x.k)
	}
	if !numeric(x.k) {
		return cval{}, false
	}
	x, _ = convert(x, promote(x.k, 'I'))
	if op == "-" {
		x.i, x.f = -x.i, -x.f
		return convert(x, x.k)
	}
	return x, true
}

func binop(op string, x, y cval) (cval, bool) {
	if op == "+" && (x.k == 'T' || y.k == 'T') {
		a, ok1 := x.str()
		b, ok2 := y.str()
		return cval{k: 'T', s: a + b}, ok1 && ok2
	}
	switch op {
	case "&&", "||":
		if x.k != 'Z' || y.k != 'Z' {
			return cval{}, false
		}
		if op == "&&" {
			return cval{k: 'Z', i: x.i & y.i}, true
		}
		return cval{k: 'Z', i: x.i | y.i}, true
	case "&", "|", "^":
		if x.k == 'Z' && y.k == 'Z' {
			return cval{k: 'Z', i: bitop(op, x.i, y.i)}, true
		}
	case "==", "!=":
		if x.k == 'Z' && y.k == 'Z' || x.k == 'T' && y.k == 'T' {
			eq := x.i == y.i && x.s == y.s
			return boolVal(eq == (op == "==")), true
		}
	case "<<", ">>", ">>>":
		if !integral(x.k) || !integral(y.k) {
			return cval{}, false
		}
		x, _ = convert(x, promote(x.k, 'I'))
		n := uint(y.i & 31)
		if x.k == 'J' {
			n = uint(y.i & 63)
		}
		switch op {
		case "<<":
			x.i <<= n
		case ">>":
			x.i >>= n
		default:
			if x.k == 'J' {
				x.i = int64(uint64(x.i) >> n)
			} else {
				x.i = intbits.ZeroExtend(x.i, 32) >> n
			}
		}
		return convert(x, x.k)
	}
	if !numeric(x.k) || !numeric(y.k) {
		return cval{}, false
	}
	k := promote(x.k, y.k)
	x, _ = convert(x, k)
	y, _ = convert(y, k)
	if k == 'F' || k == 'D' {
		a, b := x.f, y.f
		var r float64
		switch op {
		case "+":
			r = a + b
		case "-":
			r = a - b
		case "*":
			r = a * b
		case "/":
			r = a / b
		case "%":
			r = math.Mod(a, b)
		case "<":
			return boolVal(a < b), true
		case ">":
			return boolVal(a > b), true
		case "<=":
			return boolVal(a <= b), true
		case ">=":
			return boolVal(a >= b), true
		case "==":
			return boolVal(a == b), true
		case "!=":
			return boolVal(a != b), true
		default:
			return cval{}, false
		}
		return convert(cval{k: k, f: r}, k)
	}
	a, b := x.i, y.i
	var r int64
	switch op {
	case "+":
		r = a + b
	case "-":
		r = a - b
	case "*":
		r = a * b
	case "/", "%":
		if b == 0 {
			return cval{}, false
		}
		if op == "/" {
			r = a / b
		} else {
			r = a % b
		}
	case "&", "|", "^":
		r = bitop(op, a, b)
	case "<":
		return boolVal(a < b), true
	case ">":
		return boolVal(a > b), true
	case "<=":
		return boolVal(a <= b), true
	case ">=":
		return boolVal(a >= b), true
	case "==":
		return boolVal(a == b), true
	case "!=":
		return boolVal(a != b), true
	default:
		return cval{}, false
	}
	return convert(cval{k: k, i: r}, k)
}

func bitop(op string, a, b int64) int64 {
	switch op {
	case "&":
		return a & b
	case "|":
		return a | b
	}
	return a ^ b
}

func boolVal(b bool) cval {
	if b {
		return cval{k: 'Z', i: 1}
	}
	return cval{k: 'Z'}
}

// number parses a numeric literal.
func number(s string) (cval, bool) {
	s = strings.ToLower(strings.ReplaceAll(s, "_", ""))
	hex := strings.HasPrefix(s, "0x")
	if hex && strings.ContainsAny(s, ".p") || !hex && (strings.ContainsAny(s, ".e") || strings.HasSuffix(s, "f") || strings.HasSuffix(s, "d")) {
		k, bits := byte('D'), 64
		if strings.HasSuffix(s, "f") {
			k, bits = 'F', 32
		}
		s = strings.TrimRight(s, "fd")
		if hex && !strings.Contains(s, "p") {
			return cval{}, false
		}
		f, err := strconv.ParseFloat(s, bits)
		if err != nil {
			return cval{}, false
		}
		return cval{k: k, f: f}, true
	}
	k := byte('I')
	if t, ok := strings.CutSuffix(s, "l"); ok {
		k, s = 'J', t
	}
	base := 10
	switch {
	case hex:
		base, s = 16, s[2:]
	case strings.HasPrefix(s, "0b"):
		base, s = 2, s[2:]
	case len(s) > 1 && s[0] == '0':
		base, s = 8, s[1:]
	}
	u, err := strconv.ParseUint(s, base, 64)
	if err != nil {
		return cval{}, false
	}
	// A decimal int literal is at most 2147483648, as in -2147483648; the
	// others are bits.
	if k == 'I' && (base == 10 && u > 1<<31 || base != 10 && u > math.MaxUint32) ||
		k == 'J' && base == 10 && u > 1<<63 {
		return cval{}, false
	}
	i := intbits.FromBits(u)
	if k == 'I' {
		i = intbits.SignExtend(i, 32)
	}
	return cval{k: k, i: i}, true
}
