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
// kotlinc compiles as one, a counted loop: the progression of elements of
// type t, Int, Char or Long, from first to last of op, RANGE, UNTIL or
// DOWN_TO, with step if it isn't nil. If reverse is set, the loop counts
// the stepped progression backwards, from its last element to first.
type progression struct {
	op          syntax.Kind
	first, last operand
	step        syntax.Expr
	t           types.Type
	reverse     bool
}

// progression returns the counted loop of range e, or nil if it has
// none. They are those of the standard library's functions that kotlinc
// compiles as counted loops: x.indices of an array, a CharSequence or a
// Collection, which is 0 until x's size; x.indices step n; reversed() of
// x.indices, of a..b and of a downTo b, and that step n; and reversed()
// of a..b step n and of a downTo b step n.
func (g *gen) progression(e syntax.Expr) *progression {
	switch e := unwrapChecked(e).(type) {
	case *syntax.Member:
		if size := g.indicesSize(e); size != nil {
			return &progression{op: syntax.UNTIL, first: intConst(0), last: operand{push: size}, t: types.IntType}
		}
	case *syntax.Binary:
		// p step n, which the checker made p.step(n).
		if e.Op != syntax.STEP || e.Call == nil || len(e.Call.Args) != 1 {
			return nil
		}
		m, ok := e.Call.Fun.(*syntax.Member)
		if !ok || m.Safe {
			return nil
		}
		if rangesFun(e.Call, "step", stepDesc(types.IntType)) {
			if size := g.indicesSize(m.X); size != nil {
				return &progression{op: syntax.RANGE, first: intConst(0), last: operand{push: g.lastIndex(size)}, step: e.Call.Args[0], t: types.IntType}
			}
		}
		if r, ok := unwrapChecked(m.X).(*syntax.Call); ok {
			if p := g.reversed(r); p != nil && p.step == nil && rangesFun(e.Call, "step", stepDesc(p.t)) {
				p.step = e.Call.Args[0]
				return p
			}
		}
	case *syntax.Call:
		return g.reversed(e)
	}
	return nil
}

// progressionTypes maps the element types of the progressions to the
// name of their class, less Range or Progression.
var progressionTypes = map[types.Kind]string{types.Int: "Int", types.Char: "Char", types.Long: "Long"}

// stepDesc returns the descriptor of the standard library's step of the
// progressions of elements of type t.
func stepDesc(t types.Type) string {
	c, s := progressionTypes[t.K], "I"
	if t.K == types.Long {
		s = "J"
	}
	return "(Lkotlin/ranges/" + c + "Progression;" + s + ")Lkotlin/ranges/" + c + "Progression;"
}

// reversed returns the counted loop of e if it is reversed() of
// x.indices, of a..b, of a downTo b, or of either with a step, or nil.
func (g *gen) reversed(e *syntax.Call) *progression {
	m, ok := e.Fun.(*syntax.Member)
	if !ok || m.Safe || len(e.Args) != 0 || e.Sym == nil || e.Sym.Name != "reversed" {
		return nil
	}
	var t types.Type
	for k, c := range progressionTypes {
		if rangesFun(e, "reversed", "(Lkotlin/ranges/"+c+"Progression;)Lkotlin/ranges/"+c+"Progression;") {
			t = types.Type{K: k}
		}
	}
	if t.K == types.Invalid {
		return nil
	}
	if t.K == types.Int {
		if size := g.indicesSize(m.X); size != nil {
			return &progression{op: syntax.DOWN_TO, first: operand{push: g.lastIndex(size)}, last: intConst(0), t: t}
		}
	}
	r, ok := unwrapChecked(m.X).(*syntax.Binary)
	if !ok {
		return nil
	}
	if r.Op == syntax.STEP {
		// (a..b step n).reversed() and (a downTo b step n).reversed():
		// counted from the last element, which kotlinc computes as for
		// a..b step n, down to a.
		if r.Call == nil || len(r.Call.Args) != 1 || !rangesFun(r.Call, "step", stepDesc(t)) {
			return nil
		}
		sm, ok := r.Call.Fun.(*syntax.Member)
		if !ok || sm.Safe {
			return nil
		}
		inner, ok := unwrapChecked(sm.X).(*syntax.Binary)
		if !ok {
			return nil
		}
		op, a, b := g.bounds(inner, t)
		if op == 0 {
			return nil
		}
		return &progression{op: op, first: operand{x: a}, last: operand{x: b}, step: r.Call.Args[0], t: t, reverse: true}
	}
	op, a, b := g.bounds(r, t)
	if op == 0 {
		return nil
	}
	// a..b reversed is b downTo a, and a downTo b reversed is b..a.
	if op == syntax.RANGE {
		op = syntax.DOWN_TO
	} else {
		op = syntax.RANGE
	}
	// The loop starts at b, but a is evaluated first: into a local,
	// unless it is a constant or a val, whose value b can't change.
	if _, ok := constOf(a, 0); ok || stableLocal(a) != nil {
		return &progression{op: op, first: operand{x: b}, last: operand{x: a}, t: t}
	}
	d, load, store := "I", byte(cf.ILOAD), byte(cf.ISTORE)
	if t.K == types.Long {
		d, load, store = "J", cf.LLOAD, cf.LSTORE
	}
	tmp := -1
	first := operand{push: func() {
		tmp = g.code.AllocLocal(d)
		g.exprAs(a, t)
		g.code.Local(store, tmp)
		g.exprAs(b, t)
	}}
	last := operand{push: func() { g.code.Local(load, tmp) }}
	return &progression{op: op, first: first, last: last, t: t}
}

