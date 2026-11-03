package check

import (
	"github.com/barteo/kotlin-compiler-go/internal/inline"
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// inlineCall loads the body of the inline function a call resolves to, the
// first time a call does, and marks the lambda arguments that are inlined
// with it. It runs before the lambdas are checked. Code generation copies
// the body into the caller. An inline-only function whose body kc can't
// copy can't be called; a public one is called instead. It marks the
// lambdas the function calls exactly once too.
func (c *Checker) inlineCall(x *syntax.Call, cl *callee) {
	for k, a := range x.Args {
		if l, ok := a.(*syntax.Lambda); ok {
			i := cl.paramIndex(k)
			l.Once = i >= 0 && i < len(cl.f.Once) && cl.f.Once[i]
		}
	}
	c.forwardedArgs(x, cl)
	if cl.f.Builtin == types.SuspendIntrinsic {
		// Its lambda is generated in the caller, but can't suspend it.
		for _, a := range x.Args {
			if l, ok := a.(*syntax.Lambda); ok {
				l.Inline = true
				j := c.jumps[l]
				j.callee, j.crossinline, j.suspends = cl.f.Name, true, true
				l.Crossinline = true
				c.jumps[l] = j
			}
		}
		return
	}
	if cl.f.Builtin == types.ComposeKey {
		c.composeKey(x, cl)
		return
	}
	if d := syntax.SourceInline(cl.f); d != nil {
		c.sourceInlineCall(x, cl, d)
		return
	}
	// A composable takes the mask of the omitted arguments itself.
	b, err := inlineBody(cl.f, cl.omitted(len(x.Args)) > 0 && !cl.f.Composable)
	if err != nil {
		c.unsupported(x.P, "inline-only call", "can't inline '%s': %v", cl.f.Name, err)
		return
	}
	if b == nil {
		return
	}
	if c.ctorArgs && b.HasHandlers() {
		// Its handlers would start with an empty stack, without the
		// object under construction.
		c.unsupported(x.P, "try in constructor argument", "'%s' can't be an argument of a constructor call: its inlined body has exception handlers", cl.f.Name)
	}
	for k, a := range x.Args {
		if l, ok := a.(*syntax.Lambda); ok {
			c.markInline(l, cl.f, b, cl.paramIndex(k))
		}
	}
}

// markInline marks a lambda passed for f's parameter i inlined if f's body
// b only invokes the parameter, and records whether non-local jumps can
// leave it.
func (c *Checker) markInline(l *syntax.Lambda, f *types.FunSym, b *inline.Body, i int) {
	l.Inline = inlinable(f, b, i)
	if !l.Inline {
		return
	}
	j := c.jumps[l]
	j.callee, j.composable, j.suspends = f.Name, f.Composable, f.Suspend
	j.crossinline = j.crossinline || f.Inline.Crossinline[i]
	l.Crossinline = j.crossinline
	for _, t := range b.Params[jvmParam(f, b, i)].InTry {
		j.inTry = j.inTry || t
	}
	c.jumps[l] = j
}

// jvmParam returns the index of f's parameter i in its body b, where an
// extension's receiver comes first, after a member's dispatch receiver.
func jvmParam(f *types.FunSym, b *inline.Body, i int) int {
	if f.Ext {
		i++
	}
	if b.This {
		i++
	}
	return i
}

// inlineBody returns the body that a call of f copies: its own, or with
// omitted arguments its name$default method's. It returns nil if the call
// isn't inlined: f isn't inline, or it is a public inline function whose
// body kc can't copy, which is called instead.
func inlineBody(f *types.FunSym, omitted bool) (*inline.Body, error) {
	in := f.Inline
	if in == nil {
		return nil, nil
	}
	var b *inline.Body
	var err error
	if omitted {
		b, err = in.Defaults(types.DefaultBits(f))
	} else {
		b, err = in.Body()
	}
	if err != nil && !in.Private {
		return nil, nil
	}
	return b, err
}

// inlinable reports whether a lambda passed for f's parameter i is inlined
// with f's body b: the body only invokes the parameter, which isn't
// noinline.
func inlinable(f *types.FunSym, b *inline.Body, i int) bool {
	j := jvmParam(f, b, i)
	if j >= len(b.Params) || i >= len(f.Inline.Noinline) {
		return false
	}
	return b.Params[j].Inlinable && !f.Inline.Noinline[i]
}

// paramIndex returns the index of the parameter argument i is passed to.
func (cl *callee) paramIndex(i int) int {
	if cl.amap == nil {
		return i
	}
	return cl.amap[i]
}
