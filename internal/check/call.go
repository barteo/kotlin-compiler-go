package check

import (
	"slices"
	"strconv"
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// callee is a function a call may resolve to. recv maps the type
// parameters of the class declaring it to the receiver's type arguments;
// tparams are the parameters the call infers or takes explicitly: the
// function's own, and the class's for a constructor.
type callee struct {
	f       *types.FunSym
	recv    types.Subst
	tparams []*types.TypeParamSym
	ext     types.Type // an extension function's receiver's type
	// dispatch is the object that is the dispatch receiver of an
	// extension declared in it, called where the object is in scope.
	dispatch *types.ClassSym
	// dispatchRecv is set on a member extension of an implicit
	// receiver's class, a lambda's: that receiver is its dispatch
	// receiver, as in with(builder) { x eq 1 }.
	dispatchRecv *implicitRecv
	amap         []int        // the parameter each argument is passed to; nil: the one at its position
	ctx          []contextArg // the context arguments, if the function has context parameters
	// names are the call's argument names, with amap: a named argument
	// of a vararg parameter is its array.
	names []string
}

// param returns the parameter argument i is passed to.
func (cl *callee) param(i int) *types.VarSym {
	if cl.amap == nil {
		return cl.f.Params[i]
	}
	return cl.f.Params[cl.amap[i]]
}

// argType returns the type argument i is checked against: the type of the
// parameter it is passed to, or for a vararg parameter, the elements' type,
// unless the argument is spread, *xs, or named, xs = arrayOf(1): both pass
// an array.
func (cl *callee) argType(i int, args []syntax.Expr) types.Type {
	p := cl.param(i)
	if p.Vararg && !isSpread(args[i]) && (i >= len(cl.names) || cl.names[i] == "") {
		return types.VarargElem(p.Type)
	}
	return p.Type
}

func isSpread(x syntax.Expr) bool {
	_, ok := x.(*syntax.Spread)
	return ok
}

// usesVararg reports whether the callee has a vararg parameter.
func (cl *callee) usesVararg() bool {
	for _, p := range cl.f.Params {
		if p.Vararg {
			return true
		}
	}
	return false
}

// omitted returns how many parameters a call with nargs arguments leaves
// to their default values.
func (cl *callee) omitted(nargs int) int {
	if cl.amap == nil {
		return 0
	}
	used := map[int]bool{}
	for _, j := range cl.amap {
		used[j] = true
	}
	n := 0
	for j, p := range cl.f.Params {
		if !used[j] && !p.Vararg {
			n++
		}
	}
	return n
}

// samArgs returns the number of a call's lambdas, callable references
// among them, that the callee takes by SAM conversion: to a parameter of a type that
// isn't a function type or a type parameter.
func (cl *callee) samArgs(args []syntax.Expr) int {
	n := 0
	for i, a := range args {
		if _, ok := a.(*syntax.Lambda); ok {
			if t := cl.argType(i, args).Flex(); !t.IsFunc() && !t.IsParam() && t.K != types.Invalid {
				n++
			}
		}
	}
	return n
}

// fewestSAM returns the candidates among cs that take the fewest of a
// call's lambdas by SAM conversion, if that leaves out any: as in Kotlin,
// when none is the most specific, those that need no SAM conversion are
// tried alone. ktor's ConcurrentMap.computeIfAbsent(key, block: () ->
// Value) is called rather than Java's Map.computeIfAbsent(K, Function).
func fewestSAM[C any](cs []C, args []syntax.Expr, cl func(C) *callee) ([]C, bool) {
	least := -1
	for _, c := range cs {
		if n := cl(c).samArgs(args); least < 0 || n < least {
			least = n
		}
	}
	var out []C
	for _, c := range cs {
		if cl(c).samArgs(args) == least {
			out = append(out, c)
		}
	}
	return out, len(out) < len(cs)
}

// preferred picks among n equally specific candidates, as Kotlin does:
// one that takes its arguments without varargs, then one that leaves
// fewer parameters to their defaults. It returns -1 if none is preferred.
func preferred(n int, args []syntax.Expr, cand func(i int) *callee) int {
	if n == 1 {
		return 0
	}
	nargs := len(args)
	best := -1
	for _, prefer := range []func(a, b *callee) bool{
		func(a, b *callee) bool { return !a.f.LowPriority && b.f.LowPriority },
		func(a, b *callee) bool { return !a.usesVararg() && b.usesVararg() },
		func(a, b *callee) bool { return a.omitted(nargs) < b.omitted(nargs) },
	} {
		for i := range n {
			wins := true
			for j := range n {
				if j != i && !prefer(cand(i), cand(j)) {
					wins = false
					break
				}
			}
			if wins {
				return i
			}
		}
	}
	return best
}

// mapArgs matches a call's arguments to f's parameters: in order, a vararg
// parameter taking all remaining ones, and a trailing lambda taking the
// last parameter. Parameters without an argument must have default values
// (or be the vararg). It returns nil for a one-to-one match, and false if
// the arguments don't fit.
func mapArgs(f *types.FunSym, x *syntax.Call) ([]int, bool) {
	if x.Names == nil && x.RefArgs != nil {
		if names := refNames(f, x.RefArgs); names != nil {
			named := *x
			named.Names = names
			return mapNamed(f, &named)
		}
	}
	if x.Names != nil {
		return mapNamed(f, x)
	}
	ps := f.Params
	n := len(x.Args)
	simple := n == len(ps)
	for _, p := range ps {
		simple = simple && !p.Vararg && p.Type.K != types.Invalid
	}
	for _, a := range x.Args {
		simple = simple && !isSpread(a)
	}
	if simple {
		return nil, true
	}
	amap := make([]int, n)
	limit, positional := len(ps), n
	if x.Trailing() && n > 0 && len(ps) > 0 {
		if ps[len(ps)-1].Type.K == types.Invalid {
			return nil, false
		}
		amap[n-1] = len(ps) - 1
		limit, positional = len(ps)-1, n-1
	}
	pi := 0
	for i := range positional {
		if pi >= limit || ps[pi].Type.K == types.Invalid || isSpread(x.Args[i]) && !ps[pi].Vararg {
			return nil, false
		}
		amap[i] = pi
		if !ps[pi].Vararg {
			pi++
		}
	}
	used := make([]bool, len(ps))
	for _, j := range amap {
		used[j] = true
	}
	for j, p := range ps {
		if !used[j] && !p.Vararg && !p.Default {
			return nil, false
		}
	}
	return amap, true
}

// refNames names the arguments of a callable reference's call to f, of
// types ts, as Kotlin adapts a reference: those after a vararg's take
// it too while they are of its type, and after that each goes to the
// parameter at its own position if that has a default value. So
// fun f(a: Int, vararg b: String, c: Int = 0) is an (Int, String, Int)
// -> Unit, f(p0, p1, c = p2), and (Int, Array<String>, Int) -> Unit,
// f(p0, *p1, c = p2). It returns nil if none is named.
func refNames(f *types.FunSym, ts []types.Type) []string {
	var names []string
	after, vararg := false, false
	var elem types.Type
	for i, t := range ts {
		if vararg && t != elem {
			after = true
		}
		if i >= len(f.Params) {
			continue
		}
		p := f.Params[i]
		if after && p.Default {
			if names == nil {
				names = make([]string, len(ts))
			}
			names[i] = p.Name
		}
		if p.Vararg {
			elem, vararg = t, true
		}
	}
	return names
}

// mapNamed is mapArgs for a call with named arguments: positional ones
// first, then named ones, which go to the parameters with their names. As
// in Kotlin, positional arguments can follow named ones that are in
// their own positions: f(a = 1, 2).
func mapNamed(f *types.FunSym, x *syntax.Call) ([]int, bool) {
	ps := f.Params
	amap := make([]int, len(x.Args))
	used := make([]bool, len(ps))
	pi, named := 0, false
	for i := range x.Args {
		name := x.Name(i)
		switch {
		case x.Trailing() && i == len(x.Args)-1:
			if len(ps) == 0 || used[len(ps)-1] {
				return nil, false
			}
			amap[i] = len(ps) - 1
		case name == "":
			if named || pi >= len(ps) {
				return nil, false // a positional argument after a named one
			}
			amap[i] = pi
			if !ps[pi].Vararg {
				pi++
			}
		default:
			j := paramIndex(ps, name)
			if j < 0 || used[j] {
				return nil, false
			}
			if j == pi && !named {
				pi++ // in its own position, a vararg's spread array among them
			} else {
				named = true
			}
			amap[i] = j
		}
		if ps[amap[i]].Type.K == types.Invalid || isSpread(x.Args[i]) && !ps[amap[i]].Vararg {
			return nil, false
		}
		used[amap[i]] = true
	}
	for j, p := range ps {
		if !used[j] && !p.Vararg && !p.Default {
			return nil, false
		}
	}
	return amap, true
}

func paramIndex(ps []*types.VarSym, name string) int {
	for j, p := range ps {
		if p.Name == name {
			return j
		}
	}
	return -1
}

// namedArgError reports why a call with named arguments fits none of the
// candidates, if a name is the reason.
func (c *Checker) namedArgError(x *syntax.Call, cands []callee) bool {
	for _, cl := range cands {
		if o := cl.f.Owner; o != nil && o.External && o.KotlinName == "" {
			c.errorf(x.P, "named arguments are not allowed for non-Kotlin functions")
			return true
		}
	}
	seen := map[string]bool{}
	for i := range x.Args {
		name := x.Name(i)
		if name == "" {
			continue
		}
		if seen[name] {
			c.errorf(x.Args[i].Pos(), "an argument is already passed for this parameter: %s", name)
			return true
		}
		seen[name] = true
		found := false
		for _, cl := range cands {
			found = found || paramIndex(cl.f.Params, name) >= 0
		}
		if !found {
			c.errorf(x.Args[i].Pos(), "cannot find a parameter with this name: %s", name)
			return true
		}
	}
	// A positional argument can follow named ones in their own positions.
	inPlace := func(i int) bool {
		for _, cl := range cands {
			if paramIndex(cl.f.Params, x.Name(i)) == i {
				return true
			}
		}
		return false
	}
	for i := range x.Args {
		if x.Name(i) != "" && !inPlace(i) {
			for j := i + 1; j < len(x.Args); j++ {
				if x.Name(j) == "" && !(x.Trailing() && j == len(x.Args)-1) {
					c.errorf(x.Args[j].Pos(), "mixing named and positioned arguments is not allowed")
					return true
				}
			}
			break
		}
	}
	return false
}

func methodCallees(fs []*types.FunSym, recv func(*types.FunSym) types.Subst) []callee {
	out := make([]callee, len(fs))
	for i, f := range fs {
		out[i] = callee{f: f, tparams: f.TypeParams}
		if recv != nil {
			out[i].recv = recv(f)
		}
	}
	return out
}

// candidate is a callee prepared for one call: its parameter types with
// type parameters replaced by fresh variables of its own system.
type candidate struct {
	callee
	sub    types.Subst
	params []types.Type
	sys    *system
	sams   map[int]*types.FunSym // the arguments converted to fun interfaces: see samValue
	susps  map[int]bool          // the arguments converted to suspend function types: see suspendConverted
}

// args checks call arguments against the parameter types of cl (nil: no
// expectation, after an error).
func (c *Checker) args(x *syntax.Call, cl *callee) []types.Type {
	ts := make([]types.Type, len(x.Args))
	for i := range x.Args {
		want := noWant
		var f *types.FunSym
		if cl != nil {
			want, f = cl.argType(i, x.Args), cl.f
		}
		ts[i] = c.argValue(&x.Args[i], want, f)
	}
	return ts
}

// captured returns the type final that a generic parameter param was
// inferred as, with the projections of arg in the positions where param
// has a variable as an invariant type argument: an Array<out X> argument
// for an Array<T> parameter is captured, so it is an Array<out X>.
func captured(arg, param, final types.Type) types.Type {
	a := arg.Flex().NonNull()
	if final.K != types.Class || final.A == nil || param.K != types.Class || param.A == nil || a.K != types.Class || a.C != final.C || a.A == nil ||
		a.A.Len() != final.A.Len() || param.A.Len() != final.A.Len() {
		return final
	}
	args := slices.Clone(final.A.List)
	changed := false
	for i, pa := range param.A.List {
		if pa.V == types.Inv && pa.T.K == types.TypeVar && a.A.List[i].V != types.Inv && args[i].V == types.Inv {
			args[i] = a.A.List[i]
			changed = true
		}
	}
	if !changed {
		return final
	}
	t := types.GenericType(final.C, args)
	if final.N {
		t = t.Nullable()
	}
	return t
}

// capturable reports whether the projected type arguments of ext, the
// receiver of a call of generic extension f, can be captured, and returns
// ext as f's receiver class. f's type parameters that out and star
// projections are for appear in no parameter's type, which a capture
// couldn't take a value of (ResolvedCall<out D>.updateD(d: D)), and a star
// projection's has no bound that the class's type parameter doesn't have
// (Optional<*>.getOrNull() needs T : Any, but the capture of KClass<*> is
// an Any). Those that in projections are for are parameters' types only,
// which take a value of the projection's type, and not the result's.
// Array<out T>.copyOf(), KClass<out T>.cast(value) and, for a
// MutableMap<in K, in V>, set(key: K, value: V) can.
func (c *Checker) capturable(f *types.FunSym, ext types.Type) (types.Type, bool) {
	r := f.Recv.NonNull()
	a := ext.Flex().NonNull()
	if r.K != types.Class || r.A == nil {
		return a, false
	}
	if a.K != types.Class || a.C != r.C {
		if sup, ok := c.asSuper(a, r.C); ok {
			a = sup // destination[k] = v for a destination: M, M : MutableMap<in K, in V>
		}
	}
	if a.K != types.Class || a.A == nil || a.A.Len() != r.A.Len() {
		return a, false
	}
	for i, ra := range r.A.List {
		pa := a.A.List[i]
		if pa.V == types.Inv || ra.T.K != types.TypeParam {
			continue
		}
		tp := ra.T.Param()
		if pa.V == types.Star && !c.starWithin(a.C, i, tp) {
			return a, false
		}
		is := func(u types.Type) bool { return u.K == types.TypeParam && u.Param() == tp }
		if pa.V == types.In && f.Ret.Has(is) {
			return a, false
		}
		for _, p := range f.Params {
			if pa.V == types.In && is(p.Type) {
				continue
			}
			if p.Type.Has(is) {
				return a, false
			}
		}
	}
	return a, true
}

// starWithin reports whether the capture of a star projection for type
// parameter i of class cls is within the bounds of tp: each of tp's bounds
// is a supertype of one of the class parameter's.
func (c *Checker) starWithin(cls *types.ClassSym, i int, tp *types.TypeParamSym) bool {
	if i >= len(cls.TypeParams) {
		return len(tp.Bounds) == 0
	}
	own := cls.TypeParams[i].Bounds
	for _, b := range tp.Bounds {
		if b.HasParams() {
			return false
		}
		ok := false
		for _, ob := range own {
			if !ob.HasParams() && c.subtype(ob, b) {
				ok = true
			}
		}
		if !ok {
			return false
		}
	}
	return true
}

func hasCall(args []syntax.Expr) bool {
	for _, a := range args {
		if _, ok := a.(*syntax.Call); ok {
			return true
		}
	}
	return false
}

func hasLambda(args []syntax.Expr) bool {
	for _, a := range args {
		if _, ok := a.(*syntax.Lambda); ok {
			return true
		}
	}
	return false
}

// typeArgs resolves a call's explicit type arguments.
func (c *Checker) typeArgs(x *syntax.Call) []types.Type {
	if x.TypeArgs == nil {
		return nil
	}
	ts := make([]types.Type, len(x.TypeArgs))
	for i, a := range x.TypeArgs {
		if a.Variance != types.Inv {
			c.errorf(a.P, "projections are not allowed on type arguments of functions and properties")
			ts[i] = types.InvalidType
			continue
		}
		ts[i] = c.resolveType(a.Type)
	}
	return ts
}

func countArgs(n int) string {
	switch n {
	case 0:
		return "no type arguments"
	case 1:
		return "one type argument"
	}
	return strconv.Itoa(n) + " type arguments"
}

// resolve picks the function a call resolves to among cands, infers its
// type arguments and checks the arguments. It returns the call's type.
//
// With a sink (the call is an argument of another call), a generic call
// is left incomplete: its variables move to the sink, and the type it
// returns may mention them.
func (c *Checker) resolve(x *syntax.Call, cands []callee, want types.Type, sink *system) types.Type {
	t := c.resolve1(x, cands, want, sink)
	switch f := x.Sym; {
	case f == nil:
	case f.Name != "<init>":
		if !c.funVisible(f) {
			c.invisibleFun(x.Fun.Pos(), f)
		}
	case f.Owner != nil && !f.Owner.External && ctorVisibility(f) == syntax.Private && !c.canSee(syntax.Private, f.Owner, ""):
		// A superclass's private constructor, which only the classes
		// nested in it may call: a sealed class's. construct leaves
		// the others out (see visibleCtors).
		c.errorf(x.P, "cannot access 'constructor': it is private in '%s'", f.Owner.DisplayName())
	}
	if f := x.Sym; f != nil && f.Ext {
		if lf, ok := f.Decl.(*syntax.LocalFun); ok {
			c.localExtCall(lf) // a local extension function's, x.g()
		}
	}
	return t
}

// checkTopVar reports a use of a property the code can't see: another
// file's private top-level property, or a class's private or protected
// property, which a lookup falls back to if nothing else has the name.
func (c *Checker) checkTopVar(v *types.VarSym, p syntax.Pos) {
	if (v.Kind == types.Global || v.Kind == types.Prop) && !c.propVisible(v) {
		c.invisibleProp(p, v)
	}
}

func (c *Checker) resolve1(x *syntax.Call, cands []callee, want types.Type, sink *system) types.Type {
	name := cands[0].f.Name
	byArity := cands[:0] // cands is the caller's own slice
	unsupported, why, feature := false, "", "inline-only call"
	var noCtx *types.FunSym
	for _, cl := range cands {
		if cl.f.Hidden {
			continue
		}
		amap, ok := mapArgs(cl.f, x)
		if !ok {
			continue
		}
		if cl.ctx, ok = c.contextArgs(&cl, x); !ok {
			noCtx = cl.f
			continue
		}
		if cl.f.Unsupported {
			unsupported = true
			if why == "" {
				why = cl.f.Why
				if cl.f.Feature != "" {
					feature = cl.f.Feature
				}
			}
			continue
		}
		cl.amap, cl.names = amap, x.Names
		byArity = append(byArity, cl)
	}
	if len(byArity) == 0 {
		switch {
		case x.Names != nil && c.namedArgError(x, cands):
			c.args(x, nil)
			return types.InvalidType
		case spreadMisplaced(x, cands):
			c.errorf(x.P, "the spread operator (*foo) can only be applied in a vararg position")
		case why != "":
			c.unsupported(x.P, feature, "%s", why)
		case noCtx != nil:
			c.noContextArg(x.P, noCtx)
		case unsupported:
			c.unsupported(x.P, "unsupported types in signature", "'%s' uses types kc doesn't support", name)
		default:
			c.errorf(x.P, "no function '%s' takes %d arguments", name, len(x.Args))
		}
		c.args(x, nil)
		return types.InvalidType
	}
	targs := c.typeArgs(x)
	// A generic call passed to one of several plain candidates, as
	// listOf() in this(name, listOf()), is inferred with the candidate it
	// fits: with the generic path, below.
	if targs == nil && !anyGeneric(byArity) && (len(byArity) == 1 || !hasLambda(x.Args) && !hasCall(x.Args)) {
		return c.resolvePlain(x, byArity, want)
	}
	if targs != nil {
		var ok []callee
		for _, cl := range byArity {
			if len(cl.tparams) == len(targs) {
				ok = append(ok, cl)
			}
		}
		if len(ok) == 0 {
			cl := byArity[0]
			c.errorf(x.TypeArgs[0].P, "%s expected for '%s%s'", countArgs(len(cl.tparams)), name, types.TypeParamsString(cl.tparams))
			c.args(x, nil)
			return types.InvalidType
		}
		byArity = ok
	}

	cs := make([]*candidate, len(byArity))
	for i, cl := range byArity {
		cd := &candidate{callee: cl, sys: newSystem(c)}
		if targs != nil {
			cd.sub = make(types.Subst, len(cl.recv)+len(targs))
			for p, a := range cl.recv {
				cd.sub[p] = a
			}
			for j, p := range cl.tparams {
				cd.sub[p] = types.Arg{T: targs[j]}
			}
		} else {
			cd.sub = cd.sys.fresh(cl.tparams, cl.recv, x.P)
		}
		cd.params = make([]types.Type, len(x.Args))
		for j := range x.Args {
			if cl.param(j).UnsafeVariance {
				cd.params[j] = cd.sub.Apply(cl.argType(j, x.Args))
			} else {
				cd.params[j] = cd.sub.ApplyIn(cl.argType(j, x.Args))
			}
		}
		cs[i] = cd
	}

	// Arguments are checked once. With a single candidate, a parameter
	// type without variables is the argument's expected type. Generic
	// calls passed as arguments are inferred together with this call.
	// Lambdas are checked last, once a candidate is chosen.
	single := len(cs) == 1
	plain := !single && !anyGeneric(byArity)
	ts := make([]types.Type, len(x.Args))
	var partials []*system
	var lambdas []int
	for i, a := range x.Args {
		if _, ok := a.(*syntax.Lambda); ok {
			lambdas = append(lambdas, i)
			ts[i] = types.InvalidType
			continue
		}
		if single && !cs[0].params[i].HasVars() {
			ts[i] = c.value(a, cs[0].params[i])
			continue
		}
		if p, ok := c.sharedFunParam(cs, i); ok {
			// Every candidate takes the same function type: vico's
			// persistentMarkers = remember { { ... } }, whose lambda has
			// the parameter's receiver.
			ts[i] = c.value(a, p)
			continue
		}
		if _, isCall := a.(*syntax.Call); isCall {
			inner := newSystem(c)
			c.sink = inner
			ts[i] = c.value(a, noWant)
			if plain && len(inner.lambdas) == 0 && inner.selfInferable(ts[i]) {
				// Among plain candidates, a call that can be inferred on
				// its own is: println(maxOf(a, b)) picks println(Int).
				inner.finish()
				ts[i] = inner.final(ts[i])
				continue
			}
			if len(inner.vars) > 0 || len(inner.fixups) > 0 {
				partials = append(partials, inner)
			}
			continue
		}
		ts[i] = c.value(a, noWant)
	}

	var best *candidate
	if single {
		best = cs[0]
		c.constrain(best, x, ts, partials, false)
	} else {
		for i, t := range ts {
			if _, lambda := x.Args[i].(*syntax.Lambda); t.K == types.Invalid && !lambda {
				return types.InvalidType
			}
		}
		// Integer literals are Int unless no candidate takes them as Int.
		match := c.applicable(cs, x, ts, partials, false)
		if len(match) == 0 {
			match = c.applicable(cs, x, ts, partials, true)
		}
		var maximal []*candidate
		for _, cd := range match {
			if c.mostSpecific(cd, match, x.Args) {
				maximal = append(maximal, cd)
			}
		}
		maximal = overriddenGiveWay(maximal)
		if len(maximal) == 0 {
			if fewer, ok := fewestSAM(match, x.Args, func(cd *candidate) *callee { return &cd.callee }); ok {
				for _, cd := range fewer {
					if c.mostSpecific(cd, fewer, x.Args) {
						maximal = append(maximal, cd)
					}
				}
			}
		}
		if i := preferred(len(maximal), x.Args, func(i int) *callee { return &maximal[i].callee }); i >= 0 {
			best = maximal[i]
		}
		if best == nil && len(match) > 1 {
			best = c.byLambdaResult(x, match)
		}
		if best == nil {
			names := make([]string, len(ts))
			for i, t := range ts {
				names[i] = t.String()
				if _, ok := x.Args[i].(*syntax.Lambda); ok {
					names[i] = "lambda"
				}
			}
			if len(match) == 0 {
				c.errorf(x.P, "none of the %d candidates for '%s' takes (%s)", len(cs), name, strings.Join(names, ", "))
			} else {
				c.errorf(x.P, "ambiguous call: %d candidates for '%s' take (%s)", len(match), name, strings.Join(names, ", "))
			}
			return types.InvalidType
		}
	}
	for i, a := range x.Args {
		if lit, ok := a.(*syntax.IntLit); ok && fitsLit(lit, best.params[i]) {
			lit.T = best.params[i].NonNull()
		}
	}

	for i, m := range best.sams {
		x.Args[i] = c.samConverted(x.Args[i], m, best.params[i])
	}
	for i := range best.susps {
		x.Args[i] = c.suspendConverted(x.Args[i], best.params[i])
	}
	f := best.f
	x.Sym, x.ArgMap = f, best.amap
	c.setDispatch(x, &best.callee)
	c.receiverShadowed(x, &best.callee)
	c.dslCall(x, &best.callee)
	c.setContexts(x, &best.callee, best.sys, best.sub)
	ensureFun(f)
	c.suspendCall(x.P, f)
	c.composeFunCall(x.P, f)
	c.inlineCall(x, &best.callee)
	ret := best.sub.Apply(f.Ret)
	if f.Name == "<init>" {
		ret = best.sub.Apply(f.Owner.This())
	}
	sys := best.sys
	params := best.params
	for _, i := range lambdas {
		l := x.Args[i].(*syntax.Lambda)
		if ft, ok := c.eagerRefType(l); ok {
			// As in Kotlin, a reference to the only function of its name
			// has its type before the variables are fixed: T of
			// test(0, ::byteRange) is Byte.
			trial := sys.clone()
			if trial.sub(ft, params[i]); trial.ok {
				sys = trial
			}
		}
		sys.lambdas = append(sys.lambdas, pendingLambda{l, params[i], c.argScope(x.Args, i)})
	}
	if best.ext.K != types.Invalid {
		recvParam := best.sub.ApplyIn(f.Recv)
		sys.fixups = append(sys.fixups, func(s *system) {
			want := s.final(recvParam)
			if view, ok := c.capturable(f, best.ext); ok {
				want = captured(view, recvParam, want)
			}
			c.assignable(best.ext, want, x.Fun.Pos())
		})
	}
	if hasReified(f) {
		sys.fixups = append(sys.fixups, func(s *system) { c.reifiedArgs(x, f, best.sub, s) })
	}
	sys.fixups = append(sys.fixups, func(s *system) {
		for i, a := range x.Args {
			if _, ok := a.(*syntax.Lambda); ok {
				continue // checked by postponedLambda
			}
			if ch, ok := a.(*syntax.Checked); ok && ch.SAM != nil {
				ch.T = s.final(params[i]).NonNull()
				continue
			}
			if lit, ok := a.(*syntax.IntLit); ok && !lit.Long && fitsLit(lit, s.final(params[i])) {
				lit.T = s.final(params[i]).NonNull()
			}
			c.assignable(a.Type(), captured(a.Type(), params[i], s.final(params[i])), a.Pos())
		}
		for _, p := range f.Params {
			if p.Vararg {
				x.Vararg = s.final(best.sub.ApplyIn(p.Type))
			}
		}
	})
	if sink != nil {
		c.runLambdas(sys, true)
		if sys.unfixed() || len(partials) > 0 || len(sys.lambdas) > 0 {
			sys.fixups = append(sys.fixups, func(s *system) { x.T = c.unitResult(f.Ret, s.final(x.T)) })
			sink.merge(sys)
			return ret
		}
	}
	if len(sys.lambdas) > 0 {
		// The expected type can help infer the lambdas' parameters.
		if want.K != types.Invalid && !want.HasVars() && ret.HasVars() {
			trial := sys.clone()
			trial.sub(ret, want)
			if trial.ok {
				sys = trial
			}
		}
		c.runLambdas(sys, false)
	}
	if want.K != types.Invalid && !want.HasVars() && ret.HasVars() {
		trial := sys.clone()
		trial.sub(ret, want)
		if trial.ok {
			sys = trial
		}
	}
	sys.finish()
	return c.unitResult(f.Ret, sys.final(ret))
}

// eagerRefType returns the type of a reference ::f to a local or top-level
// function, when it is the only one of its name, isn't generic and takes
// no vararg, and nothing else has the name, a member of this either.
func (c *Checker) eagerRefType(l *syntax.Lambda) (types.Type, bool) {
	r := l.Ref
	if r == nil || r.X != nil || r.Type != nil || r.Bound != nil || c.isVar(r.Name) || c.lookupClass(r.Name) != nil {
		return types.InvalidType, false
	}
	fs, _ := c.scope.findFuns(r.Name)
	if len(fs) == 0 {
		fs = c.topFuns(r.Name)
	}
	if len(fs) != 1 || c.cls != nil && c.hasCallable(c.cls, r.Name) {
		return types.InvalidType, false
	}
	f := fs[0]
	ensureFun(f)
	if f.Ext || f.Suspend || len(f.TypeParams) > 0 || f.Ret.K == types.Invalid {
		return types.InvalidType, false
	}
	var ps []types.Type
	for _, q := range f.Params {
		if q.Vararg {
			return types.InvalidType, false
		}
		ps = append(ps, q.Type)
	}
	return c.u.ext.funcs.Type(ps, f.Ret), true
}

// unitResult returns the result r of a call of a function whose declared
// result is decl: a nullable or platform type parameter, V? or a Java
// method's V, that is Unit is the type of kotlin.Unit or null, Unit?,
// whose value the call keeps.
func (c *Checker) unitResult(decl, r types.Type) types.Type {
	if r.K != types.Unit || !decl.IsParam() || !decl.N {
		return r
	}
	u := c.u.ext.class("kotlin/Unit")
	if u == nil {
		return r
	}
	if decl.P {
		return types.ClassType(u).Platform()
	}
	return types.ClassType(u).Nullable()
}

// byLambdaResult chooses among equally specific candidates that differ in
// the result type of a lambda argument's parameter, as Kotlin does for the
// standard library's functions marked @OverloadResolutionByLambdaReturnType,
// such as maxOf and sumOf: the lambda is checked first, with the parameter
// types all candidates give it, and the candidates whose result type it
// doesn't fit are dropped. The lambda isn't checked again.
func (c *Checker) byLambdaResult(x *syntax.Call, match []*candidate) *candidate {
	li := -1
	for i, a := range x.Args {
		if _, ok := a.(*syntax.Lambda); ok {
			if li >= 0 {
				return nil
			}
			li = i
		}
	}
	if li < 0 {
		return nil
	}
	l := x.Args[li].(*syntax.Lambda)
	var params []types.Type
	inl := true
	for k, cd := range match {
		sys := cd.sys.clone()
		ps, _, ok := funcParts(sys.apply(cd.params[li]))
		if !ok {
			return nil
		}
		for _, p := range ps {
			sys.fixVarsIn(p)
		}
		for i := range ps {
			if ps[i] = sys.final(ps[i]); ps[i].K == types.Invalid {
				return nil
			}
		}
		if k == 0 {
			params = ps
		} else if !slices.Equal(ps, params) {
			return nil
		}
		b, err := inlineBody(cd.f, cd.omitted(len(x.Args)) > 0)
		inl = inl && err == nil && b != nil && inlinable(cd.f, b, cd.paramIndex(li))
	}
	if inl {
		for _, cd := range match {
			b, _ := inlineBody(cd.f, cd.omitted(len(x.Args)) > 0)
			c.markInline(l, cd.f, b, cd.paramIndex(li))
		}
	}
	if cd := c.byLambdaResultClass(l, li, match, params); cd != nil {
		return cd
	}
	_, bt, _ := c.checkLambda(l, lambdaWant{params: params, known: true, recv: match[0].params[li].Flex().E, ctx: match[0].params[li].Flex().Contexts(), suspend: isSuspendFunc(match[0].params[li]), composable: isComposableFunc(match[0].params[li]), ret: noWant})
	if bt.K == types.Invalid {
		return nil
	}
	var fit []*candidate
	for _, cd := range match {
		_, r, _ := funcParts(cd.params[li])
		try := &candidate{callee: cd.callee, sub: cd.sub, params: cd.params, sys: cd.sys.clone(), sams: cd.sams, susps: cd.susps}
		try.sys.sub(bt, r)
		if try.sys.ok && c.withinBounds(try) {
			fit = append(fit, try)
		}
	}
	var maximal []*candidate
	for _, cd := range fit {
		if c.mostSpecific(cd, fit, x.Args) {
			maximal = append(maximal, cd)
		}
	}
	if len(maximal) != 1 {
		return nil
	}
	c.checked[l] = bt
	return maximal[0]
}

// byLambdaResultClass chooses among the candidates of byLambdaResult
// without checking lambda l, argument li, whose parameters have types
// params, when the call it ends with has a result whose class is known
// without checking it, and only one candidate's lambda result is of a
// superclass: flatMap { s -> s.all() } for an all that returns a List
// is the Iterable one, not the Sequence one. The lambda is then checked
// once, after the candidate is chosen, so that a generic call it ends
// with can be inferred with the call's other constraints.
func (c *Checker) byLambdaResultClass(l *syntax.Lambda, li int, match []*candidate, params []types.Type) *candidate {
	n := len(l.Body.Stmts)
	if n == 0 {
		return nil
	}
	es, ok := l.Body.Stmts[n-1].(*syntax.ExprStmt)
	if !ok {
		return nil
	}
	call, ok := es.X.(*syntax.Call)
	if !ok || hasLambda(call.Args) {
		return nil
	}
	var cls *types.ClassSym
	switch fn := call.Fun.(type) {
	case *syntax.Ident:
		if declaredIn(l)[fn.Name] {
			return nil
		}
		cls = c.peekClass(call)
	case *syntax.Member:
		// A member of one of the lambda's parameters: s.all().
		recv, ok := fn.X.(*syntax.Ident)
		if !ok || fn.Safe {
			return nil
		}
		var rt types.Type
		switch {
		case !l.Arrow && recv.Name == "it" && len(params) == 1:
			rt = params[0]
		case l.Arrow:
			for i, p := range l.Params {
				if p.Name == recv.Name && len(p.Names) == 0 && i < len(params) {
					rt = params[i]
				}
			}
		}
		if rt.K == types.Invalid || rt.HasVars() {
			return nil
		}
		cls = c.memberResultClass(rt.Flex().NonNull(), fn.Name)
	}
	if cls == nil {
		return nil
	}
	var fit []*candidate
	for _, cd := range match {
		if cd.params[li].Flex().E {
			return nil // a receiver's members may be called
		}
		_, r, ok := funcParts(cd.params[li])
		if !ok || r.K != types.Class {
			return nil
		}
		if cls.IsSubclassOf(r.C) {
			fit = append(fit, cd)
		}
	}
	if len(fit) != 1 {
		return nil
	}
	return fit[0]
}

// memberResultClass returns the class that every method name of a
// receiver of type rt returns, or nil.
func (c *Checker) memberResultClass(rt types.Type, name string) *types.ClassSym {
	cls := c.classOf(rt)
	if cls == nil {
		return nil
	}
	var out *types.ClassSym
	for _, f := range c.javaMethods(cls, name) {
		ensureFun(f)
		if !f.Resolved || f.Ret.K != types.Class || out != nil && f.Ret.C != out {
			return nil
		}
		out = f.Ret.C
	}
	return out
}

// spreadMisplaced reports whether a call spreads an array where none of
// the candidates takes a vararg.
func spreadMisplaced(x *syntax.Call, cands []callee) bool {
	spread := false
	for _, a := range x.Args {
		spread = spread || isSpread(a)
	}
	if !spread {
		return false
	}
	for _, cl := range cands {
		if cl.usesVararg() {
			return false
		}
	}
	return true
}

func anyGeneric(cs []callee) bool {
	for _, cl := range cs {
		if len(cl.tparams) > 0 || len(cl.recv) > 0 {
			return true
		}
	}
	return false
}

// resolvePlain resolves a call when no candidate is generic, without
// inference: arguments are checked against the parameter types.
func (c *Checker) resolvePlain(x *syntax.Call, cs []callee, want types.Type) types.Type {
	var best *callee
	if len(cs) == 1 {
		best = &cs[0]
		c.inlineCall(x, best) // before the lambdas are checked
		ts := c.args(x, best)
		c.samArgs(x, ts, func(i int) types.Type { return best.argType(i, x.Args) })
		for i, t := range ts {
			if ch, ok := x.Args[i].(*syntax.Checked); !ok || ch.SAM == nil {
				c.assignable(t, best.argType(i, x.Args), x.Args[i].Pos())
			}
		}
		if best.ext.K != types.Invalid {
			c.assignable(best.ext, best.f.Recv, x.Fun.Pos())
		}
	} else {
		ts := c.args(x, nil)
		for _, t := range ts {
			if t.K == types.Invalid {
				return types.InvalidType
			}
		}
		// Integer literals are Int unless no candidate takes them as Int.
		match := c.plainApplicable(cs, x.Args, ts, false)
		if len(match) == 0 {
			match = c.plainApplicable(cs, x.Args, ts, true)
		}
		var maximal []*callee
		for _, cl := range match {
			if c.plainMostSpecific(cl, match, x.Args) {
				maximal = append(maximal, cl)
			}
		}
		if len(maximal) == 0 {
			if fewer, ok := fewestSAM(match, x.Args, func(cl *callee) *callee { return cl }); ok {
				for _, cl := range fewer {
					if c.plainMostSpecific(cl, fewer, x.Args) {
						maximal = append(maximal, cl)
					}
				}
			}
		}
		if i := preferred(len(maximal), x.Args, func(i int) *callee { return maximal[i] }); i >= 0 {
			best = maximal[i]
		}
		if best == nil {
			names := make([]string, len(ts))
			for i, t := range ts {
				names[i] = t.String()
			}
			if len(match) == 0 {
				c.errorf(x.P, "none of the %d candidates for '%s' takes (%s)", len(cs), cs[0].f.Name, strings.Join(names, ", "))
			} else {
				c.errorf(x.P, "ambiguous call: %d candidates for '%s' take (%s)", len(match), cs[0].f.Name, strings.Join(names, ", "))
			}
			return types.InvalidType
		}
		for i, a := range x.Args {
			if lit, ok := a.(*syntax.IntLit); ok && fitsLit(lit, best.argType(i, x.Args)) {
				lit.T = best.argType(i, x.Args).NonNull()
			}
		}
		c.samArgs(x, ts, func(i int) types.Type { return best.argType(i, x.Args) })
	}
	f := best.f
	x.Sym, x.ArgMap = f, best.amap
	c.setDispatch(x, best)
	c.receiverShadowed(x, best)
	c.dslCall(x, best)
	c.setContexts(x, best, nil, nil)
	ensureFun(f)
	c.suspendCall(x.P, f)
	c.composeFunCall(x.P, f)
	if len(cs) > 1 {
		c.inlineCall(x, best)
	}
	for _, p := range f.Params {
		if p.Vararg {
			x.Vararg = p.Type
		}
	}
	if f.Name == "<init>" {
		return f.Owner.This()
	}
	return f.Ret
}

func (c *Checker) plainApplicable(cs []callee, args []syntax.Expr, ts []types.Type, longLits bool) []*callee {
	var out []*callee
	for k := range cs {
		cl := &cs[k]
		ok := cl.ext.K == types.Invalid || c.subtype(cl.ext, cl.f.Recv)
		for i, t := range ts {
			if !ok {
				break
			}
			p := cl.argType(i, args)
			if lit, ok := args[i].(*syntax.IntLit); ok && longLits && fitsLit(lit, p) {
				continue
			}
			_, sam := c.samValue(t, p)
			ok = sam || c.subtype(t, p)
		}
		if ok {
			out = append(out, cl)
		}
	}
	return out
}

func (c *Checker) plainMostSpecific(f *callee, cs []*callee, args []syntax.Expr) bool {
	for _, g := range cs {
		for i := range args {
			if fa, ga := f.argType(i, args), g.argType(i, args); boxedPrimitive(fa, ga) || !c.subtype(fa, ga) {
				return false
			}
		}
		if f.f.Ext && g.f.Ext && !c.subtype(f.f.Recv, g.f.Recv) {
			return false
		}
	}
	return true
}

// sharedFunParam returns the function type, without type variables, that
// every candidate takes for argument i, if they all do.
func (c *Checker) sharedFunParam(cs []*candidate, i int) (types.Type, bool) {
	p := cs[0].params[i]
	if p.HasVars() || !p.NonNull().IsFunc() {
		return types.InvalidType, false
	}
	for _, cd := range cs[1:] {
		if !c.sameType(cd.params[i], p) {
			return types.InvalidType, false
		}
	}
	return p, true
}

// constrain adds the constraints of a call's arguments to a candidate's
// system. If longLits, integer literals may be passed as Long.
func (c *Checker) constrain(cd *candidate, x *syntax.Call, ts []types.Type, partials []*system, longLits bool) {
	for _, p := range partials {
		cd.sys.merge(p.clone())
	}
	if cd.ext.K != types.Invalid {
		cd.sys.sub(cd.ext, cd.sub.ApplyIn(cd.f.Recv))
	}
	for i, t := range ts {
		p := cd.params[i]
		if lit, ok := x.Args[i].(*syntax.IntLit); ok && !lit.Long && !lit.Unsigned {
			if p.K == types.TypeVar && cd.sys.addLit(p, lit) {
				continue
			}
			if longLits && fitsLit(lit, p) {
				continue
			}
		}
		if ft, ok := c.suspendConstraint(t, p, x.Args[i]); ok {
			if cd.susps == nil {
				cd.susps = map[int]bool{}
			}
			cd.susps[i] = true
			cd.sys.sub(t, ft)
			continue
		}
		if m, ft, ok := c.samConstraint(cd, t, p, x.Args[i]); ok {
			if cd.sams == nil {
				cd.sams = map[int]*types.FunSym{}
			}
			cd.sams[i] = m
			cd.sys.sub(t, ft)
			continue
		}
		cd.sys.sub(t, p)
	}
	c.constrainContexts(cd, x)
}

// constrainContexts adds the constraints of a candidate's context
// arguments, once its arguments' are in its system. As in kotlinc, an
// argument of a generic context parameter is chosen again then: the
// nearest implicit value that fits with the arguments' constraints, as
// for context(t: T) f(b: Box<T>), with A and B in scope, f(Box<A>())
// takes the A. Those of @NoInfer types are left to setContexts.
func (c *Checker) constrainContexts(cd *candidate, x *syntax.Call) {
	if len(cd.ctx) > 0 {
		cd.ctx = slices.Clone(cd.ctx) // the callee's, which other attempts share
	}
	for k, a := range cd.ctx {
		if a.noInfer {
			continue
		}
		want := cd.sub.ApplyIn(cd.f.Contexts[k].Type)
		if want.HasVars() {
			v, t, ok, amb := c.contextValue(x.P, func(t types.Type) bool {
				trial := cd.sys.clone()
				trial.sub(t, want)
				return trial.ok && trial.litsFitUppers()
			})
			if ok {
				a = contextArg{x: v, t: t, ambiguous: amb}
				cd.ctx[k] = a
			}
		}
		cd.sys.sub(a.t, want)
	}
}

// suspendConstraint reports whether argument a, of type t, is a function
// value that a candidate takes for a suspend function type p only by
// suspend conversion (see suspendConverted), and returns the function
// type it takes without it: () -> String for suspend () -> String.
func (c *Checker) suspendConstraint(t, p types.Type, a syntax.Expr) (types.Type, bool) {
	pt := p.NonNull()
	switch a.(type) {
	case *syntax.Lambda, *syntax.Checked:
		return types.InvalidType, false
	}
	ft, _ := c.funcView(t, pt.A.Len()-1)
	ft = ft.NonNull()
	if !pt.IsSuspendFunc() || p.HasVars() || !ft.IsFunc() || ft.C.Suspend || ft.C.Composable ||
		ft.A.Len() != pt.A.Len() || types.MayBeNull(t) && !types.AcceptsNull(p) {
		return types.InvalidType, false
	}
	return c.unsuspended(p), true
}

// unsuspended returns suspend function type p without suspend: () ->
// String for suspend () -> String.
func (c *Checker) unsuspended(p types.Type) types.Type {
	pt := p.NonNull()
	ps := make([]types.Type, pt.A.Len()-1)
	for i := range ps {
		ps[i] = pt.A.List[i].T
	}
	plain := c.u.ext.funcs.Type(ps, pt.A.List[len(ps)].T)
	plain.E, plain.X = pt.E, pt.X
	if p.N {
		plain = plain.Nullable()
	}
	return plain
}

// suspendConverted converts function value a, of a function type that
// isn't suspend, to suspend function type pt, as kotlinc's suspend
// conversion does: a.let { f -> { x, y -> f(x, y) } }, with a suspend
// lambda of type pt, or a?.let for a nullable a.
func (c *Checker) suspendConverted(a syntax.Expr, pt types.Type) syntax.Expr {
	p := a.Pos()
	at := a.Type()
	fn := pt.NonNull()
	if ft, ok := c.funcView(at, fn.A.Len()-1); ok && at.Flex().NonNull().K == types.TypeParam {
		at = ft // a T : () -> Unit, as a () -> Unit
	}
	f := &syntax.Ident{Name: "$f"}
	f.P = p
	call := &syntax.Call{Fun: f}
	call.P = p
	inner := &syntax.Lambda{Arrow: true}
	inner.P = p
	for k := 0; k < fn.A.Len()-1; k++ {
		switch {
		case k < int(fn.X):
			// A context parameter, which the lambda names to pass it on.
			inner.AnonCtx = append(inner.AnonCtx, &syntax.Param{P: p, Name: refParam(k)})
		case k == int(fn.X) && fn.E:
			th := &syntax.This{}
			th.P = p
			call.Args = append(call.Args, th)
			continue
		default:
			inner.Params = append(inner.Params, &syntax.LambdaParam{P: p, Name: refParam(k)})
		}
		arg := &syntax.Ident{Name: refParam(k)}
		arg.P = p
		call.Args = append(call.Args, arg)
	}
	inner.Body = &syntax.Block{P: p, Stmts: []syntax.Stmt{&syntax.ExprStmt{X: call}}}
	outer := &syntax.Lambda{Arrow: true, Params: []*syntax.LambdaParam{{P: p, Name: "$f"}}}
	outer.P = p
	outer.Body = &syntax.Block{P: p, Stmts: []syntax.Stmt{&syntax.ExprStmt{X: inner}}}
	let := &syntax.Member{X: checked(a, at), Name: "let", Safe: types.MayBeNull(at)}
	let.P = p
	x := &syntax.Call{Fun: let, Args: []syntax.Expr{outer}}
	x.P = p
	c.expr(x, pt)
	return x
}

// samConstraint reports whether argument a, of type t, is a function
// value that a candidate takes for a fun interface p only by conversion
// (see samValue), and returns the interface's method and function type.
func (c *Checker) samConstraint(cd *candidate, t, p types.Type, a syntax.Expr) (*types.FunSym, types.Type, bool) {
	pt := p.NonNull()
	if _, ok := a.(*syntax.Lambda); ok || !samInterface(pt) || types.MayBeNull(t) && !types.AcceptsNull(p) {
		return nil, types.InvalidType, false
	}
	m, ft := c.samTarget(pt)
	if m == nil || m.Composable || m.Ext {
		return nil, types.InvalidType, false
	}
	vt, ok := c.funcView(t, len(m.Params))
	if !ok {
		return nil, types.InvalidType, false
	}
	if m.Suspend && vt.NonNull().IsFunc() && !vt.NonNull().C.Suspend {
		ft = c.unsuspended(ft) // converted to the suspend type too: see samConverted
	}
	trial := cd.sys.clone()
	if trial.sub(t, p); trial.ok {
		return nil, types.InvalidType, false
	}
	return m, ft, true
}

func (c *Checker) applicable(cs []*candidate, x *syntax.Call, ts []types.Type, partials []*system, longLits bool) []*candidate {
	var out []*candidate
	for _, cd := range cs {
		// Each attempt starts from a fresh copy of the candidate.
		try := &candidate{callee: cd.callee, sub: cd.sub, params: cd.params, sys: cd.sys.clone(), sams: cd.sams, susps: cd.susps}
		c.constrain(try, x, ts, partials, longLits)
		if try.sys.ok && c.lambdasFit(try, x) && c.withinBounds(try) {
			out = append(out, try)
		}
	}
	return out
}

// withinBounds reports whether a candidate's type parameters can satisfy
// their bounds, given the arguments: R.contains(element) for R :
// ClosedRange<T> doesn't take an array. It checks a copy of the system,
// so the bounds don't take part in inference.
func (c *Checker) withinBounds(cd *candidate) bool {
	sys := cd.sys.clone()
	for _, p := range cd.tparams {
		v := cd.sub[p].T
		if v.K != types.TypeVar {
			continue
		}
		for _, b := range v.Param().Bounds {
			sys.sub(v, b)
		}
	}
	return sys.ok
}

// mostSpecific reports whether f is at least as specific as every other
// candidate, as Kotlin decides: g accepts f's parameters as arguments.
// f's own type parameters are kept as they are, g's are inferred.
func (c *Checker) mostSpecific(f *candidate, cs []*candidate, args []syntax.Expr) bool {
	for _, g := range cs {
		if g == f {
			continue
		}
		sys := newSystem(c)
		sub := sys.fresh(g.tparams, g.recv, syntax.Pos{})
		// g's type parameters keep their bounds: T.use() for T :
		// Closeable is more specific than for T : AutoCloseable.
		for _, p := range g.tparams {
			v := sub[p].T
			for _, b := range v.Param().Bounds {
				sys.sub(v, b)
			}
		}
		for i := range args {
			fa, ga := f.recv.ApplyIn(f.argType(i, args)), sub.ApplyIn(g.argType(i, args))
			if boxedPrimitive(fa, ga) {
				return false
			}
			sys.sub(fa, ga)
		}
		if f.f.Ext && g.f.Ext {
			// A member extension's receiver with its dispatch receiver's
			// type arguments, as its parameters: Raise<Error>'s
			// List<Box<Error, A>>.bindAll() is more specific than its
			// Iterable<Box<Error, A>>.bindAll().
			sys.sub(f.recv.ApplyIn(f.f.Recv), sub.ApplyIn(g.f.Recv))
		}
		if !sys.ok {
			return false
		}
	}
	return true
}

// overriddenGiveWay drops the candidates among equally specific ones
// that another one, declared in a subclass of theirs with the same
// parameters, overrides: Kotlin sees only the override, as value class
// Tag's Int.ext(n) for the T.ext(n) of its supertype Ext<Int>, which
// dropOverridden keeps, as their JVM descriptors differ.
func overriddenGiveWay(cs []*candidate) []*candidate {
	if len(cs) < 2 {
		return cs
	}
	sameParams := func(f, g *types.FunSym) bool {
		return f.Ext == g.Ext && slices.EqualFunc(f.Params, g.Params, func(p, q *types.VarSym) bool { return p.Vararg == q.Vararg })
	}
	var out []*candidate
	for _, f := range cs {
		overridden := slices.ContainsFunc(cs, func(g *candidate) bool {
			return f.f.Owner != nil && g.f.Owner != nil && g.f.Owner != f.f.Owner && g.f.Name == f.f.Name &&
				g.f.Owner.IsSubclassOf(f.f.Owner) && sameParams(f.f, g.f)
		})
		if !overridden {
			out = append(out, f)
		}
	}
	return out
}

// boxedPrimitive reports whether a is a Java boxed primitive, Long!, and b
// the primitive, Long: getLong(String, long) is more specific than
// getLong(String, Long), though each type is a subtype of the other.
func boxedPrimitive(a, b types.Type) bool {
	return a.P && b.IsPrimitive() && a.K == b.K
}

// setDispatch records on a call of a member extension which object is its
// dispatch receiver, if this isn't.
func (c *Checker) setDispatch(x *syntax.Call, cl *callee) {
	if m, ok := x.Fun.(*syntax.Member); ok && cl.f.Ext && cl.f.Owner != nil {
		m.Dispatch = cl.dispatch
		if r := cl.dispatchRecv; r != nil {
			th := recvThis(r, m.P)
			c.value(th, noWant)
			m.DispatchThis = th
		}
	}
}