// bounds returns, if r is a..b, a constructor call of the standard
// library's range of elements of type t, or a downTo b, its op, RANGE or
// DOWN_TO, and a and b. It returns 0 otherwise.
func (g *gen) bounds(r *syntax.Binary, t types.Type) (op syntax.Kind, a, b syntax.Expr) {
	if r.Call == nil {
		return 0, nil, nil
	}
	c := progressionTypes[t.K]
	switch {
	case r.Op == syntax.RANGE && r.Call.Ctor != nil && r.Call.Ctor.Name == "kotlin/ranges/"+c+"Range" && len(r.Call.Args) == 2:
		op, a, b = syntax.RANGE, r.Call.Args[0], r.Call.Args[1]
	case r.Op == syntax.DOWN_TO && len(r.Call.Args) == 1:
		d, _ := types.Desc(t)
		if !rangesFun(r.Call, "downTo", "("+d+d+")Lkotlin/ranges/"+c+"Progression;") {
			return 0, nil, nil
		}
		m, ok := r.Call.Fun.(*syntax.Member)
		if !ok || m.Safe {
			return 0, nil, nil
		}
		op, a, b = syntax.DOWN_TO, m.X, r.Call.Args[0]
	default:
		return 0, nil, nil
	}
	if a.Type().Flex() != t || b.Type().Flex() != t {
		return 0, nil, nil
	}
	return op, a, b
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

// progressionClasses maps the standard library's progression classes to
// the descriptor of their elements and of their step.
var progressionClasses = map[string][2]string{
	"kotlin/ranges/IntRange":        {"I", "I"},
	"kotlin/ranges/IntProgression":  {"I", "I"},
	"kotlin/ranges/CharRange":       {"C", "I"},
	"kotlin/ranges/CharProgression": {"C", "I"},
	"kotlin/ranges/LongRange":       {"J", "J"},
	"kotlin/ranges/LongProgression": {"J", "J"},
}

// forProgression generates, if the range of loop s is a value of the
// standard library's progression classes, and its variable of their
// element type, a counted loop over its first, last and step, as kotlinc
// does, and reports whether it did. A progression's last element is one
// that the steps from its first reach, so the loop ends at it, testing
// before the step, which can't overflow. A range's step is 1.
func (g *gen) forProgression(s *syntax.For) bool {
	rt := s.Range.Type().NonNull()
	if rt.K != types.Class || rt.C == nil {
		return false
	}
	ds, ok := progressionClasses[rt.C.Name]
	if !ok {
		return false
	}
	elem, owner := ds[0], rt.C.Name
	var et types.Type
	switch elem {
	case "I":
		et = types.IntType
	case "C":
		et = types.CharType
	default:
		et = types.LongType
	}
	if s.Sym.Type != et {
		return false
	}
	long := elem == "J"
	load, store, add := byte(cf.ILOAD), byte(cf.ISTORE), byte(cf.IADD)
	if long {
		load, store, add = cf.LLOAD, cf.LSTORE, cf.LADD
	}
	ranged := strings.HasSuffix(owner, "Range")
	g.exprAs(s.Range, rt)
	p := g.code.AllocLocal("L" + owner + ";")
	g.code.Local(cf.ASTORE, p)
	get := func(name, t string) int {
		slot := g.code.AllocLocal(t)
		g.code.Local(cf.ALOAD, p)
		g.code.Invoke(cf.INVOKEVIRTUAL, owner, name, "()"+t)
		if t == "C" {
			t = "I"
		}
		if t == "J" {
			g.code.Local(cf.LSTORE, slot)
		} else {
			g.code.Local(cf.ISTORE, slot)
		}
		return slot
	}
	i := get("getFirst", elem)
	g.setSlot(s.Sym, i)
	g.named(s.Sym)
	last := get("getLast", elem)
	st := -1
	if !ranged {
		st = get("getStep", ds[1])
	}
	// cmpJump compares i and last, or last and i, and jumps to l on op,
	// one of IF_ICMPxx.
	cmpJump := func(a, b int, op byte, l *cf.Label) {
		g.code.Local(load, a)
		g.code.Local(load, b)
		if long {
			g.code.Op(cf.LCMP)
			op -= cf.IF_ICMPEQ - cf.IFEQ
		}
		g.code.Jump(op, l)
	}
	// signJump jumps to l on op, one of IFxx, of the step's sign.
	signJump := func(op byte, l *cf.Label) {
		g.code.Local(load, st)
		if long {
			g.code.Lconst(0)
			g.code.Op(cf.LCMP)
		}
		g.code.Jump(op, l)
	}
	top, cont, exit := cf.NewLabel(), cf.NewLabel(), cf.NewLabel()
	if ranged {
		cmpJump(i, last, cf.IF_ICMPGT, exit)
	} else {
		// Empty unless the step is positive and first <= last, or
		// negative and last <= first.
		down := cf.NewLabel()
		signJump(cf.IFLE, down)
		cmpJump(i, last, cf.IF_ICMPLE, top)
		g.code.Place(down)
		signJump(cf.IFGE, exit)
		cmpJump(last, i, cf.IF_ICMPGT, exit)
	}
	g.code.Place(top)
	g.pushLoop(s, exit, cont)
	g.loopBody(func() { g.block(s.Body, types.UnitType, false) })
	g.code.Place(cont)
	g.line(s.P)
	cmpJump(i, last, cf.IF_ICMPEQ, exit)
	switch {
	case ranged && !long:
		g.code.Iinc(i, 1)
	default:
		g.code.Local(load, i)
		if ranged {
			g.code.Lconst(1)
		} else {
			g.code.Local(load, st)
		}
		g.code.Op(add)
		g.code.Local(store, i)
	}
	g.code.Jump(cf.GOTO, top)
	g.code.Place(exit)
	return true
}
