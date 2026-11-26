package check

import (
	"math"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// Arithmetic of integer literals, 10 * 60, has an integer literal type in
// K2, as a literal does: passed for a type variable, it is an Int or a
// Long as the variable's other bounds need, so createAssert(long)
// .isGreaterThanOrEqualTo(10 * 10) for <A, B : Comparable<A>> infers A
// Long.

// arithLit returns a literal of x's value, if x is arithmetic of integer
// literals, but no literal itself, whose value is known: the variable it
// is passed for records it as it does a literal (see system.addLit).
func arithLit(x syntax.Expr) *syntax.IntLit {
	x = uncheck(x)
	if _, ok := x.(*syntax.IntLit); ok || !literalArith(x) {
		return nil
	}
	v, ok := arithValue(x)
	if !ok {
		return nil
	}
	lit := &syntax.IntLit{Val: v, Long: v > math.MaxInt32 || v < math.MinInt32}
	lit.P = x.Pos()
	return lit
}

// arithValue returns the value of literal arithmetic x, if it doesn't
// overflow a Long or divide by zero.
func arithValue(x syntax.Expr) (int64, bool) {
	switch x := x.(type) {
	case *syntax.IntLit:
		return x.Val, true
	case *syntax.Unary:
		v, ok := arithValue(x.X)
		if x.Op == syntax.MINUS {
			v = -v
		}
		return v, ok
	case *syntax.Binary:
		a, ok1 := arithValue(x.X)
		b, ok2 := arithValue(x.Y)
		if !ok1 || !ok2 {
			return 0, false
		}
		switch x.Op {
		case syntax.PLUS:
			return a + b, true
		case syntax.MINUS:
			return a - b, true
		case syntax.STAR:
			if a != 0 && (a*b)/a != b {
				return 0, false
			}
			return a * b, true
		case syntax.SLASH:
			if b == 0 {
				return 0, false
			}
			return a / b, true
		case syntax.PERCENT:
			if b == 0 {
				return 0, false
			}
			return a % b, true
		}
	case *syntax.Call:
		// 2.plus(1) is 2 + 1; 1.shl(3) and the like, as Ints.
		m, ok := literalOpCall(x)
		if !ok {
			return 0, false
		}
		if len(x.Args) == 0 {
			v, ok := arithValue(m.X)
			switch m.Name {
			case "unaryMinus":
				v = -v
			case "inv":
				v = ^v
			}
			return v, ok
		}
		op, ok := map[string]syntax.Kind{"plus": syntax.PLUS, "minus": syntax.MINUS, "times": syntax.STAR, "div": syntax.SLASH, "rem": syntax.PERCENT}[m.Name]
		if ok {
			return arithValue(&syntax.Binary{Op: op, X: m.X, Y: x.Args[0]})
		}
		a, ok1 := arithValue(m.X)
		b, ok2 := arithValue(x.Args[0])
		if !ok1 || !ok2 {
			return 0, false
		}
		i, j := int32(a), int32(b)
		switch m.Name {
		case "and":
			return int64(i & j), true
		case "or":
			return int64(i | j), true
		case "xor":
			return int64(i ^ j), true
		case "shl":
			return int64(i << (j & 31)), true
		case "shr":
			return int64(i >> (j & 31)), true
		case "ushr":
			return int64(int32(uint32(i) >> (j & 31))), true
		}
	}
	return 0, false
}

// retypeArith checks literal arithmetic a, passed for a variable that was
// inferred as t, again as a Long if t is one.
func (c *Checker) retypeArith(a syntax.Expr, t types.Type) syntax.Expr {
	x := uncheck(a)
	if _, ok := x.(*syntax.IntLit); ok || t.Flex().NonNull() != types.LongType || a.Type().Flex().NonNull() == types.LongType || !literalArith(x) {
		return a
	}
	return checked(x, c.expr(x, types.LongType))
}

// uncheck returns the expression a Checked one wraps.
func uncheck(x syntax.Expr) syntax.Expr {
	for {
		ch, ok := x.(*syntax.Checked)
		if !ok || ch.SAM != nil {
			return x
		}
		x = ch.X
	}
}
