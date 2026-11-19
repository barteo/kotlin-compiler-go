package check

import (
	"cmp"
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// Annotation arguments and the default values of annotation parameters
// must be compile-time constants, which kotlinc evaluates: literals,
// const vals, enum entries, class literals, annotations, arrays of
// constants, and operators and the standard library's intrinsic
// functions on constant primitives and strings. kc doesn't evaluate them
// here: it only rejects what can't be constant, and accepts what it isn't
// sure of.

// nonConst returns the part of x, checked, that isn't a compile-time
// constant: x, or an element of an array, or nil if x may be constant.
func (c *Checker) nonConst(x syntax.Expr) syntax.Expr {
	return nonConst(x, 0)
}

// constInit reports the initializer of const property d if it isn't a
// compile-time constant, as kotlinc's CONST_VAL_WITH_NON_CONST_INITIALIZER
// and NON_CONST_VAL_USED_IN_CONSTANT_EXPRESSION.
func (c *Checker) constInit(d *syntax.PropDecl) {
	if !d.Const || d.Sym == nil || d.DelegateOf != nil || d.Mutable {
		return
	}
	if t := d.Sym.Type; t.K != types.Invalid && (!constType(t) || t.N || t.K == types.Null || t.K == types.Nothing) {
		c.errorf(modPos(d.Written, "const", d.P), "const 'val' has type '%s'. Only primitive types and String are allowed", t)
		d.Sym.ConstBad = true
		return
	}
	if d.Init == nil {
		if d.Field == nil && d.Getter == nil && d.AccessorP.Line == 0 && !d.Expect && !d.External {
			c.errorf(modPos(d.Written, "const", d.P), "const 'val' should have an initializer")
		}
		return
	}
	if d.Init.Type().K == types.Invalid {
		return
	}
	bad := nonConst(d.Init, 0)
	if bad == nil {
		var path []*types.VarSym
		if constCycle(d.Sym, d.Init, map[*types.VarSym]bool{d.Sym: true}, 0, &path) {
			bad = d.Init // const val a = b + 1 with const val b = a + 1
			// The other const vals of the cycle, checked before this
			// one, are reported too, if they are in this file.
			for _, v := range path {
				if pd, ok := v.Decl.(*syntax.PropDecl); ok && pd != d && !v.ConstBad && pd.Init != nil && propIn(c.file.Decls, pd) {
					v.ConstBad = true
					c.errorf(pd.Init.Pos(), "const 'val' initializer should be a constant value")
				}
			}
		}
	}
	if bad != nil {
		d.Sym.ConstBad = true
	}
	switch {
	case bad == nil:
	case nonConstRef(bad):
		c.errorf(bad.Pos(), "only 'const val' can be used in constant expressions")
	default:
		c.errorf(d.Init.Pos(), "const 'val' initializer should be a constant value")
	}
}

// nonConstRef reports whether x, not a constant, reads a variable or
// property that isn't a const val.
func nonConstRef(x syntax.Expr) bool {
	switch x := x.(type) {
	case *syntax.Ident:
		return x.Sym != nil && !constVar(x.Sym)
	case *syntax.Member:
		return x.Prop != nil && !constVar(x.Prop)
	}
	return false
}

// constCycle reports whether the const vals that x reads, and those
// their initializers read, read the one being checked, in seen.
func constCycle(self *types.VarSym, x syntax.Expr, seen map[*types.VarSym]bool, depth int, path *[]*types.VarSym) bool {
	found := false
	syntax.Inspect(x, func(n any) bool {
		if found || depth > 64 {
			return false
		}
		var v *types.VarSym
		switch n := n.(type) {
		case *syntax.Ident:
			v = n.Sym
		case *syntax.Member:
			v = cmp.Or(n.Prop, n.Field)
		case *syntax.Lambda, *syntax.ObjectExpr:
			return false
		}
		if v == nil || !v.Const {
			return true
		}
		if v == self {
			found = true
			return false
		}
		if seen[v] {
			return true
		}
		seen[v] = true
		if pd, ok := v.Decl.(*syntax.PropDecl); ok && pd.Init != nil && pd.Init.Type().K != types.Invalid {
			found = constCycle(self, pd.Init, seen, depth+1, path)
			if found {
				*path = append(*path, v)
			}
		}
		return !found
	})
	return found
}

func nonConst(x syntax.Expr, depth int) syntax.Expr {
	if x == nil || depth > 64 {
		return nil
	}
	switch x := x.(type) {
	case *syntax.IntLit, *syntax.DoubleLit, *syntax.FloatLit, *syntax.CharLit, *syntax.BoolLit, *syntax.NullLit:
		return nil
	case *syntax.StringLit:
		for _, p := range x.Parts {
			if p.X != nil && (!constType(p.X.Type()) || nonConst(p.X, depth+1) != nil) {
				return x // "${A::class}", "$local"
			}
		}
		return nil
	case *syntax.Ident:
		switch {
		case x.Sym != nil:
			if !constVar(x.Sym) || x.Sym.ConstBad {
				return x // const val copy = bad, with bad's initializer not constant
			}
		case x.Member != nil: // an imported member, or this.X
			if nonConst(x.Member, depth+1) != nil {
				return x
			}
		}
		return nil
	case *syntax.Member:
		switch {
		case x.Prop != nil:
			if !constVar(x.Prop) || x.Prop.ConstBad {
				return x
			}
			if id, ok := x.X.(*syntax.Ident); ok && id.Sym != nil {
				return x // a const read through a variable: v.CONST
			}
		case x.Field != nil:
			if !constVar(x.Field) || x.Field.ConstBad {
				return x
			}
		case x.Static != nil || x.Object != nil:
		case x.Builtin != 0:
			// "abc".length, 'a'.code
			if x.Safe || !constType(x.X.Type()) || nonConst(x.X, depth+1) != nil {
				return x
			}
		case x.Getter != nil:
			return x // a Java getter
		}
		return nil
	case *syntax.Unary:
		switch x.Op {
		case syntax.INC, syntax.DEC:
			return x
		}
		if !constType(x.X.Type()) || nonConst(x.X, depth+1) != nil {
			return x
		}
		return nil
	case *syntax.Binary:
		switch x.Op {
		case syntax.RANGE, syntax.RANGE_LT, syntax.ELVIS:
			return x
		}
		// "1" + MyEnum.A, A::class == B::class
		if !constType(x.X.Type()) || !constType(x.Y.Type()) {
			return x
		}
		if r := nonConst(x.X, depth+1); r != nil {
			return r
		}
		if r := nonConst(x.Y, depth+1); r != nil {
			return r
		}
		xt, yt := x.X.Type(), x.Y.Type()
		if x.Op == syntax.PLUS && xt.K == types.String && !xt.N {
			if yt.K == types.Null || yt.K == types.Nothing {
				return x // "1" + null
			}
			return nil // "1" + 2u
		}
		if isUnsigned(xt) || isUnsigned(yt) || xt.K == types.Null || yt.K == types.Null || xt.K == types.Nothing || yt.K == types.Nothing {
			return x // 1u + 2u, which kotlinc doesn't evaluate
		}
		if x.Call != nil && x.Call.Sym != nil && x.Call.Sym.Decl != nil {
			return x // an operator function of the program's
		}
		if (x.Op == syntax.SLASH || x.Op == syntax.PERCENT) && integralConst(xt) && integralConst(yt) && zeroDivisor(x.Y) {
			return x // 1 / 0, which kotlinc can't evaluate
		}
		return nil
	case *syntax.Call:
		return nonConstCall(x, depth)
	case *syntax.ClassLit:
		if x.Bound || x.Class.IsParam() {
			return x // v::class, T::class
		}
		return nil
	case *syntax.Spread:
		return nonConst(x.X, depth+1)
	case *syntax.Checked:
		return nonConst(x.X, depth+1)
	case *syntax.If, *syntax.When, *syntax.Lambda, *syntax.ObjectExpr, *syntax.This, *syntax.Try,
		*syntax.PropRef, *syntax.Index, *syntax.Is, *syntax.As, *syntax.NotNull:
		return x
	}
	return nil
}

// nonConstCall is nonConst of a call: an array of constants, an
// annotation, or a call of the standard library on a constant primitive
// or string, such as 1.toLong() or "a".trimIndent(), may be constant.
func nonConstCall(x *syntax.Call, depth int) syntax.Expr {
	args := func() syntax.Expr {
		for _, a := range x.Args {
			if r := nonConst(a, depth+1); r != nil {
				return r
			}
		}
		return nil
	}
	switch {
	case x.Invoke || x.SAM || x.Recv:
		return x
	case syntax.IsArrayOf(x):
		return args() // each element: arrayOf(a, *b, elements = c)
	case x.Ctor != nil:
		if x.Ctor.Retention == types.NoRetention {
			return x
		}
		return args() // an annotation, A(1)
	}
	m, ok := x.Fun.(*syntax.Member)
	if !ok || m.Static != nil || m.Safe {
		if x.Builtin != 0 {
			return args()
		}
		return x // f(), a function's result
	}
	if !constType(m.X.Type()) || x.Sym != nil && x.Sym.Decl != nil {
		return x // B::class.toString(), or a function of the program
	}
	if isUnsigned(m.X.Type()) {
		return x // 1u.toInt(), which kotlinc doesn't evaluate
	}
	if m.Written {
		switch m.Name {
		case "inc", "dec":
			return x // 1.inc(), which kotlinc doesn't evaluate
		case "div", "rem":
			if len(x.Args) == 1 && integralConst(m.X.Type()) && integralConst(x.Args[0].Type()) && zeroDivisor(x.Args[0]) {
				return x // 0.div(0)
			}
		}
	}
	if r := nonConst(m.X, depth+1); r != nil {
		return x
	}
	if r := args(); r != nil {
		return r
	}
	if x.Sym != nil && x.Sym.Name == "trimMargin" && len(x.Args) == 1 {
		// kotlinc can't evaluate it with a blank prefix.
		if s, ok := constString(x.Args[0], depth+1); ok && strings.TrimSpace(s) == "" {
			return x
		}
	}
	return nil
}

// constString returns the value of a String literal without templates,
// or of a const val initialized with one.
func constString(x syntax.Expr, depth int) (string, bool) {
	if depth > 64 {
		return "", false
	}
	switch x := x.(type) {
	case *syntax.StringLit:
		var b strings.Builder
		for _, p := range x.Parts {
			if p.X != nil {
				return "", false
			}
			b.WriteString(p.Lit)
		}
		return b.String(), true
	case *syntax.Binary:
		if x.Op == syntax.PLUS {
			l, ok1 := constString(x.X, depth+1)
			r, ok2 := constString(x.Y, depth+1)
			if ok1 && ok2 {
				return l + r, true
			}
		}
	case *syntax.Ident:
		if v := x.Sym; v != nil {
			if s, ok := v.Constant.(string); ok {
				return s, true
			}
			if pd, ok := v.Decl.(*syntax.PropDecl); ok && v.Const {
				return constString(pd.Init, depth+1)
			}
		}
	}
	return "", false
}

// constVar reports whether reading v may be a constant: a const val, a
// static final field, or an enum entry, but not a local, a parameter or
// another property.
func constVar(v *types.VarSym) bool {
	switch {
	case v.Const || v.Constant != nil:
		return true
	case v.Kind == types.Static:
		return !v.Mutable && !javaNonConst(v)
	}
	return false
}

// constType reports whether values of type t can be constants: those of
// the primitive types, unsigned integer types and String.
func constType(t types.Type) bool {
	switch t.K {
	case types.Bool, types.Int, types.Long, types.Double, types.Char, types.Float, types.Byte, types.Short, types.String:
		return true
	case types.Null:
		return true
	case types.Nothing:
		return t.N // null
	case types.Class:
		return isUnsigned(t)
	}
	return false
}

// integralConst reports whether t is an integer type whose constants
// kotlinc evaluates: Int, Long, Short or Byte.
func integralConst(t types.Type) bool {
	switch t.K {
	case types.Int, types.Long, types.Short, types.Byte:
		return !t.N
	}
	return false
}

// zeroDivisor reports whether x, an integer divisor, is literal
// arithmetic whose value is 0, or a const val of 0.
func zeroDivisor(x syntax.Expr) bool {
	x = uncheck(x)
	if v, ok := arithValue(x); ok {
		return v == 0
	}
	var sym *types.VarSym
	switch x := x.(type) {
	case *syntax.Ident:
		sym = x.Sym
	case *syntax.Member:
		sym = cmp.Or(x.Prop, x.Field)
	}
	if sym == nil || !sym.Const {
		return false
	}
	if pd, ok := sym.Decl.(*syntax.PropDecl); ok && pd.Init != nil {
		if v, ok := arithValue(uncheck(pd.Init)); ok {
			return v == 0
		}
	}
	switch v := sym.Constant.(type) {
	case int32:
		return v == 0
	case int64:
		return v == 0
	}
	return false
}

// propIn reports whether property d is declared among ds, or in their
// classes.
func propIn(ds []syntax.Decl, d *syntax.PropDecl) bool {
	for _, x := range ds {
		switch x := x.(type) {
		case *syntax.PropDecl:
			if x == d {
				return true
			}
		case *syntax.ClassDecl:
			if propIn(x.Members, d) {
				return true
			}
		}
	}
	return false
}
