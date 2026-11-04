package codegen

import (
	"strings"

	cf "github.com/barteo/kotlin-compiler-go/internal/classfile"
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// operand is a bound of a counted loop: expression x, or, if x is nil,
// the constant konst, or else the value that push leaves on the stack.
type operand struct {
	x     syntax.Expr
	konst *int32
	push  func()
}

// intConst is the operand of constant v.
func intConst(v int32) operand { return operand{konst: &v} }

// operand generates o as a value of type t, which is Int if o isn't an
// expression.
func (g *gen) operand(o operand, t types.Type) {
	switch {
	case o.x != nil:
		g.exprAs(o.x, t)
	case o.konst != nil:
		g.code.Iconst(*o.konst)
	default:
		o.push()
	}
}

// progression is a loop's range that is not a range expression but that
// kotlinc compiles as one, a counted loop: the Int progression from first
// to last of op, RANGE, UNTIL or DOWN_TO, with step if it isn't nil.
type progression struct {
	op          syntax.Kind
	first, last operand
	step        syntax.Expr
}

// progression returns the counted loop of range e, or nil if it has
// none. They are those of the standard library's functions that kotlinc
// compiles as counted loops: x.indices of an array, a CharSequence or a
// Collection, which is 0 until x's size; x.indices step n; and
// reversed() of x.indices, of a..b and of a downTo b.
func (g *gen) progression(e syntax.Expr) *progression {
	switch e := unwrapChecked(e).(type) {
	case *syntax.Member:
		if size := g.indicesSize(e); size != nil {
			return &progression{op: syntax.UNTIL, first: intConst(0), last: operand{push: size}}
		}
	case *syntax.Binary:
		// x.indices step n, which the checker made x.indices.step(n).
		if e.Op != syntax.STEP || !rangesFun(e.Call, "step", "(Lkotlin/ranges/IntProgression;I)Lkotlin/ranges/IntProgression;") {
			return nil
		}
		m, ok := e.Call.Fun.(*syntax.Member)
		if !ok || m.Safe || len(e.Call.Args) != 1 {
			return nil
		}
		if size := g.indicesSize(m.X); size != nil {
			return &progression{op: syntax.RANGE, first: intConst(0), last: operand{push: g.lastIndex(size)}, step: e.Call.Args[0]}
		}
	case *syntax.Call:
		if !rangesFun(e, "reversed", "(Lkotlin/ranges/IntProgression;)Lkotlin/ranges/IntProgression;") || len(e.Args) != 0 {
			return nil
		}
		m, ok := e.Fun.(*syntax.Member)
		if !ok || m.Safe {
			return nil
		}
		if size := g.indicesSize(m.X); size != nil {
			return &progression{op: syntax.DOWN_TO, first: operand{push: g.lastIndex(size)}, last: intConst(0)}
		}
		r, ok := unwrapChecked(m.X).(*syntax.Binary)
		if !ok || r.Call == nil {
			return nil
		}
		var a, b syntax.Expr
		op := syntax.DOWN_TO
		switch {
		case r.Op == syntax.RANGE && r.Call.Ctor != nil && r.Call.Ctor.Name == "kotlin/ranges/IntRange" && len(r.Call.Args) == 2:
			// a..b, an IntRange(a, b): b downTo a.
			a, b = r.Call.Args[0], r.Call.Args[1]
		case r.Op == syntax.DOWN_TO && rangesFun(r.Call, "downTo", "(II)Lkotlin/ranges/IntProgression;") && len(r.Call.Args) == 1:
			// a downTo b: b..a.
			dm, ok := r.Call.Fun.(*syntax.Member)
			if !ok || dm.Safe {
				return nil
			}
			a, b, op = dm.X, r.Call.Args[0], syntax.RANGE
		default:
			return nil
		}
		if a.Type().Flex() != types.IntType || b.Type().Flex() != types.IntType {
			return nil
		}
		// The loop starts at b, but a is evaluated first: into a local,
		// unless it is a constant or a val, whose value b can't change.
		last := operand{x: a}
		if _, ok := constOf(a, 0); !ok && stableLocal(a) == nil {
			tmp := -1
			last = operand{push: func() { g.code.Local(cf.ILOAD, tmp) }}
			first := operand{push: func() {
				tmp = g.code.AllocLocal("I")
				g.exprAs(a, types.IntType)
				g.code.Local(cf.ISTORE, tmp)
				g.exprAs(b, types.IntType)
			}}
			return &progression{op: op, first: first, last: last}
		}
		return &progression{op: op, first: operand{x: b}, last: last}
	}
	return nil
}

// lastIndex returns the code that pushes the last index of a receiver
// whose size size pushes: the size less one.
func (g *gen) lastIndex(size func()) func() {
	return func() {
		size()
		g.code.Iconst(-1)
		g.code.Op(cf.IADD)
	}
}

// indicesSize returns, if x is the standard library's indices of an
// array, of a CharSequence or of a Collection, the code that evaluates
// its receiver and pushes its size, as kotlinc reads it for a loop over
// the indices: an array's length, a CharSequence's length() or a
// Collection's size(). It returns nil otherwise: a member or another
// extension named indices is called as any other.
func (g *gen) indicesSize(x syntax.Expr) func() {
	m, ok := unwrapChecked(x).(*syntax.Member)
	if !ok || m.Safe || m.Call != nil || m.Dispatch != nil {
		return nil
	}
	f := m.Getter
	if f == nil || !f.Ext || f.Owner != nil || !strings.HasPrefix(f.Name, "getIndices") {
		return nil
	}
	var size func()
	switch f.Facade {
	case "kotlin/collections/ArraysKt", "kotlin/collections/unsigned/UArraysKt":
		if !strings.HasPrefix(f.Desc, "([") {
			return nil
		}
		size = func() { g.code.Op(cf.ARRAYLENGTH) }
	case "kotlin/text/StringsKt":
		if !strings.HasPrefix(f.Desc, "(Ljava/lang/CharSequence;)") {
			return nil
		}
		size = func() { g.code.InvokeInterface("java/lang/CharSequence", "length", "()I") }
	case "kotlin/collections/CollectionsKt":
		if !strings.HasPrefix(f.Desc, "(Ljava/util/Collection;)") {
			return nil
		}
		size = func() { g.code.InvokeInterface("java/util/Collection", "size", "()I") }
	default:
		return nil
	}
	return func() {
		g.safeCall(m, types.IntType, types.IntType, func() {
			g.coerceRecv(m.X.Type(), f.Recv)
			size()
		})
	}
}

// rangesFun reports whether call c calls the standard library's function
// name of kotlin.ranges with JVM descriptor d.
func rangesFun(c *syntax.Call, name, d string) bool {
	return c != nil && c.Sym != nil && c.Sym.Name == name && c.Sym.Facade == "kotlin/ranges/RangesKt" && c.Sym.Desc == d
}

// unwrapChecked returns the expression that x, a Checked or not, is.
func unwrapChecked(x syntax.Expr) syntax.Expr {
	for {
		c, ok := x.(*syntax.Checked)
		if !ok || c.SAM != nil {
			return x
		}
		x = c.X
	}
}
