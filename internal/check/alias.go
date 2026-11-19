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
		// Each alias of the cycle is an error: A and B of typealias A =
		// B and typealias B = A.
		cycle := c.u.aliasStack
		if i := slices.Index(cycle, a); i >= 0 {
			cycle = cycle[i:]
		} else {
			cycle = []*types.AliasSym{a}
		}
		for _, x := range cycle {
			if x.Resolved {
				continue
			}
			c.u.aliasFile[x].errorf(x.Decl.(*syntax.TypeAliasDecl).P, "recursive type alias in expansion: %s", x.Name)
			x.Type, x.Resolved = types.InvalidType, true
		}
		return
	}
	if owner := c.u.aliasFile[a]; owner != c {
		owner.resolveAlias(a)
		return
	}
	a.Resolving = true
	c.u.aliasStack = append(c.u.aliasStack, a)
	savedParams, savedCls, savedChecks := c.tparams, c.lookupCls, c.noBoundChecks
	c.tparams, c.lookupCls, c.noBoundChecks = a.TypeParams, a.Outer, true
	t := c.resolveType(d.Type)
	c.tparams, c.lookupCls, c.noBoundChecks = savedParams, savedCls, savedChecks
	c.u.aliasStack = c.u.aliasStack[:len(c.u.aliasStack)-1]
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
	if c.lookupAlias(prefix) != nil {
		return nil // C.Self.Self, through typealias Self = C
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
	if ref.Args == nil && ref == c.bareRef && bareAlias(a) {
		return c.bareType(a.Type.C, ref, c.bareSubject)
	}
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
	for i, arg := range args {
		if (arg.V == types.In || arg.V == types.Out) && projectionConflicts(a.Type, a.TypeParams[i], arg.V) {
			c.errorf(ref.Args[i].P, "conflicting projection in typealias expansion: typealias %s uses %s where '%s' conflicts with the variance", a.Name, a.TypeParams[i].Name, arg.V)
			return types.InvalidType
		}
	}
	for i, arg := range args {
		if (arg.V == types.In || arg.V == types.Out) && projectionRedundant(a.Type, a.TypeParams[i], arg.V) {
			c.redundantProjection(ref.Args[i].P)
		}
	}
	t := a.Expand(args)
	if t.K == types.Invalid {
		return t
	}
	if !c.noBoundChecks {
		known := make([]bool, len(args))
		for i := range known {
			known[i] = args[i].V != types.Star && !args[i].T.HasVars()
		}
		c.expansionBounds(a, args, known, ref, nil, nil)
	}
	return c.nullable(t, ref)
}

// bareAlias reports whether typealias a may be written without its type
// arguments in an is or as, as its class may: it expands to a class whose
// type arguments are its own type parameters, each once, as in
// typealias L<T> = List<T>.
func bareAlias(a *types.AliasSym) bool {
	t := a.Type
	if len(a.TypeParams) == 0 || t.K != types.Class || t.N || t.A == nil || len(t.A.List) != len(a.TypeParams) {
		return false
	}
	seen := map[*types.TypeParamSym]bool{}
	for _, arg := range t.A.List {
		if arg.V != types.Inv || !arg.T.IsParam() || arg.T.N || seen[arg.T.Param()] {
			return false
		}
		seen[arg.T.Param()] = true
	}
	return true
}

// projectionConflicts reports whether type parameter p of a typealias,
// given the projection v, is an argument of a class in t whose variance
// is the opposite: TA<in Any> for TA<K> = Generic<K> and Generic<out T>.
func projectionConflicts(t types.Type, p *types.TypeParamSym, v types.Variance) bool {
	if t.K != types.Class || t.A == nil {
		return false
	}
	for i, a := range t.A.List {
		if a.V == types.Star {
			continue
		}
		if a.T.IsParam() && a.T.Param() == p {
			w := a.V
			if w == types.Inv && i < len(t.C.TypeParams) {
				w = t.C.TypeParams[i].Var
			}
			if w != types.Inv && w != v {
				return true
			}
			// In<out T>, which kotlinc reports, and then In3<in Int>
			// of typealias In3<T> = In<out T>: the class's own variance
			// conflicts too.
			if i < len(t.C.TypeParams) {
				if d := t.C.TypeParams[i].Var; d != types.Inv && d != v {
					return true
				}
			}
			continue
		}
		if projectionConflicts(a.T, p, v) {
			return true
		}
	}
	return false
}

