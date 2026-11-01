package codegen

import (
	"math"
	"strings"

	cf "github.com/barteo/kotlin-compiler-go/internal/classfile"
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// block generates a block. If value, the last statement's value is left
// on the stack as type want.
func (g *gen) block(b *syntax.Block, want types.Type, value bool) {
	scope := g.code.Scope()
	pushed := false
	for i, s := range b.Stmts {
		g.line(syntax.StmtPos(s))
		if value && i == len(b.Stmts)-1 {
			if es, ok := s.(*syntax.ExprStmt); ok && !statementOnly(es.X) {
				g.exprAs(es.X, want)
				pushed = true
				break
			}
		}
		g.stmt(s)
	}
	if value && !pushed && g.code.Reachable() {
		// A block that ends with a declaration, an assignment or an if
		// without else has the value Unit, as in when (t) { 1 -> x = y;
		// else -> z }, or a lambda { if (c) x = y } that returns Any.
		g.coerce(types.UnitType, want)
	}
	g.code.EndScope(scope)
}

// statementOnly reports whether x, a block's last statement, has no
// value of its own: an if without else, or a when that the checker
// checked as a statement.
func statementOnly(x syntax.Expr) bool {
	switch x := x.(type) {
	case *syntax.If:
		return x.Else == nil
	case *syntax.When:
		return x.T.K == types.Invalid
	}
	return false
}

// named gives a variable's local its name in the LocalVariableTable,
// except in the copy of an inline function and for the checker's
// temporaries.
func (g *gen) named(v *types.VarSym) {
	if g.noLines == 0 && v.Name != "" && v.Name != "_" && !strings.HasPrefix(v.Name, "$") && !strings.HasPrefix(v.Name, "<") {
		g.code.NameLocal(v.Slot, v.Name)
	}
}

// line records that the code from here on comes from the line of p,
// except in the copy of an inline function, which keeps the line of its
// call.
func (g *gen) line(p syntax.Pos) {
	if g.noLines == 0 {
		g.code.Line(p.Line)
	}
}

func (g *gen) stmt(s syntax.Stmt) {
	if g.sm != nil {
		// In a state machine, the temporaries of a statement are dead
		// after it, so suspension points don't save them. A variable it
		// declares isn't one, though in the copy of an inline function
		// it has no name.
		from := g.code.MaxLocals()
		defer func() {
			g.code.RetireTemps(from)
			if v, ok := s.(*syntax.VarStmt); ok {
				for _, sym := range []*types.VarSym{v.Sym, v.DelegateSym} {
					if sym != nil && sym.Slot >= from {
						g.code.Unretire(sym.Slot)
					}
				}
			}
		}()
	}
	switch s := s.(type) {
	case *syntax.ExprStmt:
		g.exprStmt(s.X)
	case *syntax.VarStmt:
		if s.Delegate != nil {
			// A local delegated property: only its delegate is stored.
			d := s.DelegateSym
			d.Slot = g.code.AllocLocal(desc(d.Type))
			if g.noLines == 0 {
				g.code.NameLocal(d.Slot, s.Sym.Name+"$delegate")
			}
			g.exprAs(s.Delegate, d.Type)
			g.code.Local(storeOp(d.Type), d.Slot)
			break
		}
		if s.Sym.UnitLocal() {
			if s.Init != nil {
				g.exprAs(s.Init, types.UnitType)
			}
			break
		}
		if t := s.Sym.Type; s.Sym.Boxed() {
			s.Sym.Slot = g.code.AllocLocal(storageDesc(s.Sym))
			g.named(s.Sym)
			g.newBox(t)
			if s.Init != nil {
				g.code.Op(cf.DUP)
				g.code.Iconst(0)
				g.exprAs(s.Init, t)
				g.arrayStore(t)
			}
			g.code.Local(cf.ASTORE, s.Sym.Slot)
			break
		}
		s.Sym.Slot = g.code.AllocLocal(desc(s.Sym.Type))
		g.named(s.Sym)
		if s.Lateinit {
			g.code.Op(cf.ACONST_NULL) // null until assigned, as with kotlinc
			g.code.Local(cf.ASTORE, s.Sym.Slot)
			break
		}
		if s.Init == nil {
			break // assigned later
		}
		g.exprAs(s.Init, s.Sym.Type)
		if size(s.Sym.Type) > 0 {
			g.code.Local(storeOp(s.Sym.Type), s.Sym.Slot)
		}
	case *syntax.Destructure:
		for _, vs := range s.Stmts {
			g.stmt(vs)
		}
	case *syntax.Assign:
		g.assign(s)
	case *syntax.LocalFun:
		g.localFun(s)
	case *syntax.While:
		// In a replace group if the checker says so: see composeFlow.
		g.grouped(s.Group, g.siteKey(s.P, "loop", 0), func() { g.while(s) })
	case *syntax.For:
		scope := g.code.Scope()
		g.grouped(s.Group, g.siteKey(s.P, "loop", 0), func() { g.forLoop(s) })
		g.code.EndScope(scope)
	case *syntax.Block:
		g.block(s, types.UnitType, false)
	}
}

// while generates a while or do-while loop.
func (g *gen) while(s *syntax.While) {
	if s.Do {
		// The body, then the condition, which jumps back to it.
		top, cont, exit := cf.NewLabel(), cf.NewLabel(), cf.NewLabel()
		g.code.Place(top)
		g.pushLoop(s, exit, cont)
		g.loopBody(func() { g.stmts(s.Body.Stmts) })
		g.code.Place(cont)
		g.line(s.Cond.Pos())
		g.cond(s.Cond, top, true)
		g.code.Place(exit)
		return
	}
	top, exit := cf.NewLabel(), cf.NewLabel()
	g.code.Place(top)
	g.line(s.P)
	g.cond(s.Cond, exit, false)
	g.pushLoop(s, exit, top)
	g.loopBody(func() { g.block(s.Body, types.UnitType, false) })
	g.code.Jump(cf.GOTO, top)
	g.code.Place(exit)
}

// exprStmt generates an expression and discards its value.
func (g *gen) exprStmt(x syntax.Expr) {
	switch x := x.(type) {
	case *syntax.If:
		g.ifStmt(x)
		return
	case *syntax.When:
		g.when(x, types.UnitType, false)
		return
	case *syntax.Try:
		g.try(x, types.UnitType, false)
		return
	case *syntax.Unary:
		if x.Op == syntax.INC || x.Op == syntax.DEC {
			if x.Stmts != nil {
				g.stmts(x.Stmts) // x = x.inc()
				return
			}
			g.incDec(x)
			return
		}
	}
	g.expr(x)
	g.uncastErased()
	g.pop(x.Type())
}

func (g *gen) pop(t types.Type) {
	switch size(t) {
	case 1:
		g.code.Op(cf.POP)
	case 2:
		g.code.Op(cf.POP2)
	}
}

// ifStmt generates an if whose value is unused. Its branches run in
// replace groups if the checker says so, a missing else in an empty one.
func (g *gen) ifStmt(x *syntax.If) {
	els, end := cf.NewLabel(), cf.NewLabel()
	g.cond(x.Cond, els, false)
	g.grouped(x.Groups, g.siteKey(x.P, "if", 0), func() { g.block(x.Then, types.UnitType, false) })
	if x.Else == nil && !x.Groups {
		g.code.Place(els)
		return
	}
	g.code.Jump(cf.GOTO, end)
	g.code.Place(els)
	g.grouped(x.Groups, g.siteKey(x.P, "if", 1), func() {
		if x.Else != nil {
			g.block(x.Else, types.UnitType, false)
		}
	})
	g.code.Place(end)
}

// forEach generates a loop over a collection: it calls iterator(), then
// hasNext() and next() for each element. A loop over an iterator uses it.
func (g *gen) forEach(s *syntax.For) {
	if s.Iter != nil {
		g.suspendOperator(s.Iter, s.Range.Type().NonNull(), func() { g.expr(s.Range) })
		g.coerce(types.JVMRet(s.Iter), s.IterT)
	} else {
		g.expr(s.Range)
		g.coerce(s.Range.Type(), s.IterT)
	}
	it := g.code.AllocLocal(desc(s.IterT))
	g.code.Local(cf.ASTORE, it)
	s.Sym.Slot = g.code.AllocLocal(desc(s.Sym.Type))
	g.named(s.Sym)
	top, exit := cf.NewLabel(), cf.NewLabel()
	g.code.Place(top)
	g.line(s.P)
	g.suspendOperator(s.HasNext, s.IterT, func() { g.code.Local(cf.ALOAD, it) })
	g.coerce(types.JVMRet(s.HasNext), types.BoolType)
	g.code.Jump(cf.IFEQ, exit)
	g.suspendOperator(s.Next, s.IterT, func() { g.code.Local(cf.ALOAD, it) })
	g.coerce(types.JVMRet(s.Next), s.Sym.Type)
	g.code.Local(storeOp(s.Sym.Type), s.Sym.Slot)
	g.pushLoop(s, exit, top)
	g.loopBody(func() { g.block(s.Body, types.UnitType, false) })
	g.code.Jump(cf.GOTO, top)
	g.code.Place(exit)
}

// invokeOperator calls an operator function without parameters on a
// receiver of type rt on the stack: a method, or a top-level extension.
func (g *gen) invokeOperator(f *types.FunSym, rt types.Type) {
	if f.Ext {
		g.coerce(rt, f.Recv)
		g.invokeStatic(f.Facade, f, 0, false)
		return
	}
	g.invokeOperatorMember(f, rt)
}

// invokeOperatorMember calls member f on a receiver of type rt on the
// stack: for a value class of the program unboxed, its static method,
// which takes the value first.
func (g *gen) invokeOperatorMember(f *types.FunSym, rt types.Type) {
	if f.Impl && f.Inline == nil {
		// A class-path value class's: a static method, also taking the
		// value first, as UIntArray's iterator-impl.
		g.invokeStatic(f.Facade, f, 0, false)
		return
	}
	if o := f.Owner; o != nil && o.Value && f.Desc == "" && o.Underlying != nil && types.Unboxed(rt) {
		g.code.Invoke(cf.INVOKESTATIC, o.Name, f.JVM(), "("+desc(o.Underlying.Type)+funDesc(f)[1:])
		return
	}
	g.invokeJava(f, rt)
}

// pushLoop records loop s, whose break jumps to brk and continue to cont,
// for the jumps in its body.
func (g *gen) pushLoop(s syntax.Stmt, brk, cont *cf.Label) {
	g.loops = append(g.loops, loopLabels{node: s, brk: brk, cont: cont, depth: g.code.Depth(), groups: g.groupDepth()})
}

// loopBody generates the body of the loop pushLoop recorded last, with
// body, each run in a replace group if the checker says so, and forgets
// the loop.
func (g *gen) loopBody(body func()) {
	s := g.loops[len(g.loops)-1].node
	group := false
	switch s := s.(type) {
	case *syntax.While:
		group = s.BodyGroup
	case *syntax.For:
		group = s.BodyGroup
	}
	g.grouped(group, g.siteKey(syntax.StmtPos(s), "loop", 1), body)
	g.loops = g.loops[:len(g.loops)-1]
}

// stmts generates statements that a rewrite made.
func (g *gen) stmts(ss []syntax.Stmt) {
	for _, s := range ss {
		g.stmt(s)
	}
}

func (g *gen) forLoop(s *syntax.For) {
	if s.HasNext != nil {
		g.forEach(s)
		return
	}
	if s.Range.Type().NonNull().IsArray() {
		g.forArray(s)
		return
	}
	if s.Range.Type().NonNull() == types.StringType {
		g.forString(s)
		return
	}
	r := s.Range.(*syntax.Binary)
	op, step := r.Op, syntax.Expr(nil)
	if op == syntax.STEP {
		step = r.Y
		r = r.X.(*syntax.Binary)
		op = r.Op
	}
	if step != nil {
		g.forStepped(s, r, op, step, s.Sym.Type.K == types.Long)
		return
	}
	if s.Sym.Type.K == types.Long {
		g.forLongRange(s, r, op)
		return
	}
	i := g.code.AllocLocal("I")
	s.Sym.Slot = i
	g.named(s.Sym)
	g.exprAs(r.X, types.IntType)
	g.code.Local(cf.ISTORE, i)
	// As kotlinc does, a constant or a val's value is the bound itself;
	// anything else is computed once, into a local.
	var loadEnd func()
	if v, ok := constOf(r.Y, 0); ok && r.Y.Type().K == types.Int {
		loadEnd = func() { g.code.Iconst(int32(v.(int64))) }
	} else if p := stableLocal(r.Y); p != nil {
		loadEnd = func() { g.loadStorage(p) }
	} else {
		end := g.code.AllocLocal("I")
		g.exprAs(r.Y, types.IntType)
		g.code.Local(cf.ISTORE, end)
		loadEnd = func() { g.code.Local(cf.ILOAD, end) }
	}
	top, cont, exit := cf.NewLabel(), cf.NewLabel(), cf.NewLabel()
	// exitOp leaves the loop when i is past end.
	var exitOp byte
	switch op {
	case syntax.RANGE:
		exitOp = cf.IF_ICMPGT
	case syntax.RANGE_LT, syntax.UNTIL:
		exitOp = cf.IF_ICMPGE
	case syntax.DOWN_TO:
		exitOp = cf.IF_ICMPLT
	}
	inc := 1
	if op == syntax.DOWN_TO {
		inc = -1
	}
	// Inclusive ranges test for the last element before incrementing, so a
	// range ending at Int.MAX_VALUE does not overflow.
	inclusive := op == syntax.RANGE || op == syntax.DOWN_TO
	if inclusive {
		g.code.Local(cf.ILOAD, i)
		loadEnd()
		g.code.Jump(exitOp, exit)
		g.code.Place(top)
	} else {
		g.code.Place(top)
		g.code.Local(cf.ILOAD, i)
		loadEnd()
		g.code.Jump(exitOp, exit)
	}
	g.pushLoop(s, exit, cont)
	g.loopBody(func() { g.block(s.Body, types.UnitType, false) })
	g.code.Place(cont)
	g.line(s.P)
	if inclusive {
		g.code.Local(cf.ILOAD, i)
		loadEnd()
		g.code.Jump(cf.IF_ICMPEQ, exit)
	}
	g.code.Iinc(i, inc)
	g.code.Jump(cf.GOTO, top)
	g.code.Place(exit)
}

// forLongRange generates a for loop over a Long range, as forLoop does
// over an Int range, with the bound in a local.
func (g *gen) forLongRange(s *syntax.For, r *syntax.Binary, op syntax.Kind) {
	i := g.code.AllocLocal("J")
	s.Sym.Slot = i
	g.named(s.Sym)
	g.exprAs(r.X, types.LongType)
	g.code.Local(cf.LSTORE, i)
	end := g.code.AllocLocal("J")
	g.exprAs(r.Y, types.LongType)
	g.code.Local(cf.LSTORE, end)
	top, cont, exit := cf.NewLabel(), cf.NewLabel(), cf.NewLabel()
	// i compared with end: exitOp leaves the loop.
	cmp := func(exitOp byte) {
		g.code.Local(cf.LLOAD, i)
		g.code.Local(cf.LLOAD, end)
		g.code.Op(cf.LCMP)
		g.code.Jump(exitOp, exit)
	}
	var exitOp byte
	switch op {
	case syntax.RANGE:
		exitOp = cf.IFGT
	case syntax.RANGE_LT, syntax.UNTIL:
		exitOp = cf.IFGE
	case syntax.DOWN_TO:
		exitOp = cf.IFLT
	}
	inclusive := op == syntax.RANGE || op == syntax.DOWN_TO
	if inclusive {
		cmp(exitOp)
		g.code.Place(top)
	} else {
		g.code.Place(top)
		cmp(exitOp)
	}
	g.pushLoop(s, exit, cont)
	g.loopBody(func() { g.block(s.Body, types.UnitType, false) })
	g.code.Place(cont)
	g.line(s.P)
	if inclusive {
		cmp(cf.IFEQ)
	}
	g.code.Local(cf.LLOAD, i)
	g.code.Lconst(1)
	if op == syntax.DOWN_TO {
		g.code.Op(cf.LSUB)
	} else {
		g.code.Op(cf.LADD)
	}
	g.code.Local(cf.LSTORE, i)
	g.code.Jump(cf.GOTO, top)
	g.code.Place(exit)
}

// forStepped generates a for loop over range r with a step, an Int or a
// Long one, as kotlinc does: the step must be positive, an exclusive bound
// becomes an inclusive one, and the loop ends at the progression's last
// element, which getProgressionLastElement computes, so that it does not
// overflow.
func (g *gen) forStepped(s *syntax.For, r *syntax.Binary, op syntax.Kind, step syntax.Expr, long bool) {
	t, d, load, store, add, sub, neg, rem := types.IntType, "I", byte(cf.ILOAD), byte(cf.ISTORE), byte(cf.IADD), byte(cf.ISUB), byte(cf.INEG), byte(cf.IREM)
	if long {
		t, d, load, store, add, sub, neg, rem = types.LongType, "J", cf.LLOAD, cf.LSTORE, cf.LADD, cf.LSUB, cf.LNEG, cf.LREM
	}
	local := func(e syntax.Expr) int {
		slot := g.code.AllocLocal(d)
		g.exprAs(e, t)
		g.code.Local(store, slot)
		return slot
	}
	i := local(r.X)
	s.Sym.Slot = i
	g.named(s.Sym)
	last, st := local(r.Y), local(step)
	exit := cf.NewLabel()
	// cmpJump compares the values pushed and jumps to l on op, one of
	// IF_ICMPxx.
	cmpJump := func(op byte, l *cf.Label) {
		if long {
			g.code.Op(cf.LCMP)
			op -= cf.IF_ICMPEQ - cf.IFEQ
		}
		g.code.Jump(op, l)
	}
	zero := func() {
		if long {
			g.code.Lconst(0)
		} else {
			g.code.Iconst(0)
		}
	}
	if v, _ := constOf(step, 0); !isPositive(v) {
		ok := cf.NewLabel()
		g.code.Local(load, st)
		zero()
		cmpJump(cf.IF_ICMPGT, ok)
		if g.shared.opts.Stdlib {
			g.code.Iconst(0)
			g.code.Local(load, st)
			g.box(t)
			g.code.Invoke(cf.INVOKESTATIC, "kotlin/ranges/RangesKt", "checkStepIsPositive", "(ZLjava/lang/Number;)V")
		} else {
			g.code.New("java/lang/IllegalArgumentException")
			g.code.Op(cf.DUP)
			g.code.New("java/lang/StringBuilder")
			g.code.Op(cf.DUP)
			g.code.Sconst("Step must be positive, was: ")
			g.code.Invoke(cf.INVOKESPECIAL, "java/lang/StringBuilder", "<init>", "(Ljava/lang/String;)V")
			g.code.Local(load, st)
			g.code.Invoke(cf.INVOKEVIRTUAL, "java/lang/StringBuilder", "append", "("+d+")Ljava/lang/StringBuilder;")
			g.code.Sconst(".")
			g.code.Invoke(cf.INVOKEVIRTUAL, "java/lang/StringBuilder", "append", "(Ljava/lang/String;)Ljava/lang/StringBuilder;")
			g.code.Invoke(cf.INVOKEVIRTUAL, "java/lang/StringBuilder", "toString", "()Ljava/lang/String;")
			g.code.Invoke(cf.INVOKESPECIAL, "java/lang/IllegalArgumentException", "<init>", "(Ljava/lang/String;)V")
			g.code.Op(cf.ATHROW)
		}
		g.code.Place(ok)
	}
	if op == syntax.RANGE_LT || op == syntax.UNTIL {
		// Nothing is below MIN_VALUE.
		g.code.Local(load, last)
		if long {
			g.code.Lconst(math.MinInt64)
		} else {
			g.code.Iconst(math.MinInt32)
		}
		cmpJump(cf.IF_ICMPEQ, exit)
		g.code.Local(load, last)
		if long {
			g.code.Lconst(1)
		} else {
			g.code.Iconst(1)
		}
		g.code.Op(sub)
		g.code.Local(store, last)
	}
	if g.shared.opts.Stdlib {
		g.code.Local(load, i)
		g.code.Local(load, last)
		g.code.Local(load, st)
		if op == syntax.DOWN_TO {
			g.code.Op(neg)
		}
		g.code.Invoke(cf.INVOKESTATIC, "kotlin/internal/ProgressionUtilKt", "getProgressionLastElement", "("+d+d+d+")"+d)
		g.code.Local(store, last)
	} else {
		// getProgressionLastElement's arithmetic: last moves towards i
		// by the difference of i and last modulo the step.
		mod := func() { // the value on the stack modulo st, from 0
			ok := cf.NewLabel()
			g.code.Local(load, st)
			g.code.Op(rem)
			if long {
				g.code.Op(cf.DUP2)
				g.code.Lconst(0)
				g.code.Op(cf.LCMP)
			} else {
				g.code.Op(cf.DUP)
			}
			g.code.Jump(cf.IFGE, ok)
			g.code.Local(load, st)
			g.code.Op(add)
			g.code.Place(ok)
		}
		from, to, done := i, last, cf.NewLabel()
		skip := byte(cf.IF_ICMPGE)
		if op == syntax.DOWN_TO {
			from, to, skip = last, i, cf.IF_ICMPLE
		}
		g.code.Local(load, i)
		g.code.Local(load, last)
		cmpJump(skip, done)
		g.code.Local(load, last)
		g.code.Local(load, to)
		mod()
		g.code.Local(load, from)
		mod()
		g.code.Op(sub)
		mod()
		if op == syntax.DOWN_TO {
			g.code.Op(add)
		} else {
			g.code.Op(sub)
		}
		g.code.Local(store, last)
		g.code.Place(done)
	}
	g.code.Local(load, i)
	g.code.Local(load, last)
	if op == syntax.DOWN_TO {
		cmpJump(cf.IF_ICMPLT, exit)
	} else {
		cmpJump(cf.IF_ICMPGT, exit)
	}
	top, cont := cf.NewLabel(), cf.NewLabel()
	g.code.Place(top)
	g.pushLoop(s, exit, cont)
	g.loopBody(func() { g.block(s.Body, types.UnitType, false) })
	g.code.Place(cont)
	g.line(s.P)
	g.code.Local(load, i)
	g.code.Local(load, last)
	cmpJump(cf.IF_ICMPEQ, exit)
	g.code.Local(load, i)
	g.code.Local(load, st)
	if op == syntax.DOWN_TO {
		g.code.Op(sub)
	} else {
		g.code.Op(add)
	}
	g.code.Local(store, i)
	g.code.Jump(cf.GOTO, top)
	g.code.Place(exit)
}

// stableLocal returns the local or parameter that x reads if it is an
// Int val of the function being generated: its value can't change.
func stableLocal(x syntax.Expr) *types.VarSym {
	id, ok := x.(*syntax.Ident)
	if !ok || id.Sym == nil || id.Cast {
		return nil
	}
	v := id.Sym
	if (v.Kind != types.Local && v.Kind != types.Param) || v.Mutable || v.Delegate != nil || v.Type != types.IntType {
		return nil
	}
	return v
}

// Assignment.

// arithOp returns the instruction of an arithmetic operator on values of
// type t, which is Int, Long, Float or Double.
func arithOp(t types.Type, op syntax.Kind) byte {
	base := map[syntax.Kind][4]byte{
		syntax.PLUS:    {cf.IADD, cf.LADD, cf.FADD, cf.DADD},
		syntax.MINUS:   {cf.ISUB, cf.LSUB, cf.FSUB, cf.DSUB},
		syntax.STAR:    {cf.IMUL, cf.LMUL, cf.FMUL, cf.DMUL},
		syntax.SLASH:   {cf.IDIV, cf.LDIV, cf.FDIV, cf.DDIV},
		syntax.PERCENT: {cf.IREM, cf.LREM, cf.FREM, cf.DREM},
	}[op]
	switch t.K {
	case types.Long:
		return base[1]
	case types.Float:
		return base[2]
	case types.Double:
		return base[3]
	}
	return base[0]
}

func compoundOp(k syntax.Kind) syntax.Kind {
	switch k {
	case syntax.PLUS_EQ, syntax.INC:
		return syntax.PLUS
	case syntax.MINUS_EQ, syntax.DEC:
		return syntax.MINUS
	case syntax.STAR_EQ:
		return syntax.STAR
	case syntax.SLASH_EQ:
		return syntax.SLASH
	}
	return syntax.PERCENT
}

// lvalue describes how to read and write an assignment target. prepare
// pushes any receiver; for compound assignment it is duplicated first.
type lvalue struct {
	t       types.Type
	prepare func()
	recv    int // stack slots prepare pushed: a receiver, or an array and index
	load    func()
	store   func()
	// reread is set when reading calls an accessor, which ++x and --x
	// call again for their value after storing, as kotlinc does.
	reread bool
}

// dupRecv duplicates what prepare pushed, for compound assignment.
func (g *gen) dupRecv(lv lvalue) {
	switch lv.recv {
	case 1:
		g.code.Op(cf.DUP)
	case 2:
		g.code.Op(cf.DUP2)
	}
}

func (g *gen) lvalue(x syntax.Expr) lvalue {
	switch x := x.(type) {
	case *syntax.Index:
		return g.indexLvalue(x)
	case *syntax.Ident:
		sym := x.Sym
		t := sym.Type
		switch sym.Kind {
		case types.Local, types.Param:
			if vs, ok := sym.Decl.(*syntax.VarStmt); ok && sym.Delegate != nil {
				// A local delegated var: its delegate's getValue and
				// setValue, which takes the value from a hidden local.
				return lvalue{t: t, prepare: func() {}, reread: true,
					load: func() {
						g.expr(vs.Get)
						g.coerce(vs.Get.T, t)
					},
					store: func() {
						v := vs.ValueSym
						v.Slot = g.code.AllocLocal(desc(t))
						g.code.Local(storeOp(t), v.Slot)
						g.expr(vs.Set)
						g.pop(vs.Set.T)
						g.code.Retire(v.Slot)
					}}
			}
			if sym.Boxed() {
				return lvalue{t: t, recv: 2,
					prepare: func() {
						g.loadStorage(sym)
						g.code.Iconst(0)
					},
					load:  func() { g.arrayLoad(t) },
					store: func() { g.arrayStore(t) }}
			}
			return lvalue{t: t, prepare: func() {},
				load:  func() { g.code.Local(loadOp(t), sym.Slot) },
				store: func() { g.code.Local(storeOp(t), sym.Slot) }}
		case types.BackingField:
			p := sym.Prop
			if p.Kind == types.Global {
				return lvalue{t: t, prepare: func() {},
					load:  func() { g.backingField(p, true) },
					store: func() { g.backingField(p, false) }}
			}
			if p.StaticField() {
				owner, ft := p.ConstOwner().Name, types.FieldType(p)
				return lvalue{t: t, prepare: func() {},
					load: func() { g.backingField(p, true) },
					store: func() {
						g.coerce(p.Type, ft)
						g.code.FieldInsn(cf.PUTSTATIC, owner, p.Name, desc(ft))
					}}
			}
			return lvalue{t: t, prepare: g.loadThis, recv: 1,
				load:  func() { g.getField(p) },
				store: func() { g.putField(p) }}
		case types.Global:
			return g.globalLvalue(sym)
		case types.Prop:
			return g.propLvalue(sym, x.T, g.loadThis, g.thisClass(), true)
		}
	case *syntax.Member:
		if f := x.Field; f != nil {
			// A static field: a Java one, or a @JvmField of an object
			// of the class path.
			owner, t := x.Static.Name, x.T
			return lvalue{t: t, prepare: func() {},
				load: func() {
					g.code.FieldInsn(cf.GETSTATIC, owner, f.Name, desc(f.Type))
					g.coerce(f.Type, t)
				},
				store: func() {
					g.coerce(t, f.Type)
					g.code.FieldInsn(cf.PUTSTATIC, owner, f.Name, desc(f.Type))
				}}
		}
		if f, s := x.Getter, x.Setter; f != nil && f.Ext && s != nil {
			// A var extension property: its accessors take the receiver,
			// after the dispatch receiver of one declared in a class.
			rt, t := x.X.Type(), x.T
			prepare := func() {
				g.expr(x.X)
				g.coerce(rt, f.Recv)
			}
			lv := lvalue{t: t, prepare: prepare, recv: size(f.Recv), reread: true,
				load: func() {
					g.invokeFun(nil, f, 0)
					g.coerce(types.JVMRet(f), t)
				},
				store: func() {
					g.coerce(t, s.Params[0].Type)
					g.invokeFun(nil, s, 0)
				}}
			if gd := syntax.SourceInline(f); gd != nil {
				// Accessors with reified type parameters: copies of their
				// bodies, as of calls of inline functions. The value is
				// the setter's argument, from a local.
				get := &syntax.Call{Fun: x, Sym: f, Reified: x.Reified}
				get.P, get.T = x.P, t
				lv.load = func() {
					g.inlineBody(get, gd)
					g.coerce(f.Ret, t)
				}
				lv.store = func() {
					pt := s.Params[0].Type
					g.coerce(t, pt)
					v := &types.VarSym{Name: s.Params[0].Name, Type: pt, Kind: types.Local, Slot: g.code.AllocLocal(desc(pt))}
					g.code.Local(storeOp(pt), v.Slot)
					arg := &syntax.Ident{Name: v.Name, Sym: v}
					arg.P, arg.T = x.P, pt
					set := &syntax.Call{Fun: x, Sym: s, Args: []syntax.Expr{arg}, Reified: x.Reified}
					set.P, set.T = x.P, types.UnitType
					g.inlineBody(set, syntax.SourceInline(s))
					g.pop(s.Ret)
				}
			}
			if f.Owner != nil {
				lv.prepare, lv.recv = func() {
					g.dispatch(x, f.Owner)
					prepare()
				}, 2
				lv.load = func() {
					g.callMethod(f, f.Owner, f.JVM(), funDesc(f))
					g.coerce(types.JVMRet(f), t)
				}
				lv.store = func() {
					g.coerce(t, s.Params[0].Type)
					g.callMethod(s, s.Owner, s.JVM(), funDesc(s))
				}
			}
			return lv
		}
		if s, ok := x.X.(*syntax.Super); ok {
			// super.p = v calls the supertype's accessors directly.
			t := x.T.Flex()
			lv := lvalue{t: t, prepare: func() { g.superThis(s) }, recv: 1}
			if f := x.Getter; f != nil {
				param := x.Setter.Params[0].Type
				lv.load = func() {
					g.invokeSuperBody(s, f.JVM(), f.Desc)
					g.coerce(types.JVMRet(f), t)
				}
				lv.store = func() {
					g.coerce(t, param)
					g.invokeSuperBody(s, x.Setter.JVM(), x.Setter.Desc)
					g.pop(types.JVMRet(x.Setter))
				}
				return lv
			}
			p := x.Prop
			lv.load = func() {
				g.invokeSuperBody(s, types.GetterJVM(p), "()"+desc(types.GetterType(p)))
				g.coerce(types.GetterType(p), t)
			}
			lv.store = func() {
				g.coerce(t, types.FieldType(p))
				g.invokeSuperBody(s, types.SetterJVM(p), "("+desc(types.FieldType(p))+")V")
			}
			return lv
		}
		if x.Getter != nil {
			// A Java property: getX() and setX(v).
			rt := x.X.Type().NonNull()
			t := x.T.Flex()
			param := x.Setter.Params[0].Type
			return lvalue{t: t, prepare: func() { g.expr(x.X) }, recv: 1, reread: true,
				load: func() {
					g.invokeJava(x.Getter, rt)
					g.coerce(types.JVMRet(x.Getter), t)
				},
				store: func() {
					g.coerce(t, param)
					g.invokeJava(x.Setter, rt)
					g.pop(types.JVMRet(x.Setter)) // a setter that returns a value
				}}
		}
		if p := x.Prop; p.Kind == types.Global {
			return g.globalLvalue(p)
		}
		_, this := x.X.(*syntax.This)
		return g.propLvalue(x.Prop, x.T, func() { g.expr(x.X) }, recvClass(x.X.Type()), this)
	}
	panic("bad assignment target")
}

// globalLvalue accesses top-level property p: its field, or with a
// setter, a delegated property's accessors.
func (g *gen) globalLvalue(p *types.VarSym) lvalue {
	ft := types.FieldType(p)
	d := desc(ft)
	if p.Setter != nil {
		return lvalue{t: p.Type, prepare: func() {}, reread: true,
			load: func() { g.global(p) },
			store: func() {
				g.coerce(p.Type, ft)
				g.code.Invoke(cf.INVOKESTATIC, p.Facade, p.Setter.JVM(), "("+d+")V")
			}}
	}
	if g.globalField(p) {
		return lvalue{t: p.Type, prepare: func() {},
			load: func() {
				g.code.FieldInsn(cf.GETSTATIC, p.FieldClass(), p.Name, d)
				g.coerce(ft, p.Type)
			},
			store: func() {
				g.coerce(p.Type, ft)
				g.code.FieldInsn(cf.PUTSTATIC, p.FieldClass(), p.Name, d)
			}}
	}
	set := types.SetterJVM(p)
	if p.Private || p.SetterPrivate {
		g.shared.access[p] |= 2
		set = privateSetter(p)
	}
	return lvalue{t: p.Type, prepare: func() {}, reread: true,
		load: func() { g.global(p) },
		store: func() {
			g.coerce(p.Type, ft)
			g.code.Invoke(cf.INVOKESTATIC, p.Facade, set, "("+d+")V")
		}}
}

// propLvalue accesses property p as type t, which differs from p's type
// when p's type mentions a type parameter of its class: directly in its
// field where this is the receiver and its accessor allows it, or else
// through the accessors of a receiver of class cls.
func (g *gen) propLvalue(p *types.VarSym, t types.Type, recv func(), cls *types.ClassSym, this bool) lvalue {
	ft := types.FieldType(p)
	d := desc(ft)
	lv := lvalue{t: t, prepare: recv, recv: 1}
	switch {
	case this && p.StaticField() && g.directGet(p) && g.directSet(p):
		// An object's field: no receiver.
		owner := p.ConstOwner().Name
		lv.prepare, lv.recv = func() {}, 0
		lv.load = func() {
			g.code.FieldInsn(cf.GETSTATIC, owner, p.Name, d)
			g.lateinit(p)
			g.coerce(ft, p.Type)
		}
		lv.store = func() {
			g.coerce(p.Type, ft)
			g.code.FieldInsn(cf.PUTSTATIC, owner, p.Name, d)
		}
	case this && g.directGet(p):
		lv.load = func() {
			g.getField(p)
			g.lateinit(p)
		}
	default:
		lv.load = func() { g.getProp(cls, p) }
		lv.reread = true
	}
	switch {
	case lv.store != nil:
	case this && g.directSet(p):
		lv.store = func() { g.putField(p) }
	default:
		lv.store = func() { g.setProp(cls, p) }
	}
	load, store := lv.load, lv.store
	lv.load = func() {
		load()
		g.coerce(p.Type, t)
	}
	lv.store = func() {
		g.coerce(t, p.Type)
		store()
	}
	return lv
}

// accessCall returns the call of an accessor that x, the read or the
// assignment of a property with context parameters, is, or nil.
func accessCall(x syntax.Expr) *syntax.Call {
	switch x := x.(type) {
	case *syntax.Ident:
		return x.Call
	case *syntax.Member:
		return x.Call
	}
	return nil
}

func (g *gen) assign(s *syntax.Assign) {
	if s.Stmts != nil {
		g.stmts(s.Stmts) // operator calls
		return
	}
	if ix, ok := s.Target.(*syntax.Index); ok && s.Op == syntax.ASSIGN {
		g.call(ix.Call) // x[i] = v is x.set(i, v)
		g.pop(ix.Call.T)
		return
	}
	if call := accessCall(s.Target); call != nil {
		g.call(call) // a property with context parameters: x.setP(v)
		g.pop(call.T)
		return
	}
	if id, ok := s.Target.(*syntax.Ident); ok && id.Sym != nil && id.Sym.UnitLocal() {
		g.exprAs(s.Value, types.UnitType)
		return
	}
	lv := g.lvalue(s.Target)
	lv.prepare()
	if s.Op == syntax.ASSIGN {
		g.exprAs(s.Value, lv.t)
		lv.store()
		return
	}
	g.dupRecv(lv)
	lv.load()
	if lv.t.Flex() == types.StringType {
		g.stringValueOf(s.Value)
		g.code.Invoke(cf.INVOKEVIRTUAL, "java/lang/String", "concat", "(Ljava/lang/String;)Ljava/lang/String;")
	} else {
		// Computed in the operator's type: c += 1 on a Char adds Ints.
		op := s.OpType
		g.coerce(lv.t, op)
		g.exprAs(s.Value, op)
		g.code.Op(arithOp(op, compoundOp(s.Op)))
		g.convert(op, lv.t.Flex())
		g.coerce(lv.t.Flex(), lv.t)
	}
	lv.store()
}

// incDecValue generates x++ or ++x as a value: the old or the new value,
// kept below the receiver that the store takes.
func (g *gen) incDecValue(u *syntax.Unary) {
	lv := g.lvalue(u.X)
	lv.prepare()
	reread := u.Prefix && lv.reread
	if reread {
		g.dupRecv(lv) // the receiver of the read after the store
	}
	g.dupRecv(lv)
	lv.load()
	// A platform type's value, boxed, is computed with unboxed.
	t := lv.t.Flex()
	if !u.Prefix {
		g.dupBelow(lv, lv.t)
	}
	g.coerce(lv.t, t)
	g.addOne(u, t)
	g.coerce(t, lv.t)
	if u.Prefix && !reread {
		g.dupBelow(lv, lv.t)
	}
	lv.store()
	if reread {
		lv.load()
	}
}

// dupBelow duplicates the value of type t on top of the stack below the
// receiver slots of lv.
func (g *gen) dupBelow(lv lvalue, t types.Type) {
	wide := size(t) == 2
	switch {
	case lv.recv == 0 && !wide:
		g.code.Op(cf.DUP)
	case lv.recv == 0:
		g.code.Op(cf.DUP2)
	case lv.recv == 1 && !wide:
		g.code.Op(cf.DUP_X1)
	case lv.recv == 1:
		g.code.Op(cf.DUP2_X1)
	case !wide:
		g.code.Op(cf.DUP_X2)
	default:
		g.code.Op(cf.DUP2_X2)
	}
}

// addOne adds or subtracts 1 from the value of type t on top of the stack.
func (g *gen) addOne(u *syntax.Unary, t types.Type) {
	switch t.K {
	case types.Long:
		g.code.Lconst(1)
	case types.Float:
		g.code.Fconst(1)
	case types.Double:
		g.code.Dconst(1)
	default:
		g.code.Iconst(1)
	}
	g.code.Op(arithOp(t, compoundOp(u.Op)))
	g.convert(types.Wider(t, t), t) // Byte, Short and Char keep their low bits
}

func (g *gen) incDec(u *syntax.Unary) {
	if id, ok := u.X.(*syntax.Ident); ok && id.Sym.Type == types.IntType && !id.Sym.Boxed() && id.Sym.Delegate == nil &&
		(id.Sym.Kind == types.Local || id.Sym.Kind == types.Param) {
		inc := 1
		if u.Op == syntax.DEC {
			inc = -1
		}
		g.code.Iinc(id.Sym.Slot, inc)
		return
	}
	lv := g.lvalue(u.X)
	lv.prepare()
	reread := u.Prefix && lv.reread
	if reread {
		g.dupRecv(lv) // the receiver of the read after the store
	}
	g.dupRecv(lv)
	lv.load()
	t := lv.t.Flex()
	g.coerce(lv.t, t)
	switch t.K {
	case types.Long:
		g.code.Lconst(1)
	case types.Float:
		g.code.Fconst(1)
	case types.Double:
		g.code.Dconst(1)
	default:
		g.code.Iconst(1)
	}
	g.code.Op(arithOp(t, compoundOp(u.Op)))
	g.convert(types.Wider(t, t), t) // Byte, Short and Char keep their low bits
	g.coerce(t, lv.t)
	lv.store()
	if reread {
		lv.load()
		g.pop(lv.t)
	}
}

// isPositive reports whether v, a constant's value, is an integer above 0.
func isPositive(v any) bool {
	n, ok := v.(int64)
	return ok && n > 0
}
