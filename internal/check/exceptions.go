package check

import (
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// tryExpr checks try { } catch (e: E) { } finally { }. Used as a value,
// its type is the join of the try block's and the catch blocks'; the
// finally block's value is discarded.
func (c *Checker) tryExpr(x *syntax.Try, want types.Type, valueUsed bool) types.Type {
	if valueUsed && c.ctorArgs {
		// Its handlers would start with an empty stack, without the
		// object under construction.
		c.unsupported(x.P, "try in constructor argument", "a try expression can't be an argument of a constructor call")
	}
	// The catch and finally blocks don't know how far the body got: its
	// assignments drop the vars' narrowing there, and after the try.
	bodyVars := c.watchedVars(x.Body)
	defer c.reset(c.watchedVars(x))
	exits := c.loopExits()
	result := c.block(x.Body, want, valueUsed)
	allJump := result.K == types.Nothing
	for _, cb := range x.Catches {
		c.localAnnotations(cb.Annotations, catchSite)
		t := c.resolveType(cb.Type)
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
		c.scope.declare(cb.Sym)
		bt := c.block(cb.Body, want, valueUsed)
		c.pop()
		allJump = allJump && bt.K == types.Nothing
		if valueUsed {
			j, ok := c.joinBranch(result, bt, want, cb.Body.P)
			if !ok {
				c.errorf(cb.Body.P, "type mismatch: 'try' and 'catch' blocks have incompatible types %s and %s", result, bt)
				return types.InvalidType
			}
			result = j
		}
	}
	if x.Finally != nil {
		c.push()
		c.reset(bodyVars)
		ft := c.block(x.Finally, noWant, false)
		c.pop()
		if ft.K == types.Nothing {
			// A break or continue in the try or its catch blocks runs the
			// finally block, which jumps instead: it doesn't leave the
			// loop.
			c.restoreLoopExits(exits)
			return types.NothingType
		}
	}
	if !valueUsed {
		if allJump {
			return types.NothingType
		}
		return types.UnitType
	}
	return result
}

// loopExits returns whether each of the loops the code is in has a break
// and a continue so far.
func (c *Checker) loopExits() [][2]bool {
	exits := make([][2]bool, len(c.loops))
	for i, l := range c.loops {
		exits[i] = [2]bool{l.hasBreak, l.hasContinue}
	}
	return exits
}

// restoreLoopExits forgets the breaks and continues of the loops that
// loopExits returned found after it.
func (c *Checker) restoreLoopExits(exits [][2]bool) {
	for i, e := range exits {
		c.loops[i].hasBreak, c.loops[i].hasContinue = e[0], e[1]
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
	t := c.value(x.X, noWant)
	if t.K != types.Invalid && t.K != types.Nothing && !c.throwable(t.NonNull()) {
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