// projectionRedundant reports whether the expansion t of a typealias
// passes its parameter p, given the projection v, to a class's type
// parameter declared with variance v: List<out String> for TL<out String>
// with typealias TL<T> = List<T>.
func projectionRedundant(t types.Type, p *types.TypeParamSym, v types.Variance) bool {
	if t.K != types.Class || t.A == nil {
		return false
	}
	for i, a := range t.A.List {
		if a.V == types.Star {
			continue
		}
		if a.T.IsParam() && a.T.Param() == p {
			if i < len(t.C.TypeParams) && t.C.TypeParams[i].Var == v && (a.V == types.Inv || a.V == v) {
				return true
			}
			continue
		}
		if projectionRedundant(a.T, p, v) {
			return true
		}
	}
	return false
}

// redundantProjection warns of a use-site projection that repeats the
// declared variance of its type parameter, as kotlinc does.
func (c *Checker) redundantProjection(p syntax.Pos) {
	c.warnf(p, "REDUNDANT_PROJECTION", "projection is redundant: the corresponding type parameter has the same variance")
}

// aliasConstruct checks Alias(args), a call of the constructor of the
// class that a typealias expands to. The expansion is the call's expected
// type when it is known, which infers the class's type arguments that the
// alias fixes: Names() is an ArrayList<String> for typealias Names =
// ArrayList<String>. Type arguments of the call are the alias's.
func (c *Checker) aliasConstruct(x *syntax.Call, a *types.AliasSym, cls *types.ClassSym, p syntax.Pos, want types.Type, sink *system) types.Type {
	c.checkAliasVisible(a, p)
	if c.aliasCtors == nil {
		c.aliasCtors = map[*syntax.Call]*types.AliasSym{}
	}
	c.aliasCtors[x] = a
	if args, ok := c.aliasTypeArgs[x]; ok {
		// Checked again: the class's type arguments, as rewritten.
		x.TypeArgs = args
		return c.construct(x, cls, p, want, sink)
	}
	if c.inferAliasArgs[x] || len(x.TypeArgs) == 0 && len(a.TypeParams) > 0 && !bareAlias(a) && a.Type.K == types.Class && a.Type.C == cls {
		// Alias(x) infers the alias's type arguments, as Alias<_>(x),
		// and the class's are those the alias gives: Int for A of
		// TA { "s" } for typealias TA<K> = Foo<Int, K>, also when the
		// call is checked again.
		if c.inferAliasArgs == nil {
			c.inferAliasArgs = map[*syntax.Call]bool{}
		}
		c.inferAliasArgs[x] = true
		x.TypeArgs = nil
		for range a.TypeParams {
			x.TypeArgs = append(x.TypeArgs, &syntax.TypeArg{P: p, Underscore: true})
		}
	}
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
		if t.K == types.Class && t.C == cls && t.A != nil && slices.ContainsFunc(t.A.List, func(a types.Arg) bool { return a.V != types.Inv }) {
			// CIn() of typealias CIn = C<in Int>.
			c.errorf(p, "type alias with projections in its expansion can't be used as a constructor or a supertype: %s", t)
			c.args(x, nil)
			return types.InvalidType
		}
		if len(x.TypeArgs) > 0 && t.K == types.Class && t.C == cls && !t.HasVars() {
			// The explicit type arguments are the class's, not just
			// the expected type: PR<Int, String>(1, "") is an error for
			// typealias PR<T1, T2> = Pair<T2, T1>.
			pos := x.TypeArgs[0].P
			own := len(cls.TypeParams) - cls.OuterParams
			out := make([]*syntax.TypeArg, min(own, t.A.Len()))
			for i := range out {
				at := t.A.Get(i).T
				out[i] = &syntax.TypeArg{P: pos, Type: &syntax.TypeRef{P: pos, Known: &at}}
			}
			if c.aliasTypeArgs == nil {
				c.aliasTypeArgs = map[*syntax.Call][]*syntax.TypeArg{}
			}
			c.aliasTypeArgs[x] = out
			x.TypeArgs = out
			return c.construct(x, cls, p, want, sink)
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
	// The placeholders have no bounds to check.
	saved := c.noBoundChecks
	c.noBoundChecks = true
	t := c.expandAlias(a, &syntax.TypeRef{P: p, Name: a.Name, Args: args})
	c.noBoundChecks = saved
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
			if dep, ok := holeDep(arg.T, t, cls, holes); ok && arg.V == types.Inv && !(arg.T.IsParam() && holes[arg.T.Param()]) {
				out[i].Type = &syntax.TypeRef{P: pos, Known: &dep} // see relateUnderscores
			}
			continue
		}
		at := arg.T
		out[i] = &syntax.TypeArg{P: pos, Type: &syntax.TypeRef{P: pos, Known: &at}}
	}
	x.TypeArgs = out
	return true
}

