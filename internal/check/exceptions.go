package check

import (
	"maps"
	"slices"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// tryExpr checks try { } catch (e: E) { } finally { }. Used as a value,
// its type is the join of the try block's and the catch blocks'; the
// finally block's value is discarded.
func (c *Checker) tryExpr(x *syntax.Try, want types.Type, valueUsed bool) types.Type {
	sink := c.takeBranchSink()
	if valueUsed && c.ctorArgs {
		// Its handlers would start with an empty stack, without the
		// object under construction.
		c.unsupported(x.P, "try in constructor argument", "a try expression can't be an argument of a constructor call")
	}
	// The catch and finally blocks don't know how far the body got: its
	// assignments drop the vars' narrowing there, and after the try.
	bodyVars := c.watchedVars(x.Body)
	allVars := c.watchedVars(x)
	var inBody []narrow
	blocks := []*syntax.Block{x.Body}
	for _, cb := range x.Catches {
		blocks = append(blocks, cb.Body)
	}
	var handlerVars []*types.VarSym
	var inFinally []narrow
	if x.Finally != nil {
		handlerVars = c.watchedVars(x.Body)
		for _, cb := range x.Catches {
			for _, v := range c.watchedVars(cb.Body) {
				if !slices.Contains(handlerVars, v) {
					handlerVars = append(handlerVars, v)
				}
			}
		}
	}
	// After the try, the smart casts are those where its body and its
	// catch blocks end, of any value, as x as A in a try whose catch
	// blocks jump makes x an A after it, past its finally block, whose
	// vars have its, and whose own smart casts add to them: an x that is
	// a C there is an A & C.
	var ends []*branchExit
	var finallyEnd *branchExit
	defer func() {
		c.reset(allVars)
		var states []flowState
		for _, e := range ends {
			if e != nil && !e.jumped && e.state != nil {
				states = append(states, e.state)
			}
		}
		var fvars []*types.VarSym
		if x.Finally != nil {
			fvars = c.watchedVars(x.Finally)
		}
		finallyOK := finallyEnd != nil && !finallyEnd.jumped && finallyEnd.state != nil
		if len(states) == 0 {
			// The code after is unreachable, but checked, as K2 does,
			// with the finally block's smart casts.
			if finallyOK {
				c.scope.narrows = append(c.scope.narrows, c.finallyNarrows(finallyEnd.state, nil, fvars)...)
			}
			return
		}
		after := c.joinFlows(states)
		if finallyEnd != nil {
			kept := after[:0:0]
			for _, n := range after {
				if n.recv != nil || !slices.Contains(fvars, n.sym) {
					kept = append(kept, n)
				}
			}
			after = kept
			if finallyOK {
				for _, v := range fvars {
					if t, ok := finallyEnd.state[flowKey{sym: v}]; ok && t != v.Type {
						after = append(after, narrow{sym: v, t: t})
					}
				}
				after = append(after, c.finallyNarrows(finallyEnd.state, after, fvars)...)
			}
		}
		c.scope.narrows = append(c.scope.narrows, after...)
	}()
	exits := c.loopExits()
	if x.Finally != nil {
		// A break or a continue in the body or a catch block runs the
		// finally block, which may assign vars, before it jumps.
		c.finallies = append(c.finallies, finallyCtx{loops: len(c.loops), vars: c.watchedVars(x.Finally)})
	}
	ends = append(ends, c.watchExit(nil))
	c.sinkTail(x.Body, sink)
	// Without a sink, the body and the catch blocks are inferred together
	// as an if's branches: try { list } catch (e: E) { emptyList() }.
	var pending *[]pendingBranch
	if valueUsed && sink == nil {
		pending = &[]pendingBranch{}
	}
	block := func(b *syntax.Block) types.Type {
		if pending != nil {
			return c.ifBranch(b, want, valueUsed, true, pending)
		}
		return c.block(b, want, valueUsed)
	}
	result := block(x.Body)
	if len(x.Catches) > 0 {
		inBody = c.tryAssigned(bodyVars, x.Body)
	}
	allJump := result.K == types.Nothing
	for _, cb := range x.Catches {
		c.localAnnotations(cb.Annotations, catchSite)
		t := c.resolveType(cb.Type)
		if t.IsParam() && t.Param() != nil && !t.Param().Reified {
			// The JVM's exception table names a class, which only an
			// inlined copy knows.
			c.errorf(cb.Type.P, "non-reified type parameters cannot be used by catch parameters")
			t = types.InvalidType
		}
		if t.K == types.Nothing && !t.N {
			c.errorf(cb.Type.P, "throwable type mismatch: actual type is 'Nothing'")
			t = types.InvalidType
		}
		if t.K != types.Invalid && !c.throwable(t) {
			c.errorf(cb.Type.P, "the type of a catch parameter must be a subtype of Throwable")
			t = types.InvalidType
		}
		if t.N {
			c.errorf(cb.Type.P, "a catch parameter can't be nullable")
		}
		cb.Sym = &types.VarSym{Name: cb.Name, Type: t.NonNull(), Kind: types.Local, Decl: cb}
		c.push()
		c.reset(bodyVars)
		c.scope.narrows = append(c.scope.narrows, inBody...)
		if cb.Name != "_" || cb.Quoted {
			// An unnamed catch parameter, _, declares no name.
			c.scope.declare(cb.Sym)
		}
		ends = append(ends, c.watchExit(nil))
		bt := block(cb.Body)
		c.pop()
		allJump = allJump && bt.K == types.Nothing
		if valueUsed {
			j, ok := c.joinBranch(result, bt, want, x.P)
			if !ok {
				c.errorf(cb.Body.P, "type mismatch: 'try' and 'catch' blocks have incompatible types %s and %s", result, bt)
				if x.Finally != nil {
					c.finallies = c.finallies[:len(c.finallies)-1]
				}
				return types.InvalidType
			}
			result = j
		}
	}
	if pending != nil {
		result = c.completeBranches(*pending, result, want, true)
	}
	if x.Finally != nil {
		c.finallies = c.finallies[:len(c.finallies)-1]
		inFinally = c.tryAssigned(handlerVars, blocks...)
		c.push()
		c.reset(handlerVars)
		c.scope.narrows = append(c.scope.narrows, inFinally...)
		finallyEnd = c.watchExit(nil)
		// The finally block's own breaks and continues are kept apart
		// from those of the try and its catch blocks.
		saved := c.takeLoopExits(exits)
		ft := c.block(x.Finally, noWant, false)
		c.pop()
		if ft.K == types.Nothing {
			// A break or continue in the try or its catch blocks runs the
			// finally block, which jumps instead: it doesn't leave the
			// loop.
			return types.NothingType
		}
		// They jump once the finally block has run, with its smart casts.
		if finallyEnd.state != nil {
			fvars := c.watchedVars(x.Finally)
			for _, j := range saved {
				for _, sts := range [][]flowState{j.breaks, j.continues} {
					for i, st := range sts {
						sts[i] = c.withFinally(st, finallyEnd.state, fvars)
					}
				}
			}
		}
		c.addLoopExits(saved)
	}
	if !valueUsed {
		if allJump {
			return types.NothingType
		}
		return types.UnitType
	}
	return result
}

// finallyNarrows returns the smart casts that a finally block, which
// ends in state, adds to those after its try's body and catch blocks,
// after, of the values it doesn't assign, fvars: of a value that after
// narrows too, the intersection of its types. They are sorted, as
// joinFlows's are.
func (c *Checker) finallyNarrows(state flowState, after []narrow, fvars []*types.VarSym) []narrow {
	var out []narrow
	for k, t := range state {
		if k.sym == nil || k.recv == nil && slices.Contains(fvars, k.sym) || k.recv != nil && slices.Contains(fvars, k.recv) {
			continue
		}
		cur := c.scope.narrowedMember(k.recv, k.sym, k.sym.Type)
		for _, n := range after {
			if n.recv == k.recv && n.sym == k.sym {
				cur = n.t
			}
		}
		if c.subtype(cur, t) {
			continue
		}
		out = append(out, narrow{recv: k.recv, sym: k.sym, t: c.isNarrowed(cur, t)})
	}
	slices.SortFunc(out, cmpNarrow)
	return out
}

// withFinally returns flow state st, where a break or a continue in a
// try or a catch block jumps from, with the smart casts that its finally
// block, which ends in fstate, adds to those of the values it doesn't
// assign, fvars: of a value narrowed in both, the intersection of its
// types.
func (c *Checker) withFinally(st, fstate flowState, fvars []*types.VarSym) flowState {
	var out flowState
	for k, t := range fstate {
		if k.sym == nil || k.recv == nil && slices.Contains(fvars, k.sym) || k.recv != nil && slices.Contains(fvars, k.recv) {
			continue
		}
		cur, ok := st[k]
		if !ok {
			cur = k.sym.Type
		}
		if c.subtype(cur, t) {
			continue
		}
		if out == nil {
			out = maps.Clone(st)
		}
		out[k] = c.isNarrowed(cur, t)
	}
	if out == nil {
		return st
	}
	return out
}

// loopExit is what a loop knows of its breaks and continues.
type loopExit struct {
	hasBreak, hasContinue bool
	breaks, continues     int // the number of their smart casts
}

// loopExits returns the breaks and continues each of the loops the code
// is in has so far.
func (c *Checker) loopExits() []loopExit {
	exits := make([]loopExit, len(c.loops))
	for i, l := range c.loops {
		exits[i] = loopExit{l.hasBreak, l.hasContinue, len(l.flow.breaks), len(l.flow.continues)}
	}
	return exits
}

// takeLoopExits returns the breaks and continues the loops have got
// since exits, and restores them to exits.
func (c *Checker) takeLoopExits(exits []loopExit) []loopJumps {
	taken := make([]loopJumps, len(exits))
	for i, e := range exits {
		l := c.loops[i]
		taken[i] = loopJumps{
			hasBreak: l.hasBreak, hasContinue: l.hasContinue,
			breaks: slices.Clone(l.flow.breaks[e.breaks:]), continues: slices.Clone(l.flow.continues[e.continues:]),
		}
	}
	c.restoreLoopExits(exits)
	return taken
}

// addLoopExits adds the breaks and continues takeLoopExits took back to
// the loops.
func (c *Checker) addLoopExits(taken []loopJumps) {
	for i, t := range taken {
		l := c.loops[i]
		l.hasBreak, l.hasContinue = l.hasBreak || t.hasBreak, l.hasContinue || t.hasContinue
		l.flow.breaks = append(l.flow.breaks, t.breaks...)
		l.flow.continues = append(l.flow.continues, t.continues...)
	}
}

// loopJumps are breaks and continues of a loop that takeLoopExits took.
type loopJumps struct {
	hasBreak, hasContinue bool
	breaks, continues     []flowState
}

// restoreLoopExits forgets the breaks and continues of the loops that
// loopExits returned found after it.
func (c *Checker) restoreLoopExits(exits []loopExit) {
	for i, e := range exits {
		l := c.loops[i]
		l.hasBreak, l.hasContinue = e.hasBreak, e.hasContinue
		l.flow.breaks, l.flow.continues = l.flow.breaks[:e.breaks], l.flow.continues[:e.continues]
	}
}

// throwable reports whether t is a subtype of Throwable. Without a JDK,
// there is no Throwable, and any class is accepted.
func (c *Checker) throwable(t types.Type) bool {
	th := c.u.ext.class("java/lang/Throwable")
	if th == nil {
		return t.K == types.Class
	}
	return c.subtype(t, types.ClassType(th))
}

// throw checks throw e.
func (c *Checker) throw(x *syntax.Jump) {
	n := len(c.errs)
	t := c.value(x.X, noWant)
	if th := c.u.ext.class("java/lang/Throwable"); th != nil && c.cannotInferSince(n) && retryableCall(x.X) {
		// throw a() for fun <T> a(): T is a's T Throwable.
		c.errs = c.errs[:n]
		t = c.value(x.X, types.ClassType(th))
	}
	if t.K != types.Invalid && t.K != types.Nothing && (!c.throwable(t.NonNull()) || types.MayBeNull(t) && !t.P) {
		// A platform value may be thrown, but not a nullable one.
		c.errorf(x.X.Pos(), "type mismatch: inferred type is %s but Throwable was expected", t)
	}
}

// assertions are the standard library's preconditions and errors. They
// are inline-only functions, so kc knows their bodies. They work without
// the standard library too.
var assertions = map[string]types.Builtin{
	"require": types.Require, "check": types.Check, "error": types.ErrorCall, "TODO": types.Todo,
	"requireNotNull": types.RequireNotNull, "checkNotNull": types.CheckNotNull,
}

// assertCall checks a call of require, check, error, TODO, requireNotNull
// or checkNotNull. The call's Sym names the exception class it throws, in
// Facade. As requireNotNull(value: T?): T, an expected T expects a T? of
// the value: checkNotNull(state[key]) infers get's T.
func (c *Checker) assertCall(x *syntax.Call, b types.Builtin, name string, want types.Type) types.Type {
	x.Builtin = b
	c.noSpread(x)
	args, msg := syntax.AssertArgs(x)
	if len(x.TypeArgs) > 0 {
		c.errorf(x.TypeArgs[0].P, "no type arguments expected for '%s'", name)
	}
	exc := map[types.Builtin]string{
		types.Require: "java/lang/IllegalArgumentException", types.RequireNotNull: "java/lang/IllegalArgumentException",
		types.Check: "java/lang/IllegalStateException", types.CheckNotNull: "java/lang/IllegalStateException",
		types.ErrorCall: "java/lang/IllegalStateException", types.Todo: "java/lang/UnsupportedOperationException",
	}[b]
	if b == types.Todo && c.u.ext.kotlin {
		exc = "kotlin/NotImplementedError"
	}
	x.Sym = &types.FunSym{Name: name, Facade: exc, Resolved: true}
	wrong := func() types.Type {
		c.errorf(x.P, "no function '%s' takes %d arguments", name, len(x.Args))
		c.args(x, nil)
		return types.InvalidType
	}
	if msg != nil {
		if b == types.ErrorCall || b == types.Todo {
			return wrong()
		}
		// The lazy message: a () -> Any, called if the check fails. A
		// lambda is inlined, as with the standard library's inline-only
		// functions: it can suspend, ensurePresent's "missing from
		// context $coroutineContext".
		if l, ok := msg.(*syntax.Lambda); ok {
			l.Inline = true
			j := c.jumps[l]
			j.callee = name
			c.jumps[l] = j
		}
		ft := c.u.ext.funcs.Type(nil, types.AnyType)
		c.useArity(0)
		c.assignable(c.value(msg, ft), ft, msg.Pos())
	}
	switch b {
	case types.Require, types.Check:
		if len(args) != 1 {
			return wrong()
		}
		c.assignable(c.value(args[0], types.BoolType), types.BoolType, args[0].Pos())
		return types.UnitType
	case types.RequireNotNull, types.CheckNotNull:
		if len(args) != 1 {
			return wrong()
		}
		w := noWant
		if want.K != types.Invalid && want.K != types.Unit && !want.HasVars() {
			w = want.Nullable()
		}
		t := c.value(args[0], w)
		if t.K == types.Invalid {
			return t
		}
		return t.NonNull()
	case types.ErrorCall:
		if len(args) != 1 {
			return wrong()
		}
		c.value(args[0], noWant)
	case types.Todo:
		if len(args) > 1 {
			return wrong()
		}
		if len(args) == 1 {
			c.assignable(c.value(args[0], types.StringType), types.StringType, args[0].Pos())
		}
	}
	return types.NothingType
}
