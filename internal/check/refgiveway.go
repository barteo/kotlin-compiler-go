package check

import (
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// refsMissAll reports whether call x has a callable reference whose
// types kc can tell (see refShapes) and no callee of cands takes its
// callable references: as in K2, members that don't apply give way to
// extensions. Baz<String?>'s baz(KProperty1<T, *>) doesn't take
// String::length, Baz<T>.baz(KProperty1<T & Any, *>) does.
func (c *Checker) refsMissAll(cands []callee, x *syntax.Call) bool {
	if x.TypeArgs != nil || len(cands) == 0 {
		return false
	}
	shapes := map[int][]types.Type{}
	for i, a := range x.Args {
		if l, ok := a.(*syntax.Lambda); ok && l.Ref != nil {
			ts := c.refShapes(l)
			if len(ts) == 0 {
				ts = c.libPropShape(l.Ref)
			}
			if len(ts) > 0 {
				shapes[i] = ts
			}
		}
	}
	if len(shapes) == 0 {
		return false
	}
	for _, cl := range cands {
		amap, ok := mapArgs(cl.f, x)
		if !ok {
			continue
		}
		cl.amap, cl.names = amap, x.Names
		sys := newSystem(c)
		sub := sys.fresh(cl.tparams, cl.recv, x.P)
		fits := true
		for i, ts := range shapes {
			p := sub.ApplyIn(cl.argType(i, x.Args))
			var fit *system
			for _, ft := range ts {
				if s, ok := c.refFits(sys, ft, p); ok {
					fit = s
					break
				}
			}
			if fit == nil {
				fits = false
				break
			}
			sys = fit
		}
		if fits {
			return false
		}
	}
	return true
}

// libPropShape returns the type of reference C::name to a property of a
// class of the class path without type parameters, or String's length,
// which refShapes leaves out.
func (c *Checker) libPropShape(r *syntax.CallableRef) []types.Type {
	id, ok := r.X.(*syntax.Ident)
	if !ok || r.Type != nil || r.Bound != nil || c.isVar(id.Name) {
		return nil
	}
	cls := c.lookupClass(id.Name)
	if cls == nil && id.Name == "String" && r.Name == "length" {
		// Kotlin's String, a builtin type.
		recv := types.StringType
		if s, ok := c.kpropShape(false, &recv, types.IntType); ok {
			return []types.Type{s}
		}
	}
	if cls == nil || len(cls.TypeParams) > 0 || cls.Object {
		return nil
	}
	if len(c.javaMethods(cls, r.Name)) > 0 {
		return nil // a function too
	}
	recv := cls.This()
	var t types.Type
	mutable := false
	switch kp := cls.KProps[r.Name]; {
	case cls.External && kp != nil && kp.Getter != nil:
		t, mutable = kp.Getter.Ret, kp.Setter != nil
	default:
		return nil
	}
	if t.K == types.Invalid || t.HasParams() {
		return nil
	}
	if s, ok := c.kpropShape(mutable, &recv, t); ok {
		return []types.Type{s}
	}
	return nil
}