// checkAliasBounds checks the type arguments of the file's typealiases'
// right sides against their classes' bounds, once those are resolved:
// TC<T, Any> for class TC<T, C : Collection<T>>, as kotlinc's
// UPPER_BOUND_VIOLATED, and those of the typealiases they name, as
// UPPER_BOUND_VIOLATED_IN_TYPEALIAS_EXPANSION. An argument that mentions
// the typealias's own parameters, which have no bounds, isn't checked,
// as TC<T, C> isn't: its uses are.
func (c *Checker) checkAliasBounds() {
	savedParams, savedCls := c.tparams, c.lookupCls
	for _, d := range c.aliases {
		a := d.Sym
		if a == nil || !a.Resolved || a.Type.K == types.Invalid || d.Type == nil {
			continue
		}
		if a.Type.K == types.Class && compilerRequiredAnnotations[a.Type.C.Name] {
			c.errorf(d.Type.P, "typealias to a compiler-required annotation '%s' is not allowed. Remove the typealias and use the annotation directly", strings.ReplaceAll(a.Type.C.Name, "/", "."))
		}
		own := func(t types.Type) bool {
			return t.Has(func(u types.Type) bool { return u.IsParam() && u.Param().Owner == a })
		}
		// A bound that mentions them is met if some arguments meet it:
		// List<String> is within List<T>, but Any isn't within
		// Collection<T>.
		stars := types.Subst{}
		for _, p := range a.TypeParams {
			stars[p] = types.Arg{V: types.Star}
		}
		c.tparams, c.lookupCls = a.TypeParams, a.Outer
		if r := d.Type; a.Type.IsParam() && len(r.Args) == 0 && r.Ret == nil && r.Outer == nil && r.Known == nil && c.lookupTypeParam(r.Name) != nil {
			c.errorf(r.P, "typealias should expand to a class, an interface or an object, not a type parameter")
		}
		c.refBounds(a.Type, d.Type, own, stars)
	}
	c.tparams, c.lookupCls = savedParams, savedCls
	c.abbreviatedNothing()
}

// compilerRequiredAnnotations are the annotations kotlinc reads before it
// resolves types, which a typealias can't stand for
// (TYPEALIAS_EXPANDS_TO_COMPILER_REQUIRED_ANNOTATION_ERROR), by internal
// name: its requiredAnnotations for the JVM.
var compilerRequiredAnnotations = map[string]bool{
	"kotlin/Deprecated":            true,
	"kotlin/annotation/Target":     true,
	"kotlin/DeprecatedSinceKotlin": true,
	"kotlin/SinceKotlin":           true,
	"kotlin/WasExperimental":       true,
	"kotlin/IntroducedAt":          true,
	"java/lang/annotation/Target":  true,
	"java/lang/Deprecated":         true,
	"kotlin/jvm/JvmName":           true,
	"kotlin/jvm/JvmRecord":         true,
	"kotlin/jvm/JvmInline":         true,
}

// refBounds checks the type arguments of type t, written as ref, and of
// the types among them, against their bounds, but those skip reports,
// with erase applied to the bounds.
func (c *Checker) refBounds(t types.Type, ref *syntax.TypeRef, skip func(types.Type) bool, erase types.Subst) {
	if ref == nil || t.K != types.Class || t.A == nil || ref.Ret != nil || ref.Known != nil || ref.Outer != nil {
		return
	}
	if a := c.lookupAlias(ref.Name); a != nil {
		c.resolveAlias(a)
		if len(ref.Args) != len(a.TypeParams) || a.Type.K == types.Invalid {
			return
		}
		args, known := aliasArgs(a, t)
		c.expansionBounds(a, args, known, ref, skip, erase)
		for i, r := range ref.Args {
			if known[i] && r.Type != nil && args[i].V != types.Star {
				c.refBounds(args[i].T, r.Type, skip, erase)
			}
		}
		return
	}
	if len(ref.Args) != len(t.A.List) {
		return
	}
	c.argBounds(t, func(i int) syntax.Pos { return ref.Args[i].P }, skip, erase, nil)
	for i, r := range ref.Args {
		if a := t.A.Get(i); r.Type != nil && a.V != types.Star {
			c.refBounds(a.T, r.Type, skip, erase)
		}
	}
}

// aliasArgs returns the type arguments of a use of typealias a that
// expands to t, and which of them t shows: those of the parameters its
// right side uses.
func aliasArgs(a *types.AliasSym, t types.Type) ([]types.Arg, []bool) {
	args := make([]types.Arg, len(a.TypeParams))
	known := make([]bool, len(a.TypeParams))
	var match func(p, u types.Type)
	match = func(p, u types.Type) {
		if p.IsParam() && p.Param().Owner == a {
			if i := p.Param().Index; i < len(args) && !known[i] {
				if p.N {
					u = u.NonNull()
				}
				args[i], known[i] = types.Arg{T: u}, true
			}
			return
		}
		if p.K != types.Class || u.K != types.Class || p.A == nil || u.A == nil || p.C != u.C || p.A.Len() != u.A.Len() {
			return
		}
		for i, x := range p.A.List {
			if y := u.A.Get(i); x.V != types.Star && y.V != types.Star {
				match(x.T, y.T)
			}
		}
	}
	match(a.Type, t)
	return args, known
}

