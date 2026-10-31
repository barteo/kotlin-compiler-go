package codegen

import (
	cf "github.com/barteo/kotlin-compiler-go/internal/classfile"
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// Conditions are generated as jumps: cond jumps to l if x evaluates to
// jumpIf and falls through otherwise.
func (g *gen) cond(x syntax.Expr, l *cf.Label, jumpIf bool) {
	switch x := x.(type) {
	case *syntax.BoolLit:
		if x.Val == jumpIf {
			g.code.Jump(cf.GOTO, l)
		}
		return
	case *syntax.Unary:
		if x.Op == syntax.NOT && x.Call == nil {
			g.cond(x.X, l, !jumpIf)
			return
		}
	case *syntax.Is:
		g.expr(x.X)
		g.instanceOf(x.X.Type(), x.Target)
		if x.Not == jumpIf {
			g.code.Jump(cf.IFEQ, l)
		} else {
			g.code.Jump(cf.IFNE, l)
		}
		return
	case *syntax.Binary:
		switch x.Op {
		case syntax.AND, syntax.OR:
			if x.Groups {
				g.groupedLogic(x)
				if jumpIf {
					g.code.Jump(cf.IFNE, l)
				} else {
					g.code.Jump(cf.IFEQ, l)
				}
				return
			}
			// a && b jumps on true only if both hold; a || b jumps on false
			// only if both fail. Otherwise each operand can jump directly.
			if (x.Op == syntax.AND) == jumpIf {
				skip := cf.NewLabel()
				g.cond(x.X, skip, !jumpIf)
				g.cond(x.Y, l, jumpIf)
				g.code.Place(skip)
			} else {
				g.cond(x.X, l, jumpIf)
				g.cond(x.Y, l, jumpIf)
			}
			return
		case syntax.EQ, syntax.NE, syntax.EQ_EQ_EQ, syntax.NOT_EQ_EQ:
			g.equals(func() { g.expr(x.X) }, x.X.Type(), x.Y, x.OpType, x.Op, l, jumpIf)
			return
		case syntax.LT, syntax.GT, syntax.LE, syntax.GE:
			g.compare(x, l, jumpIf)
			return
		case syntax.IN, syntax.NOT_IN:
			if x.Call != nil {
				// y.contains(x)
				g.exprAs(x.Call, types.BoolType)
				if (x.Op == syntax.IN) == jumpIf {
					g.code.Jump(cf.IFNE, l)
				} else {
					g.code.Jump(cf.IFEQ, l)
				}
				return
			}
			vt := types.IntType
			if x.X.Type().Flex() == types.LongType {
				vt = types.LongType
			}
			slot := g.code.AllocLocal(desc(vt))
			eval := func() {
				g.exprAs(x.X, vt)
				g.code.Local(storeOp(vt), slot)
			}
			g.inRange(eval, func() { g.code.Local(loadOp(vt), slot) }, vt, x.Y.(*syntax.Binary), x.Op == syntax.IN, l, jumpIf)
			return
		}
	}
	g.exprAs(x, types.BoolType)
	if jumpIf {
		g.code.Jump(cf.IFNE, l)
	} else {
		g.code.Jump(cf.IFEQ, l)
	}
}

// groupedLogic pushes the value of a && b or a || b, whose right side
// runs in a replace group, and the code that skips it in an empty one: a
// jump can't leave the group.
func (g *gen) groupedLogic(x *syntax.Binary) {
	skip, end := cf.NewLabel(), join(types.BoolType)
	and := x.Op == syntax.AND
	g.cond(x.X, skip, !and)
	g.grouped(true, g.siteKey(x.P, "logic", 0), func() { g.exprAs(x.Y, types.BoolType) })
	g.code.Jump(cf.GOTO, end)
	g.code.Place(skip)
	g.grouped(true, g.siteKey(x.P, "logic", 1), func() {})
	if and {
		g.code.Iconst(0)
	} else {
		g.code.Iconst(1)
	}
	g.code.Place(end)
}

// negate maps a comparison to its opposite: < to >=, == to !=, and so on.
func negate(op syntax.Kind) syntax.Kind {
	switch op {
	case syntax.LT:
		return syntax.GE
	case syntax.GE:
		return syntax.LT
	case syntax.GT:
		return syntax.LE
	case syntax.LE:
		return syntax.GT
	case syntax.EQ:
		return syntax.NE
	}
	return syntax.EQ
}

var (
	intCmp  = map[syntax.Kind]byte{syntax.EQ: cf.IF_ICMPEQ, syntax.NE: cf.IF_ICMPNE, syntax.LT: cf.IF_ICMPLT, syntax.GE: cf.IF_ICMPGE, syntax.GT: cf.IF_ICMPGT, syntax.LE: cf.IF_ICMPLE}
	zeroCmp = map[syntax.Kind]byte{syntax.EQ: cf.IFEQ, syntax.NE: cf.IFNE, syntax.LT: cf.IFLT, syntax.GE: cf.IFGE, syntax.GT: cf.IFGT, syntax.LE: cf.IFLE}
)

// branchCompare compares the two values of type t on the stack and jumps
// to l if `a op b` is jumpIf.
func (g *gen) branchCompare(t types.Type, op syntax.Kind, l *cf.Label, jumpIf bool) {
	// The NaN-aware choice of dcmpg/dcmpl depends on the original operator.
	dcmp, fcmp := byte(cf.DCMPL), byte(cf.FCMPL)
	if op == syntax.LT || op == syntax.LE {
		dcmp, fcmp = cf.DCMPG, cf.FCMPG
	}
	if !jumpIf {
		op = negate(op)
	}
	switch t.K {
	case types.Int, types.Bool, types.Char, types.Byte, types.Short:
		g.code.Jump(intCmp[op], l)
	case types.Float:
		g.code.Op(fcmp)
		g.code.Jump(zeroCmp[op], l)
	case types.Long:
		g.code.Op(cf.LCMP)
		g.code.Jump(zeroCmp[op], l)
	case types.Double:
		g.code.Op(dcmp)
		g.code.Jump(zeroCmp[op], l)
	case types.String:
		g.code.Invoke(cf.INVOKEVIRTUAL, "java/lang/String", "compareTo", "(Ljava/lang/String;)I")
		g.code.Jump(zeroCmp[op], l)
	}
}

func (g *gen) compare(x *syntax.Binary, l *cf.Label, jumpIf bool) {
	if x.Call != nil {
		// x.compareTo(y), compared with 0.
		g.exprAs(x.Call, types.IntType)
		op := x.Op
		if !jumpIf {
			op = negate(op)
		}
		g.code.Jump(zeroCmp[op], l)
		return
	}
	if x.OpType.K == types.Class { // Comparable
		g.exprAs(x.X, x.OpType)
		g.exprAs(x.Y, types.AnyType.Nullable())
		g.code.InvokeInterface("java/lang/Comparable", "compareTo", "(Ljava/lang/Object;)I")
		op := x.Op
		if !jumpIf {
			op = negate(op)
		}
		g.code.Jump(zeroCmp[op], l)
		return
	}
	g.exprAs(x.X, x.OpType)
	g.exprAs(x.Y, x.OpType)
	g.branchCompare(x.OpType, x.Op, l, jumpIf)
}

// equals compares a value pushed by left (of type lt) with right, with op
// (==, !=, === or !==) and the comparison type chosen by the checker, and
// jumps to l if (left op right) == jumpIf. === compares references by
// identity, and primitives by value.
func (g *gen) equals(left func(), lt types.Type, right syntax.Expr, opType types.Type, op syntax.Kind, l *cf.Label, jumpIf bool) {
	eq := op == syntax.EQ || op == syntax.EQ_EQ_EQ
	wantEqual := eq == jumpIf // jump when the operands are equal
	rt := right.Type()
	switch {
	case opType.K == types.Invalid:
		panic("unchecked equality")
	case opType.K == types.Null:
		// One side is the null literal: a null check on the other side.
		var other types.Type
		if rt.K == types.Null {
			left()
			other = lt
		} else {
			g.expr(right)
			other = rt
		}
		if other.K == types.Null || !types.MayBeNull(other) && !nullableRef(other) {
			// null == null, or a value that can never be null.
			g.pop(other)
			if other.K == types.Null == wantEqual {
				g.code.Jump(cf.GOTO, l)
			}
			return
		}
		if wantEqual {
			g.code.Jump(cf.IFNULL, l)
		} else {
			g.code.Jump(cf.IFNONNULL, l)
		}
	case opType.IsPrimitive():
		left()
		g.coerce(lt, opType)
		g.exprAs(right, opType)
		op := syntax.EQ
		if !wantEqual {
			op = syntax.NE
		}
		g.branchCompare(opType, op, l, true)
	default:
		ref := opType.Nullable()
		left()
		g.coerce(lt, ref)
		g.exprAs(right, ref)
		if op == syntax.EQ_EQ_EQ || op == syntax.NOT_EQ_EQ {
			if wantEqual {
				g.code.Jump(cf.IF_ACMPEQ, l)
			} else {
				g.code.Jump(cf.IF_ACMPNE, l)
			}
			return
		}
		if f := opType.NonNull(); f.K == types.Double || f.K == types.Float {
			// IEEE 754 equality of nullable numbers.
			g.areEqual(f)
		} else {
			g.code.Invoke(cf.INVOKESTATIC, "java/util/Objects", "equals", "(Ljava/lang/Object;Ljava/lang/Object;)Z")
		}
		if wantEqual {
			g.code.Jump(cf.IFNE, l)
		} else {
			g.code.Jump(cf.IFEQ, l)
		}
	}
}

// inRange tests whether the value of type vt, Int or Long, pushed by load
// lies in range r, and jumps to l if (in range) == (in == jumpIf). The
// values are compared as Longs if one of them is. As with kotlinc, both
// bounds are evaluated, in order, before eval, if not nil, evaluates the
// value.
func (g *gen) inRange(eval, load0 func(), vt types.Type, r *syntax.Binary, in bool, l *cf.Label, jumpIf bool) {
	t := types.IntType
	if vt.Flex() == types.LongType || r.X.Type().Flex() == types.LongType || r.Y.Type().Flex() == types.LongType {
		t = types.LongType
	}
	load := func() {
		load0()
		g.coerce(vt, t)
	}
	x, y := g.bound(r.X, t), g.bound(r.Y, t)
	if eval != nil {
		eval()
	}
	lo, hi := x, y
	hiOp := syntax.LE
	switch r.Op {
	case syntax.RANGE_LT, syntax.UNTIL:
		hiOp = syntax.LT
	case syntax.DOWN_TO:
		lo, hi = y, x
	}
	if in == jumpIf {
		// Jump to l when lo <= v && v hiOp hi.
		skip := cf.NewLabel()
		lo()
		load()
		g.branchCompare(t, syntax.LE, skip, false)
		load()
		hi()
		g.branchCompare(t, hiOp, l, true)
		g.code.Place(skip)
		return
	}
	// Jump to l when v is outside the range.
	lo()
	load()
	g.branchCompare(t, syntax.LE, l, false)
	load()
	hi()
	g.branchCompare(t, hiOp, l, false)
}

// bound returns a function that pushes range bound e as a t: e itself if
// it is a literal or a val, which may be evaluated later, or else a local
// e is stored in now.
func (g *gen) bound(e syntax.Expr, t types.Type) func() {
	switch e := e.(type) {
	case *syntax.IntLit, *syntax.CharLit:
		return func() { g.exprAs(e, t) }
	case *syntax.Unary:
		if _, ok := e.X.(*syntax.IntLit); ok && e.Op == syntax.MINUS && e.Call == nil {
			return func() { g.exprAs(e, t) }
		}
	case *syntax.Ident:
		if v := e.Sym; v != nil && e.Member == nil && e.Getter == nil && !v.Mutable && (v.Kind == types.Local || v.Kind == types.Param) {
			return func() { g.exprAs(e, t) }
		}
	}
	slot := g.code.AllocLocal(desc(t))
	g.exprAs(e, t)
	g.code.Local(storeOp(t), slot)
	return func() { g.code.Local(loadOp(t), slot) }
}

// areEqual replaces two nullable Doubles or Floats, as f says, with
// whether they are equal as IEEE 754 says, or both null, 1 or 0, as
// Intrinsics.areEqual does.
func (g *gen) areEqual(f types.Type) {
	ref := desc(f.Nullable())
	if g.shared.opts.Stdlib {
		g.code.Invoke(cf.INVOKESTATIC, "kotlin/jvm/internal/Intrinsics", "areEqual", "("+ref+ref+")Z")
		return
	}
	a, b := g.code.AllocLocal(ref), g.code.AllocLocal(ref)
	g.code.Local(cf.ASTORE, b)
	g.code.Local(cf.ASTORE, a)
	aSet, yes, no, end := cf.NewLabel(), cf.NewLabel(), cf.NewLabel(), cf.NewLabel()
	g.code.Local(cf.ALOAD, a)
	g.code.Jump(cf.IFNONNULL, aSet)
	g.code.Local(cf.ALOAD, b)
	g.code.Jump(cf.IFNULL, yes)
	g.code.Jump(cf.GOTO, no)
	g.code.Place(aSet)
	g.code.Local(cf.ALOAD, b)
	g.code.Jump(cf.IFNULL, no)
	g.code.Local(cf.ALOAD, a)
	g.unbox(f)
	g.code.Local(cf.ALOAD, b)
	g.unbox(f)
	g.branchCompare(f, syntax.EQ, no, false)
	g.code.Place(yes)
	g.code.Iconst(1)
	g.code.Jump(cf.GOTO, end)
	g.code.Place(no)
	g.code.Iconst(0)
	g.code.Place(end)
}
