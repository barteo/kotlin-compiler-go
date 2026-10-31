package check

import (
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// Coroutines.
//
// A suspend function can only be called where the code can suspend: in a
// suspend function, or in a suspend lambda, the body of a lambda whose
// expected type is a suspend function type. A lambda inlined into its
// caller, which isn't crossinline, is part of the code around it.

// suspendScope returns the innermost lambda whose body is code of its
// own, or nil for the function being checked, whether that code can
// suspend, and whether an inlined lambda passed to synchronized is
// between the two: a suspension point there would hold the monitor.
func (c *Checker) suspendScope() (l *syntax.Lambda, ok, critical bool) {
	for i := len(c.lambdas) - 1; i >= c.base; i-- {
		l := c.lambdas[i]
		j := c.jumps[l]
		switch {
		case l.Suspend:
			return l, true, critical
		case l.Inline && !j.crossinline:
			critical = critical || j.callee == "synchronized"
			continue
		}
		return l, false, critical
	}
	return nil, c.fn != nil && c.fn.Suspend, critical
}

// suspendCall checks a call at p of f, if it is a suspend function.
func (c *Checker) suspendCall(p syntax.Pos, f *types.FunSym) {
	if f == nil || !f.Suspend {
		return
	}
	l, ok, critical := c.suspendScope()
	switch {
	case !ok:
		c.errorf(p, "suspend function '%s' can only be called from a coroutine or another suspend function", f.Name)
	case critical:
		c.errorf(p, "the '%s' suspension point is inside a critical section", f.Name)
	case l != nil && l.Recv != nil && !restrictedCallee(l.Recv.Type, f):
		c.errorf(p, "restricted suspending functions can invoke member or extension suspending functions only on their restricted coroutine scope")
	}
}

// restrictedCallee reports whether a suspend lambda with a receiver of
// type recv can call f: if the receiver's class is annotated
// @RestrictsSuspension, f must be a member or an extension of it.
func restrictedCallee(recv types.Type, f *types.FunSym) bool {
	cls := restrictedClass(recv)
	if cls == nil {
		return true
	}
	if f.Owner != nil && !f.Ext && cls.IsSubclassOf(f.Owner) {
		return true
	}
	if f.Ext && f.Recv.K == types.Class {
		if rc := restrictedClass(f.Recv); rc != nil && cls.IsSubclassOf(f.Recv.C) {
			return true
		}
	}
	return false
}

// restrictedClass returns the class of t if it or a supertype is
// annotated @RestrictsSuspension.
func restrictedClass(t types.Type) *types.ClassSym {
	t = t.Flex().NonNull()
	if t.K != types.Class {
		return nil
	}
	var found bool
	t.C.Supertypes(func(s *types.ClassSym) bool {
		found = s.RestrictsSuspension
		return !found
	})
	if !found {
		return nil
	}
	return t.C
}