// expansionBounds checks the type arguments of the generic types of
// typealias a's right side, with the arguments args of its use ref that
// are known, against their bounds. An error in an argument that is one
// of a's parameters is reported on ref's argument for it, others on ref.
func (c *Checker) expansionBounds(a *types.AliasSym, args []types.Arg, known []bool, ref *syntax.TypeRef, skip func(types.Type) bool, erase types.Subst) {
	s := types.Subst{}
	// The bounds that mention the unknown arguments are erased as those
	// of the typealias's right side are (see checkAliasBounds).
	stars := types.Subst{}
	for p, x := range erase {
		stars[p] = x
	}
	for i, p := range a.TypeParams {
		if known[i] {
			s[p] = args[i]
		} else {
			stars[p] = types.Arg{V: types.Star}
		}
	}
	erase = stars
	unknown := func(t types.Type) bool {
		return skip != nil && skip(t) || t.Has(func(u types.Type) bool { return u.K == types.TypeVar || u.IsParam() && u.Param().Owner == a })
	}
	var walk func(t types.Type)
	walk = func(t types.Type) {
		if t.K != types.Class || t.A == nil {
			return
		}
		at := func(i int) syntax.Pos {
			if x := t.A.Get(i); x.V != types.Star && x.T.IsParam() && x.T.Param().Owner == a && x.T.Param().Index < len(ref.Args) {
				return ref.Args[x.T.Param().Index].P
			}
			return ref.P
		}
		c.argBounds(s.ApplyExact(t), at, unknown, erase, a)
		for _, x := range t.A.List {
			if x.V != types.Star {
				walk(x.T)
			}
		}
	}
	walk(a.Type)
}

// argBounds checks the type arguments of class type t, but those skip
// reports, against the bounds of its class's type parameters, and
// reports an error at(i) for argument i out of them; erase, if set, is
// applied to the bounds, and alias, if set, is the typealias whose
// expansion t is part of.
func (c *Checker) argBounds(t types.Type, at func(int) syntax.Pos, skip func(types.Type) bool, erase types.Subst, alias *types.AliasSym) {
	ps := t.C.TypeParams
	if t.K != types.Class || t.A == nil || len(ps) != len(t.A.List) {
		return
	}
	s := classSubst(t)
	for i, a := range t.A.List {
		if a.V == types.Star || a.T.K == types.Invalid || skip != nil && skip(a.T) {
			continue
		}
		for _, b := range ps[i].Bounds {
			if b = s.Apply(b); erase != nil {
				b = erase.Apply(b)
			}
			if !c.subtype(a.T, b) {
				if alias != nil {
					c.errorf(at(i), "type argument is not within its bounds in the expansion of typealias %s: type parameter '%s' must be subtype of '%s', but actual: '%s'", alias.Name, ps[i].Name, b, a.T)
				} else {
					c.errorf(at(i), "type argument is not within its bounds: type parameter '%s' must be subtype of '%s', but actual: '%s'", ps[i].Name, b, a.T)
				}
				break
			}
		}
	}
}

// abbreviatedNothing reports the file's functions and properties whose
// declared type is a typealias for Nothing, which kotlinc wants written
// as Nothing (ABBREVIATED_NOTHING_RETURN_TYPE and _PROPERTY_TYPE).
func (c *Checker) abbreviatedNothing() {
	saved := c.lookupCls
	var walk func(decls []syntax.Decl, cls *types.ClassSym)
	walk = func(decls []syntax.Decl, cls *types.ClassSym) {
		for _, d := range decls {
			c.lookupCls = cls
			switch d := d.(type) {
			case *syntax.FunDecl:
				if c.nothingAlias(d.Ret) {
					c.errorf(d.P, "'Nothing' return type needs to be specified explicitly")
				}
			case *syntax.PropDecl:
				if c.nothingAlias(d.Type) {
					c.errorf(d.P, "'Nothing' property type needs to be specified explicitly")
				}
			case *syntax.ClassDecl:
				walk(d.Members, d.Sym)
			}
		}
	}
	walk(c.file.Decls, nil)
	c.lookupCls = saved
}

// nothingAlias reports whether ref names a typealias for Nothing.
func (c *Checker) nothingAlias(ref *syntax.TypeRef) bool {
	if ref == nil || ref.Name == "" || ref.Nullable || ref.Ret != nil || ref.Known != nil || len(ref.Args) > 0 {
		return false
	}
	a := c.lookupAlias(ref.Name)
	if a == nil {
		return false
	}
	c.resolveAlias(a)
	return a.Type.K == types.Nothing && !a.Type.N
}
