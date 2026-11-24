package check

import (
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// divisionWarning reports n if it divides by zero written as a literal,
// as kotlinc's FirDivisionByZeroChecker does: a call of one of package
// kotlin's div functions, x / 0, x /= 0 or x.div(0), whose argument is a
// literal 0, 0L, 0u, 0.0 or 0f, -0 or +0. -0.0 isn't zero for kotlinc,
// nor is a constant that is 0, and x % 0 isn't reported.
func (c *Checker) divisionWarning(n any) {
	var p syntax.Pos
	switch n := n.(type) {
	case *syntax.Binary:
		if n.Op != syntax.SLASH || n.X == nil || !zeroLiteral(n.Y) {
			return
		}
		if n.Call != nil && !kotlinDiv(n.Call.Sym) || n.Call == nil && !n.OpType.IsNumeric() {
			return
		}
		p = syntax.StartPos(n)
	case *syntax.Assign:
		if n.Op != syntax.SLASH_EQ || n.Stmts != nil || !n.OpType.IsNumeric() || !zeroLiteral(n.Value) {
			return
		}
		p = syntax.StartPos(n.Target)
	case *syntax.Call:
		fn, ok := n.Fun.(*syntax.Member)
		if !ok || fn.Name != "div" || len(n.Args) != 1 || !zeroLiteral(n.Args[0]) {
			return
		}
		if n.Builtin != types.OperatorFun && !(fn.Written && kotlinDiv(n.Sym)) {
			return
		}
		p = syntax.StartPos(n)
	default:
		return
	}
	c.warnf(p, "DIVISION_BY_ZERO", "division by zero")
}

// kotlinDiv reports whether f is a div member of a class of package
// kotlin, as UInt's.
func kotlinDiv(f *types.FunSym) bool {
	if f == nil || f.Name != "div" || f.Owner == nil {
		return false
	}
	rest, ok := strings.CutPrefix(f.Owner.Name, "kotlin/")
	return ok && !strings.Contains(rest, "/")
}

// zeroLiteral reports whether x is a literal whose value kotlinc takes
// for 0: an integer literal 0, -0 or +0, but not -0L, or a
// floating-point one 0.0, but not -0.0 or +0.0.
func zeroLiteral(x syntax.Expr) bool {
	switch x := x.(type) {
	case *syntax.IntLit:
		return x.Val == 0
	case *syntax.DoubleLit:
		return x.Val == 0 && !negZero(float64(x.Val))
	case *syntax.FloatLit:
		return x.Val == 0 && !negZero(float64(x.Val))
	case *syntax.Unary:
		if x.Op == syntax.MINUS || x.Op == syntax.PLUS {
			// kotlinc takes -0 and -(0) for the literal 0, but not -0L.
			if l, ok := x.X.(*syntax.IntLit); ok && x.Call == nil {
				return l.Val == 0 && !l.Long && !l.Unsigned
			}
		}
	}
	return false
}

func negZero(f float64) bool { return f == 0 && 1/f < 0 }
