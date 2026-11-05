package check

import (
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
			if !constVar(x.Sym) {
				return x
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
			if !constVar(x.Prop) {
				return x
			}
			if id, ok := x.X.(*syntax.Ident); ok && id.Sym != nil {
				return x // a const read through a variable: v.CONST
			}
		case x.Field != nil:
			if !constVar(x.Field) {
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
		if !constType(x.X.Type()) || !constType(x.Y.Type()) || nonConst(x.X, depth+1) != nil || nonConst(x.Y, depth+1) != nil {
			return x
		}
		return nil
	case *syntax.Call:
		return nonConstCall(x, depth)
	case *syntax.ClassLit:
		if x.Bound {
			return x // v::class
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
		return !v.Mutable
	}
	return false
}

// constType reports whether values of type t can be constants: those of
// the primitive types, unsigned integer types and String.
func constType(t types.Type) bool {
	switch t.K {
	case types.Bool, types.Int, types.Long, types.Double, types.Char, types.Float, types.Byte, types.Short, types.String:
		return true
	case types.Nothing:
		return t.N // null
	case types.Class:
		return isUnsigned(t)
	}
	return false
}
