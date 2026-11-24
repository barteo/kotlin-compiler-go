package check

import (
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// This file reports kotlinc's warnings of a type the code didn't write
// that names a class it can't see, which compiles but may fail when the
// program runs, as a cast to the class does:
//
//   - INFERRED_INVISIBLE_REIFIED_TYPE_ARGUMENT_WARNING, of a reified type
//     argument a call infers (or one written _);
//   - INFERRED_INVISIBLE_VARARG_TYPE_ARGUMENT_WARNING, of the elements of
//     the array a call passes its vararg arguments in, of a type it infers;
//   - INFERRED_INVISIBLE_RETURN_TYPE_WARNING, of the type of a call,
//     otherwise;
//   - INFERRED_INVISIBLE_WHEN_TYPE_WARNING, of the type of an if or a when,
//     the join of its branches.
//
// The classes judged invisible are those whose visibility doesn't depend
// on where in the file the code is: a private top-level class of another
// file, or one nested in it, and a package-private Java class of another
// package, or one nested in it. A private or protected nested class
// isn't judged, nor a local or anonymous one.

// inferredInvisibleCall reports the call x's types that name a class the
// code can't see.
func (c *Checker) inferredInvisibleCall(x *syntax.Call) {
	if x.Ctor != nil || x.Invoke || x.Value != nil || x.Sym == nil || x.Type().K == types.Invalid {
		return
	}
	written := func(i int) bool {
		return i < len(x.TypeArgs) && len(x.TypeArgs) == len(x.Sym.TypeParams) && !x.TypeArgs[i].Underscore
	}
	inferred := len(x.TypeArgs) == 0
	for _, a := range x.TypeArgs {
		inferred = inferred || a.Underscore
	}
	pos := calleePos(x)
	if len(x.Reified) == len(x.Sym.TypeParams) {
		for i, tp := range x.Sym.TypeParams {
			if tp.Reified && !written(i) && c.invisibleIn(x.Reified[i]) != nil {
				c.warnf(pos, "INFERRED_INVISIBLE_REIFIED_TYPE_ARGUMENT_WARNING", "type argument for reified type parameter '%s' is inferred as invisible in this scope type '%s'", tp.Name, x.Reified[i])
				return
			}
		}
	}
	if x.Builtin == types.ArrayLiteral && !x.Literal && inferred {
		// arrayOf(a, b), emptyArray(): T, reified, is the elements'.
		if t := x.Type(); t.K == types.Class && t.C.Array && t.A != nil && len(t.A.List) == 1 {
			if e := t.A.List[0].T; c.invisibleIn(e) != nil {
				c.warnf(pos, "INFERRED_INVISIBLE_REIFIED_TYPE_ARGUMENT_WARNING", "type argument for reified type parameter 'T' is inferred as invisible in this scope type '%s'", e)
				return
			}
		}
	}
	if p := genericVararg(x.Sym); inferred && p != nil && x.Vararg.K == types.Class && x.Vararg.A != nil && len(x.Vararg.A.List) == 1 {
		if e := x.Vararg.A.List[0].T; c.invisibleIn(e) != nil {
			tp := "T"
			if pt := p.Type; pt.A != nil && len(pt.A.List) == 1 && pt.A.List[0].T.IsParam() {
				tp = pt.A.List[0].T.Param().Name
			}
			c.warnf(pos, "INFERRED_INVISIBLE_VARARG_TYPE_ARGUMENT_WARNING", "type argument for type parameter '%s' used as type of vararg parameter '%s' is inferred as invisible in this scope type '%s'", tp, p.Name, e)
			return
		}
	}
	// Not of f<A>(), whose type argument is written.
	if t := x.Type(); (inferred || len(x.Sym.TypeParams) == 0) && c.invisibleIn(t) != nil {
		c.warnf(syntax.StartPos(x), "INFERRED_INVISIBLE_RETURN_TYPE_WARNING", "inferred return type '%s' for '%s' is not visible in this scope", t, x.Sym.Name)
	}
}

// inferredInvisibleBranches reports an if or a when, x, whose branches'
// join names a class the code can't see: where no expected type gives it
// a type, a statement's too, if each branch gives a value.
func (c *Checker) inferredInvisibleBranches(x syntax.Expr) {
	if t, ok := c.branchJoins[x]; ok && c.invisibleIn(t) != nil {
		what := "when"
		if _, ok := x.(*syntax.If); ok {
			what = "if"
		}
		c.warnf(syntax.StartPos(x), "INFERRED_INVISIBLE_WHEN_TYPE_WARNING", "inferred type '%s' for %s expression is not visible in this scope", t, what)
	}
}

// genericVararg returns the vararg parameter of f, if it has one, only
// one, whose type names one of f's type parameters, or nil.
func genericVararg(f *types.FunSym) *types.VarSym {
	if len(f.TypeParams) == 0 {
		return nil
	}
	var v *types.VarSym
	for _, p := range f.Params {
		if p.Vararg {
			if v != nil {
				return nil // more than one is an error
			}
			v = p
		}
	}
	if v == nil || !v.Type.HasParams() {
		return nil
	}
	return v
}

// calleePos returns where the name of the function call x calls is.
func calleePos(x *syntax.Call) syntax.Pos {
	switch f := x.Fun.(type) {
	case *syntax.Member:
		return f.P
	case *syntax.Ident:
		return f.P
	}
	return syntax.StartPos(x)
}

// invisibleIn returns a class t names, as itself or a type argument at
// any depth, that the code can't see, as this file judges it, or nil.
func (c *Checker) invisibleIn(t types.Type) *types.ClassSym {
	return c.invisibleInDepth(t, 0)
}

func (c *Checker) invisibleInDepth(t types.Type, depth int) *types.ClassSym {
	if depth > 10 || t.K != types.Class || c.file == nil || c.file.SuppressInvisible {
		return nil
	}
	for k := t.C; k != nil; k = k.Outer {
		if c.surelyInvisible(k) {
			return t.C
		}
	}
	if t.A != nil {
		for _, a := range t.A.List {
			if a.V == types.Star {
				continue
			}
			if cls := c.invisibleInDepth(a.T, depth+1); cls != nil {
				return cls
			}
		}
	}
	return nil
}

// surelyInvisible reports whether class cls, not as nested in another,
// can't be seen in the file being checked wherever in it: a private
// top-level class of another file, or a package-private Java class of
// another package.
func (c *Checker) surelyInvisible(cls *types.ClassSym) bool {
	if cls.External {
		cls.Ensure()
		return !cls.Missing && !cls.FromKotlin && cls.PackagePrivate && !c.samePackage(cls)
	}
	if cls.Outer != nil || cls.Anonymous {
		return false
	}
	v, ok := declVisibility(cls.Decl)
	return ok && v == syntax.Private && cls.File != "" && cls.File != c.file.Class()
}
