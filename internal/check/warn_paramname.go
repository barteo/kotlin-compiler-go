package check

import (
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
