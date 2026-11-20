package check

import (
	"slices"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// paramNameWarnings reports PARAMETER_NAME_CHANGED_ON_OVERRIDE, as
// kotlinc's FirOverrideChecker: a parameter of override fd that the
// function it directly overrides names otherwise, for each such function
// whose parameter names are stable, a Kotlin function's. A Java method's
// aren't: through a Java override in between, nothing is reported.
func (c *Checker) paramNameWarnings(fd *syntax.FunDecl, over []*types.FunSym) {
	for _, o := range over {
		if !stableParamNames(o) || len(o.Params) != len(fd.Params) || overriddenByAnother(o, over) {
			continue
		}
		for i, p := range fd.Params {
			if q := o.Params[i].Name; q != "" && p.Name != q {
				c.warnf(p.P, "PARAMETER_NAME_CHANGED_ON_OVERRIDE", "The corresponding parameter in the supertype '%s' is named '%s'. This may cause problems when calling this function with named arguments.", o.Owner.DisplayName(), q)
			}
		}
	}
}

// stableParamNames reports whether function f's parameter names are
// part of its API: it is the program's Kotlin function, or a Kotlin
// library's, read from its metadata.
func stableParamNames(f *types.FunSym) bool {
	if f.Owner == nil || f.Owner.Func {
		return false // a function type's invoke
	}
	if _, ok := f.Decl.(*syntax.FunDecl); ok {
		return true
	}
	return f.Owner.External && f.Owner.FromKotlin
}

// overriddenByAnother reports whether another of the functions over,
// one of a subclass of o's class, overrides o: then o isn't overridden
// directly.
func overriddenByAnother(o *types.FunSym, over []*types.FunSym) bool {
	for _, g := range over {
		if g != o && g.Owner != o.Owner && g.Owner.IsSubclassOf(o.Owner) {
			return true
		}
	}
	return false
}

// supertypeParamNameWarnings reports DIFFERENT_NAMES_FOR_THE_SAME_PARAMETER_IN_SUPERTYPES
// for class d, as kotlinc: the functions gr it inherits as one member
// from several supertypes and doesn't override name one of their
// parameters differently. A Java method among them has no stable names,
// and then nothing is reported.
func (c *Checker) supertypeParamNameWarnings(d *syntax.ClassDecl, gr []*types.FunSym) {
	if slices.ContainsFunc(gr, func(g *types.FunSym) bool { return !stableParamNames(g) || len(g.Params) != len(gr[0].Params) }) {
		return
	}
	for i := range gr[0].Params {
		for _, g := range gr[1:] {
			if a, b := gr[0].Params[i].Name, g.Params[i].Name; a != b {
				c.warnf(d.P, "DIFFERENT_NAMES_FOR_THE_SAME_PARAMETER_IN_SUPERTYPES", "Names '%s' and '%s' of parameter #%d conflict in the following members of supertypes: '%s' defined in '%s', '%s' defined in '%s'. This may cause problems when calling this function with named arguments.",
					a, b, i, gr[0].Name, gr[0].Owner.DisplayName(), g.Name, g.Owner.DisplayName())
				return
			}
		}
	}
}

// ambiguousNamedArgs reports the named arguments of call x of member
// x.Sym on a receiver of class cls and type rt for a parameter that the
// functions it is inherited from as one member name differently
// (NAME_FOR_AMBIGUOUS_PARAMETER), as kotlinc: interface C : A, B, with
// A's foo(x: Int) and B's foo(y: Int), takes neither c.foo(x = 0) nor
// c.foo(y = 0). An override in between names them: nothing is reported.
// As the warning, nothing is reported of a Java method's names.
func (c *Checker) ambiguousNamedArgs(x *syntax.Call, cls *types.ClassSym, rt types.Type) {
	f := x.Sym
	if f == nil || f.Owner == nil || f.Static || f.Owner == cls || !stableParamNames(f) {
		return
	}
	var gr []*types.FunSym
	own := false
	cls.Supertypes(func(s *types.ClassSym) bool {
		for _, g := range s.Methods[f.Name] {
			if g.Static || g.Private || g.Ext != f.Ext || !c.sameParams(rt, f, g) {
				continue
			}
			if s == cls {
				own = true
			} else if !slices.Contains(gr, g) {
				gr = append(gr, g)
			}
		}
		return !own
	})
	if own {
		return
	}
	gr = slices.DeleteFunc(gr, func(g *types.FunSym) bool { return overriddenByAnother(g, gr) })
	if len(gr) < 2 || slices.ContainsFunc(gr, func(g *types.FunSym) bool { return !stableParamNames(g) || len(g.Params) != len(f.Params) }) {
		return
	}
	for i, a := range x.Args {
		name := x.Name(i)
		if name == "" {
			continue
		}
		j := slices.IndexFunc(f.Params, func(p *types.VarSym) bool { return p.Name == name })
		if j < 0 {
			continue
		}
		if slices.ContainsFunc(gr, func(g *types.FunSym) bool { return g.Params[j].Name != name }) {
			c.errorf(a.Pos(), "named argument is prohibited for parameter with an ambiguous name")
		}
	}
}
