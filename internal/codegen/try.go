package codegen

import (
	cf "github.com/barteo/kotlin-compiler-go/internal/classfile"
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// tryCtx is a try block being generated, for the jumps out of it.
type tryCtx struct {
	finally *syntax.Block // nil without a finally block
	// copyFinally, if set, generates the finally block of an inlined
	// body's try, which an inlined lambda's jumps out of it run.
	copyFinally func()
	loops       int  // len(g.loops) at the start of the try
	source      bool // a try of the program's code, not of an inlined body
	// gaps are the code in the try that its handlers don't protect: the
	// copies of finally blocks that jumps out of the try run.
	gaps [][2]int
}

// ranges returns the offsets in [start, end) that the try's handlers
// protect: all but the gaps.
func (t *tryCtx) ranges(start, end int) [][2]int {
	out := [][2]int{{start, end}}
	for _, gap := range t.gaps {
		var next [][2]int
		for _, r := range out {
			if gap[1] <= r[0] || gap[0] >= r[1] {
				next = append(next, r)
				continue
			}
			if r[0] < gap[0] {
				next = append(next, [2]int{r[0], gap[0]})
			}
			if gap[1] < r[1] {
				next = append(next, [2]int{gap[1], r[1]})
			}
		}
		out = next
	}
	return out
}

// try generates try { } catch (e: E) { } finally { }, as kotlinc does: the
// finally block is copied to every exit (the normal ends of the try and
// catch blocks, each jump out, and a handler of any exception, which
// rethrows it). Used as a value, the values on the operand stack, which
// a handler would lose, are kept in locals meanwhile.
func (g *gen) try(x *syntax.Try, t types.Type, value bool) {
	spilled, ok := g.code.Spill()
	if !ok {
		panic("a try expression can't be an argument of a constructor call")
	}
	snap := g.code.Snapshot()
	end := cf.NewLabel()
	if value {
		end = join(t)
	}
	ctx := &tryCtx{finally: x.Finally, loops: len(g.loops), source: true}
	g.tries = append(g.tries, ctx)
	start := g.code.Pos()
	g.block(x.Body, t, value)
	ranges := ctx.ranges(start, g.code.Pos())
	all := ranges
	g.normalExit(t, value, end)
	for _, cb := range x.Catches {
		h := cf.NewLabel()
		g.code.Catch(snap, ranges, h, instanceClass(cb.Sym.Type))
		g.code.Place(h)
		cstart := g.code.Pos()
		scope := g.code.Scope()
		cb.Sym.Slot = g.code.AllocLocal(desc(cb.Sym.Type))
		g.named(cb.Sym)
		g.code.Local(cf.ASTORE, cb.Sym.Slot)
		g.block(cb.Body, t, value)
		g.code.EndScope(scope)
		all = append(all, ctx.ranges(cstart, g.code.Pos())...)
		g.normalExit(t, value, end)
	}
	g.tries = g.tries[:len(g.tries)-1]
	if x.Finally != nil {
		h := cf.NewLabel()
		g.code.Catch(snap, all, h, "")
		g.code.Place(h)
		exc := g.code.AllocLocal("Ljava/lang/Throwable;")
		g.code.Local(cf.ASTORE, exc)
		g.block(x.Finally, types.UnitType, false)
		g.code.Local(cf.ALOAD, exc)
		g.code.Op(cf.ATHROW)
	}
	g.code.Place(end)
	if len(spilled) > 0 && g.code.Reachable() {
		// Put the values back below the try's value.
		tmp := -1
		if value && size(t) > 0 {
			tmp = g.code.AllocLocal(desc(t))
			g.code.Local(storeOp(t), tmp)
		}
		g.code.Unspill(spilled)
		if tmp >= 0 {
			g.code.Local(loadOp(t), tmp)
			g.code.Retire(tmp)
		}
	}
}

// normalExit ends a try or catch block that completes normally: it runs
// the innermost try's finally block, keeping the block's value, and jumps
// to end.
func (g *gen) normalExit(t types.Type, value bool, end *cf.Label) {
	if !g.code.Reachable() {
		return
	}
	if g.tries[len(g.tries)-1].finally != nil {
		tmp := -1
		if value && size(t) > 0 {
			tmp = g.code.AllocLocal(desc(t))
			g.code.Local(storeOp(t), tmp)
		}
		g.inlineFinally(len(g.tries) - 1)
		if tmp >= 0 {
			g.code.Local(loadOp(t), tmp)
			g.code.Retire(tmp)
		}
	}
	g.code.Jump(cf.GOTO, end)
}

// inlineFinally generates a copy of the finally block of g.tries[i]. The
// copy is outside that try and the ones in it, so a jump in it doesn't run
// the block again.
func (g *gen) inlineFinally(i int) {
	saved := g.tries
	g.tries = g.tries[:i:i] // a try in the copy, or an inlined call's, mustn't overwrite saved[i]
	if saved[i].copyFinally != nil {
		saved[i].copyFinally()
	} else {
		g.block(saved[i].finally, types.UnitType, false)
	}
	g.tries = saved
}

// leaveTries runs the finally blocks of g.tries[from:], innermost first,
// before a jump out of them. Each copy is a gap in the tries it leaves.
func (g *gen) leaveTries(from int) {
	for i := len(g.tries) - 1; i >= from; i-- {
		if !g.tries[i].hasFinally() {
			continue
		}
		start := g.code.Pos()
		g.inlineFinally(i)
		gap := [2]int{start, g.code.Pos()}
		for _, t := range g.tries[i:] {
			t.gaps = append(t.gaps, gap)
		}
	}
}

// hasFinally reports whether a try in g.tries[from:] has a finally block.
func (g *gen) hasFinally(from int) bool {
	for _, t := range g.tries[from:] {
		if t.hasFinally() {
			return true
		}
	}
	return false
}

// hasFinally reports whether the try has a finally block, its own or an
// inlined body's.
func (t *tryCtx) hasFinally() bool { return t.finally != nil || t.copyFinally != nil }

// loopTries returns the index of the first try that a break or continue
// of loop g.loops[loop] leaves.
func (g *gen) loopTries(loop int) int {
	for i, t := range g.tries {
		if t.loops > loop {
			return i
		}
	}
	return len(g.tries)
}

// assertion generates require, check, error, TODO, requireNotNull and
// checkNotNull, whose bodies kc knows: the standard library declares
// them inline-only.
func (g *gen) assertion(x *syntax.Call) {
	exc := x.Sym.Facade
	args, msg := syntax.AssertArgs(x)
	// throw pushes the exception's message and throws it.
	throw := func(message string) {
		if l, ok := msg.(*syntax.Lambda); ok && l.Inline {
			// An inlined lazy message, before the exception, as kotlinc
			// does: it may suspend.
			g.inlineLambda(l)
			g.code.Invoke(cf.INVOKESTATIC, "java/lang/String", "valueOf", "(Ljava/lang/Object;)Ljava/lang/String;")
			s := g.code.AllocLocal("Ljava/lang/String;")
			g.code.Local(cf.ASTORE, s)
			g.code.New(exc)
			g.code.Op(cf.DUP)
			g.code.Local(cf.ALOAD, s)
			g.code.Invoke(cf.INVOKESPECIAL, exc, "<init>", "(Ljava/lang/String;)V")
			g.code.Op(cf.ATHROW)
			return
		}
		g.code.New(exc)
		g.code.Op(cf.DUP)
		if msg != nil {
			// The lazy message is created and called only on failure.
			g.expr(msg)
			g.code.InvokeInterface(msg.Type().C.Name, "invoke", "()Ljava/lang/Object;")
			g.code.Invoke(cf.INVOKESTATIC, "java/lang/String", "valueOf", "(Ljava/lang/Object;)Ljava/lang/String;")
		} else {
			g.code.Sconst(message)
		}
		g.code.Invoke(cf.INVOKESPECIAL, exc, "<init>", "(Ljava/lang/String;)V")
		g.code.Op(cf.ATHROW)
	}
	switch x.Builtin {
	case types.Require, types.Check:
		ok := cf.NewLabel()
		g.cond(args[0], ok, true)
		if x.Builtin == types.Require {
			throw("Failed requirement.")
		} else {
			throw("Check failed.")
		}
		g.code.Place(ok)
	case types.RequireNotNull, types.CheckNotNull:
		at := args[0].Type()
		g.expr(args[0])
		if types.MayBeNull(at) {
			ok := cf.NewLabel()
			g.code.Op(cf.DUP)
			g.code.Jump(cf.IFNONNULL, ok)
			throw("Required value was null.")
			g.code.Place(ok)
		}
		g.coerce(at, x.T)
	case types.ErrorCall:
		g.code.New(exc)
		g.code.Op(cf.DUP)
		g.exprAs(args[0], types.AnyType.Nullable())
		g.code.Invoke(cf.INVOKESTATIC, "java/lang/String", "valueOf", "(Ljava/lang/Object;)Ljava/lang/String;")
		g.code.Invoke(cf.INVOKESPECIAL, exc, "<init>", "(Ljava/lang/String;)V")
		g.code.Op(cf.ATHROW)
	case types.Todo:
		if len(args) == 0 {
			throw("An operation is not implemented.")
			return
		}
		g.code.New(exc)
		g.code.Op(cf.DUP)
		g.newStringBuilder()
		g.code.Sconst("An operation is not implemented: ")
		g.code.Invoke(cf.INVOKEVIRTUAL, "java/lang/StringBuilder", "append", appendDesc(types.StringType))
		g.appendExpr(args[0])
		g.code.Invoke(cf.INVOKEVIRTUAL, "java/lang/StringBuilder", "toString", "()Ljava/lang/String;")
		g.code.Invoke(cf.INVOKESPECIAL, exc, "<init>", "(Ljava/lang/String;)V")
		g.code.Op(cf.ATHROW)
	}
}
