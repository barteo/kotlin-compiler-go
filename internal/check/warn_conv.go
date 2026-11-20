package check

import (
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// This file reports three of kotlinc's warnings about calls and
// comparisons that don't do what they seem to:
// REDUNDANT_CALL_OF_CONVERSION_METHOD on a conversion to the type the
// value has already, ARRAY_EQUALITY_OPERATOR_CAN_BE_REPLACED_WITH_CONTENT_EQUALS
// on == of two arrays, which compares references, and, with -Wextra,
// EMPTY_RANGE on a range of constants that is empty.

// conversionTargets are the conversions by name, with whether a type is
// the one each converts to.
var conversionTargets = map[string]func(types.Type) bool{
	"toString": kindIs(types.String), "toDouble": kindIs(types.Double), "toFloat": kindIs(types.Float),
	"toLong": kindIs(types.Long), "toInt": kindIs(types.Int), "toChar": kindIs(types.Char),
	"toShort": kindIs(types.Short), "toByte": kindIs(types.Byte),
	"toULong": classIs("kotlin/ULong"), "toUInt": classIs("kotlin/UInt"),
	"toUShort": classIs("kotlin/UShort"), "toUByte": classIs("kotlin/UByte"),
}

func kindIs(k types.Kind) func(types.Type) bool {
	return func(t types.Type) bool { return t.K == k }
}

func classIs(name string) func(types.Type) bool {
	return func(t types.Type) bool { return t.K == types.Class && t.C != nil && t.C.Name == name }
}

// conversionWarning reports call x if it is a member conversion, as
// toInt(), of a value of the type it converts to: kotlinc's
// RedundantCallOfConversionMethodChecker. A safe call's receiver is
// judged as not null. An integer literal without a suffix, or arithmetic
// of them, isn't judged: its type may be another, which the conversion
// picks, as in f(1.toInt()) for overloads f(Int) and f(Long).
func (c *Checker) conversionWarning(x *syntax.Call) {
	m, ok := x.Fun.(*syntax.Member)
	if !ok || m.X == nil || x.Invoke || x.Ctor != nil || len(x.Args) > 0 || m.Static != nil {
		return
	}
	target := conversionTargets[m.Name]
	if target == nil || x.Sym != nil && (x.Sym.Ext || x.Sym.Name != m.Name || len(x.Sym.Params) > 0) {
		return
	}
	if x.Sym == nil && x.Builtin == types.NotBuiltin {
		return
	}
	t := m.X.Type()
	if m.Safe {
		t = t.NonNull()
	}
	if t.N || t.P || literalTypeAmbiguous(m.X) || !target(t) {
		return
	}
	c.warnf(syntax.StartPos(x), "REDUNDANT_CALL_OF_CONVERSION_METHOD", "Redundant call of conversion method.")
}

// literalTypeAmbiguous reports whether e is an integer literal without a
// suffix, or arithmetic of them, whose type its use may pick.
func literalTypeAmbiguous(e syntax.Expr) bool {
	switch e := e.(type) {
	case *syntax.IntLit:
		return !e.Long
	case *syntax.Unary:
		return (e.Op == syntax.MINUS || e.Op == syntax.PLUS) && literalTypeAmbiguous(e.X)
	case *syntax.Binary:
		switch e.Op {
		case syntax.PLUS, syntax.MINUS, syntax.STAR, syntax.SLASH, syntax.PERCENT:
			return literalTypeAmbiguous(e.X) && literalTypeAmbiguous(e.Y)
		}
	}
	return false
}

// unsignedArrays are the classes of arrays of unsigned numbers.
var unsignedArrays = map[string]bool{
	"kotlin/UIntArray": true, "kotlin/ULongArray": true, "kotlin/UShortArray": true, "kotlin/UByteArray": true,
}

// arrayEqualityWarning reports x, == or != of two arrays of the same
// class, which compares their references: kotlinc's
// ArrayEqualityCanBeReplacedWithContentEquals.
func (c *Checker) arrayEqualityWarning(x *syntax.Binary) {
	if x.Op != syntax.EQ && x.Op != syntax.NE || x.X == nil || x.Y == nil {
		return
	}
	xc, yc := c.arrayClass(x.X.Type()), c.arrayClass(x.Y.Type())
	if xc == nil || xc != yc {
		return
	}
	op, not := "==", ""
	if x.Op == syntax.NE {
		op, not = "!=", "!"
	}
	c.warnf(syntax.StartPos(x), "ARRAY_EQUALITY_OPERATOR_CAN_BE_REPLACED_WITH_CONTENT_EQUALS",
		"'%s' on arrays compares only references. Replace '%s' with '%scontentEquals' to compare the arrays' contents or use '%s=' to remove the warning.",
		op, op, not, op)
}

// arrayClass returns the class of arrays of type t, or nil: of a type
// parameter smart cast to an array type too, as a of T in
// a is IntArray && a == b.
func (c *Checker) arrayClass(t types.Type) *types.ClassSym {
	var out *types.ClassSym
	for _, cls := range c.boundClasses(t, nil, 0) {
		if cls.Array || unsignedArrays[cls.Name] {
			out = cls
		}
	}
	return out
}

// emptyRangeWarning reports range x, x..y, x..<y, x until y or x downTo
// y, or a call of rangeTo, rangeUntil, until or downTo, if it is of two
// constants and empty, as 2..1 or 1 until 1: kotlinc's extra
// EmptyRangeChecker.
func (c *Checker) emptyRangeWarning(x syntax.Expr) {
	var lo, hi syntax.Expr
	var name string
	switch x := x.(type) {
	case *syntax.Binary:
		lo, hi = x.X, x.Y
		switch x.Op {
		case syntax.RANGE:
			name = "rangeTo"
		case syntax.RANGE_LT:
			name = "rangeUntil"
		case syntax.UNTIL:
			name = "until"
		case syntax.DOWN_TO:
			name = "downTo"
		}
	case *syntax.Call:
		if m, ok := x.Fun.(*syntax.Member); ok && m.X != nil && len(x.Args) == 1 && !x.Invoke {
			lo, hi, name = m.X, x.Args[0], m.Name
		}
	}
	if name == "" {
		return
	}
	cmp, ok := compareLiterals(lo, hi)
	if !ok {
		return
	}
	var empty bool
	switch name {
	case "rangeTo":
		empty = cmp > 0
	case "downTo":
		empty = cmp < 0
	case "until", "rangeUntil":
		empty = cmp >= 0
	}
	if empty {
		c.warnf(syntax.StartPos(x), "EMPTY_RANGE", "this range is empty")
	}
}

// compareLiterals compares literals x and y, both integers, both floating
// point numbers of a type or both characters: -1, 0 or 1.
func compareLiterals(x, y syntax.Expr) (int, bool) {
	xv, xk, ok := literalValue(x)
	if !ok {
		return 0, false
	}
	yv, yk, ok := literalValue(y)
	if !ok || xk != yk {
		return 0, false
	}
	switch {
	case xv < yv:
		return -1, true
	case xv > yv:
		return 1, true
	}
	return 0, true
}

// literalValue returns the value of literal e, a signed integer, a Float,
// a Double or a Char, possibly negated, and which of those it is.
func literalValue(e syntax.Expr) (float64, string, bool) {
	switch e := e.(type) {
	case *syntax.Checked:
		return literalValue(e.X)
	case *syntax.IntLit:
		if e.Unsigned {
			return 0, "", false
		}
		return float64(e.Val), "int", true
	case *syntax.DoubleLit:
		return e.Val, "double", true
	case *syntax.FloatLit:
		return float64(e.Val), "float", true
	case *syntax.CharLit:
		return float64(e.Val), "char", true
	case *syntax.Unary:
		if e.Op == syntax.MINUS {
			if v, k, ok := literalValue(e.X); ok && k != "char" {
				return -v, k, true
			}
		}
	}
	return 0, "", false
}
