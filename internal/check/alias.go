package check

import (
	"slices"
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// declareAlias creates the symbol of a typealias, at the top level of the
// file's package or nested in outer.
func (c *Checker) declareAlias(d *syntax.TypeAliasDecl, outer *types.ClassSym) *types.AliasSym {
	a := &types.AliasSym{Name: d.Name, Pkg: c.file.Package, Outer: outer, Decl: d,
		Private: d.Visibility == syntax.Private, Internal: d.Visibility == syntax.Internal}
	for _, p := range d.TypeParams {
		if len(p.Bounds) > 0 {
			c.errorf(p.P, "bounds are not allowed on type alias parameters")
		}
		if p.Reified {
			c.errorf(p.P, "only type parameters of inline functions can be reified")
		}
	}
	a.TypeParams = c.declareTypeParams(d.TypeParams, a, false)
	d.Sym = a
	c.u.aliasFile[a] = c
	c.aliases = append(c.aliases, d)
	return a
}

// resolveAliases resolves the right sides of the file's typealiases, in
// the file's scope. One alias can stand for another, which is resolved
// first, in its own file's scope.
func (c *Checker) resolveAliases() {
	for _, d := range c.aliases {
		c.resolveAlias(d.Sym)
	}
}

// resolveAlias resolves a typealias's right side once. An alias that
// expands to itself is an error, as with kotlinc.
func (c *Checker) resolveAlias(a *types.AliasSym) {
	if a.Resolved {
		return
	}
	d := a.Decl.(*syntax.TypeAliasDecl)
	if a.Resolving {
		c.u.aliasFile[a].errorf(d.P, "recursive type alias in expansion: %s", a.Name)
		a.Type, a.Resolved = types.InvalidType, true
		return
	}
	if owner := c.u.aliasFile[a]; owner != c {
		owner.resolveAlias(a)
		return
	}
	a.Resolving = true
	savedParams, savedCls, savedChecks := c.tparams, c.lookupCls, c.noBoundChecks
	c.tparams, c.lookupCls, c.noBoundChecks = a.TypeParams, a.Outer, true
	t := c.resolveType(d.Type)
	c.tparams, c.lookupCls, c.noBoundChecks = savedParams, savedCls, savedChecks
	a.Resolving = false
	if !a.Resolved { // not already invalid as part of a cycle
		a.Type, a.Resolved = t, true
	}
}

// aliasClass returns the class a typealias expands to, if it expands to a
// class type: Alias(x) calls its constructor, Alias.x reads its companion.
func (c *Checker) aliasClass(a *types.AliasSym) *types.ClassSym {
	c.resolveAlias(a)
	return a.Class()
}

// lookupAlias finds one of the program's typealiases by its simple or
// qualified name: Alias, a.b.Alias or Outer.Alias.
func (c *Checker) lookupAlias(name string) *types.AliasSym {
	i := strings.LastIndexByte(name, '.')
	if i < 0 {
		return c.findClassifier(name).alias
	}
	prefix, last := name[:i], name[i+1:]
	if p := c.u.Pkgs[prefix]; p != nil {
		if a := p.Aliases[last]; a != nil {
			return a
		}
	}
	if cls := c.lookupClass(prefix); cls != nil {
		cls.Ensure()
		if a := cls.Aliases[last]; a != nil {
			return a
		}
	}
	if c.u.ext.kotlin {
		return c.u.ext.pkgAlias(prefix, last)
	}
	return nil
}

// expandAlias resolves a reference to a typealias: the type it stands
// for, with the reference's type arguments.
func (c *Checker) expandAlias(a *types.AliasSym, ref *syntax.TypeRef) types.Type {
	c.resolveAlias(a)
	if len(ref.Args) != len(a.TypeParams) {
		p := ref.P
		if ref.Args != nil {
			p = ref.Args[0].P
		}
		if len(a.TypeParams) == 0 {
			c.errorf(p, "no type arguments expected for typealias %s", a.Name)
		} else {
			c.errorf(p, "%s expected for typealias %s%s", countArgs(len(a.TypeParams)), a.Name, types.TypeParamsString(a.TypeParams))
		}
		return types.InvalidType
	}
	args := make([]types.Arg, len(ref.Args))
	for i, r := range ref.Args {
		args[i] = types.Arg{V: r.Variance}
		if r.Variance == types.Star {
			continue
		}
		if args[i].T = c.resolveType(r.Type); args[i].T.K == types.Invalid {
			return types.InvalidType
		}
	}
	t := a.Expand(args)
	if t.K == types.Invalid {
		return t
	}
	return c.nullable(t, ref)
}

// aliasConstruct checks Alias(args), a call of the constructor of the
// class that a typealias expands to. The expansion is the call's expected
// type when it is known, which infers the class's type arguments that the
// alias fixes: Names() is an ArrayList<String> for typealias Names =
// ArrayList<String>. Type arguments of the call are the alias's.
func (c *Checker) aliasConstruct(x *syntax.Call, a *types.AliasSym, cls *types.ClassSym, p syntax.Pos, want types.Type, sink *system) types.Type {
	c.checkAliasVisible(a, p)
	if hasUnderscore(x.TypeArgs) {
		if !c.aliasUnderscores(x, a, cls, p) {
			c.args(x, nil)
			return types.InvalidType
		}
		return c.construct(x, cls, p, want, sink)
	}
	if len(a.TypeParams) == 0 || len(x.TypeArgs) > 0 {
		t := c.expandAlias(a, &syntax.TypeRef{P: p, Name: a.Name, Args: x.TypeArgs})
		if t.K == types.Invalid {
			c.args(x, nil)
			return t
		}
		x.TypeArgs = nil
		want = t
		if !t.HasVars() {
			// Complete, as an argument too: atomic(Core<E>(capacity)).
			sink = nil
		}
	}
	return c.construct(x, cls, p, want, sink)
}

// hasUnderscore reports whether type arguments of a call include _, one
// the call infers.
func hasUnderscore(args []*syntax.TypeArg) bool {
	return slices.ContainsFunc(args, func(a *syntax.TypeArg) bool { return a.Underscore })
}

// aliasUnderscores rewrites the type arguments of Alias<A, _>(x), a call
// of the constructor of class cls that typealias a expands to, as the
// class's own: those the alias's _ are in are _ too, and the call infers
// them, as in Alias<_>("OK") for typealias Alias<T> = Box<T>. It reports
// false, after an error, if the arguments don't fit the alias.
func (c *Checker) aliasUnderscores(x *syntax.Call, a *types.AliasSym, cls *types.ClassSym, p syntax.Pos) bool {
	// Each _ is a placeholder, which the expansion shows the uses of.
	holes := map[*types.TypeParamSym]bool{}
	args := make([]*syntax.TypeArg, len(x.TypeArgs))
	for i, ta := range x.TypeArgs {
		args[i] = ta
		if ta.Underscore {
			h := &types.TypeParamSym{Name: "_"}
			holes[h] = true
			t := types.ParamType(h)
			args[i] = &syntax.TypeArg{P: ta.P, Type: &syntax.TypeRef{P: ta.P, Known: &t}}
		}
	}
	t := c.expandAlias(a, &syntax.TypeRef{P: p, Name: a.Name, Args: args})
	if t.K == types.Invalid {
		return false
	}
	if t.K != types.Class || t.C != cls {
		c.errorf(x.TypeArgs[0].P, "type arguments with _ are not supported for typealias %s", a.Name)
		return false
	}
	pos := x.TypeArgs[0].P
	own := len(cls.TypeParams) - cls.OuterParams // an inner class's outer ones aren't the call's
	out := make([]*syntax.TypeArg, min(own, t.A.Len()))
	for i := range out {
		arg := t.A.Get(i)
		if arg.V == types.Star || arg.T.Has(func(u types.Type) bool { return u.IsParam() && holes[u.Param()] }) {
			out[i] = &syntax.TypeArg{P: pos, Underscore: true}
			continue
		}
		at := arg.T
		out[i] = &syntax.TypeArg{P: pos, Type: &syntax.TypeRef{P: pos, Known: &at}}
	}
	x.TypeArgs = out
	return true
}
