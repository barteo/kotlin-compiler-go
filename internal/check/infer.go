package check

import (
	"fmt"
	"os"
	"slices"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// Type inference for calls of generic functions and constructors.
//
// Each type parameter of the callee becomes an inference variable, and
// each argument adds a constraint arg <: param. Constraints are reduced
// to lower and upper bounds on variables, by subtyping through generic
// supertypes and type arguments. Completing the system fixes each
// variable to the common supertype of its lower bounds, or failing
// that, its most specific upper bound.
//
// A generic call that is itself an argument of another call is not
// completed on its own: its variables join the outer call's system, so
// takesBox(Box(null)) infers Box's T from what takesBox expects. The
// inner call's type keeps its variables until the outer call completes
// and runs the fixups, which write the final types into the tree.

// tvar is an inference variable and its bounds.
type tvar struct {
	v     *types.TypeParamSym
	pos   syntax.Pos
	lower []types.Type
	upper []types.Type
	lits  []*syntax.IntLit // integer literals passed for the variable: Int, or Long if needed
	fixed bool
	// star is set when the variable stands for a captured star
	// projection, as T in KClass<T>.java for a KClass<*>: without other
	// bounds, it is its declared bound.
	star bool
	// starArg is set when star is for a star projection of an argument's
	// type, which is then known whatever variables the variable is equal
	// to; not for the parameters of a delegate's operators.
	starArg bool
	// proj is set when it stands for a captured out or in projection, as
	// T in Array<T> for an Array<out Enum<*>>: the capture is within T's
	// bounds, even where its approximation isn't.
	proj bool
	// captBound is set if the variable's bound is a captured type, of out
	// X or *, only Nothing is a subtype of.
	captBound bool
	// capt is the projection the variable is equal to, as T of Array<T>
	// is the capture of an Array<out Int>'s argument: the variable is
	// fixed to it, which Subst approximates as Kotlin does, out Int in
	// an invariant argument and Int at the top level (V is Inv if none).
	capt types.Arg
	// invRet is set when the variable is in an invariant or contravariant
	// position of its call's result type, as T of Bar<T>, where K2 infers
	// it from a T <: V? as T rather than T & Any.
	invRet bool
	// synth is set for a variable kc adds, which isn't a type parameter
	// the code can give: openResult's. One without bounds, of a lambda
	// whose body has errors, is the error type, without another error.
	synth bool
	// boundUp are the upper bounds that come from another variable's
	// declared bound, which, as K2's DeclaredUpperBoundConstraintPosition,
	// don't make the variable ready to fix.
	boundUp []types.Type
	// declUp are the upper bounds added from the declared bounds of the
	// call's own type parameters, R <: T of <T : A, R : T>: a variable
	// with no other constraints can't be inferred, as K2 has it.
	declUp []types.Type
}

type inferError struct {
	pos syntax.Pos
	msg string
}

type system struct {
	c      *Checker
	vars   []*tvar
	byVar  map[*types.TypeParamSym]*tvar
	vals   types.Subst // fixed variables
	fixups []func(*system)
	errs   []inferError
	// lambdas are lambda arguments not checked yet. A generic call that is
	// an argument of another call leaves those whose parameter types are
	// still unknown to the outer call.
	lambdas []pendingLambda
	ok      bool // false once a constraint cannot hold
	depth   int
	// nest counts the type arguments that sub is inside of: only a top-level
	// argument's projection is captured.
	nest int
	// contra counts the in-projected type arguments sub is inside of.
	contra int
	// declaring is set while the declared bounds of fresh variables are
	// added (see tvar.declUp).
	declaring int
	// forks are the fork points not resolved yet (see fork).
	forks []fork
}

// fork is K2's fork point: an intersection below a class type through
// more than one of its components, A<Int> & A<T> <: A<F>, each giving
// other constraints. The first consistent with the others is taken.
type fork struct {
	sups []types.Type // the components' instances of b's class
	b    types.Type
}

func newSystem(c *Checker) *system {
	return &system{c: c, ok: true}
}

// fresh adds a variable for each type parameter in ps and returns base
// extended with the substitution of parameters by variables.
func (s *system) fresh(ps []*types.TypeParamSym, base types.Subst, pos syntax.Pos) types.Subst {
	sub := make(types.Subst, len(base)+len(ps))
	for p, a := range base {
		sub[p] = a
	}
	if s.byVar == nil {
		s.byVar = map[*types.TypeParamSym]*tvar{}
	}
	vs := make([]*types.TypeParamSym, len(ps))
	for i, p := range ps {
		vs[i] = &types.TypeParamSym{Name: p.Name, Var: p.Var, Index: p.Index, Owner: p.Owner, Origin: p}
		sub[p] = types.Arg{T: types.VarType(vs[i])}
	}
	for i, p := range ps {
		tv := &tvar{v: vs[i], pos: pos}
		for _, b := range p.Bounds {
			if b.IsParam() && !b.N && (base[b.Param()].V == types.Out || base[b.Param()].V == types.Star) {
				// Below a capture of out X or *, as T : B of a Foo<*>'s
				// fun <T : B> bar(t: T), is only Nothing: no argument is.
				tv.captBound = true
			}
			vs[i].Bounds = append(vs[i].Bounds, sub.Apply(b))
		}
		s.vars = append(s.vars, tv)
		s.byVar[vs[i]] = tv
	}
	// A bound that mentions another of the variables relates them: C's
	// in <C : Iterable<T>> gives T from C's argument, as in onEach. So
	// does one that mentions a builder's variable that stubInference
	// infers: S of yieldAll<S : T>(seq: Inv<S>) on an Inv<U> is U's.
	for i := range ps {
		for _, b := range vs[i].Bounds {
			other := b.Has(func(u types.Type) bool {
				return u.K == types.TypeVar && u.Param() != vs[i] && (s.byVar[u.Param()] != nil || s.c.stubs[u.Param()] != nil)
			})
			if other {
				s.declaring++
				s.addUpper(s.byVar[vs[i]], b)
				s.declaring--
			}
		}
	}
	return sub
}

func (s *system) clone() *system {
	if len(s.vars) == 0 && len(s.fixups) == 0 && len(s.errs) == 0 && len(s.lambdas) == 0 {
		return &system{c: s.c, ok: s.ok}
	}
	n := &system{c: s.c, byVar: make(map[*types.TypeParamSym]*tvar, len(s.vars)), vals: make(types.Subst, len(s.vals)),
		fixups: append([]func(*system){}, s.fixups...), errs: append([]inferError{}, s.errs...), ok: s.ok,
		lambdas: append([]pendingLambda{}, s.lambdas...), forks: slices.Clip(s.forks)}
	for _, tv := range s.vars {
		c := *tv
		c.lower = append([]types.Type{}, tv.lower...)
		c.upper = append([]types.Type{}, tv.upper...)
		c.lits = append([]*syntax.IntLit{}, tv.lits...)
		n.vars = append(n.vars, &c)
		n.byVar[c.v] = &c
	}
	for k, v := range s.vals {
		n.vals[k] = v
	}
	return n
}

// merge takes over o's variables, constraints and fixups.
func (s *system) merge(o *system) {
	if len(o.vars) > 0 && s.byVar == nil {
		s.byVar = map[*types.TypeParamSym]*tvar{}
	}
	if len(o.vals) > 0 && s.vals == nil {
		s.vals = types.Subst{}
	}
	for _, tv := range o.vars {
		s.vars = append(s.vars, tv)
		s.byVar[tv.v] = tv
	}
	for k, v := range o.vals {
		s.vals[k] = v
	}
	s.fixups = append(s.fixups, o.fixups...)
	s.errs = append(s.errs, o.errs...)
	s.lambdas = append(s.lambdas, o.lambdas...)
	s.forks = append(s.forks, o.forks...)
	s.ok = s.ok && o.ok
}

// resolveForks takes, for each fork point, the first branch whose
// constraints are consistent with the others, or the first
// (applyTheBestBranchFromForkPoint).
func (s *system) resolveForks() {
	for len(s.forks) > 0 && s.ok {
		f := s.forks[0]
		s.forks = s.forks[1:]
		chosen := false
		for _, sup := range f.sups {
			t := s.clone()
			t.subArgs(sup, f.b)
			if t.ok {
				*s = *t
				chosen = true
				break
			}
		}
		if !chosen {
			s.subArgs(f.sups[0], f.b)
		}
	}
}

// forkSups returns the instances of b's class of the components of
// intersection a, if more than one, differing, give one.
func (s *system) forkSups(a, b types.Type) []types.Type {
	var sups []types.Type
	for _, comp := range a.Param().Bounds {
		if sup, ok := s.c.asSuper(comp, b.C); ok && !slices.Contains(sups, sup) {
			sups = append(sups, sup)
		}
	}
	if len(sups) < 2 {
		return nil
	}
	return sups
}

// apply replaces fixed variables in t by their values.
func (s *system) apply(t types.Type) types.Type {
	if !t.HasVars() {
		return t
	}
	return s.vals.Apply(t)
}

// proper reports whether t mentions no unfixed variable.
func (s *system) proper(t types.Type) bool { return !s.apply(t).HasVars() }

// sub adds the constraint a <: b.
func (s *system) sub(a, b types.Type) {
	if !s.ok || a.K == types.Invalid || b.K == types.Invalid {
		return
	}
	a, b = s.apply(a), s.apply(b)
	if r := s.c.stubConstraint(a, b); r != nil && !s.isVar(a.Flex()) && !s.isVar(b.Flex()) {
		// A builder's variable, in a call in its lambda: see stubInference.
		s.ok = r.add(a, b)
		return
	}
	if !a.HasVars() && !b.HasVars() {
		if !s.c.subtype(a, b) {
			s.ok = false
		}
		return
	}
	if s.depth > 40 {
		return
	}
	s.depth++
	defer func() { s.depth-- }()
	dnn := a.D && !a.P // T & Any, not a Java T!
	pa := a
	if a.P {
		a = a.Flex()
	}
	switch {
	case b.K == types.TypeVar && s.byVar[b.Param()] != nil:
		tv := s.byVar[b.Param()]
		switch {
		case pa.P && b.P:
			// T! <: V! gives V = T!, which takes null, not V = T & Any:
			// SortedComboBoxModel(Comparator.comparing(f)) is a
			// SortedComboBoxModel<DataView!>, as in Kotlin.
			a = pa
			if a.K == types.TypeVar && s.byVar[a.Param()] != nil {
				// As K2's (T & Any .. T?) <: V, a T! with D, which is X!
				// for T = X?: F of asList(x.toArray(y)) for a y of
				// Array<String?> is a String!.
				a.D = true
			}
		case b.N && !b.P && tv.invRet && a.K == types.TypeParam && !a.N && !a.D && !a.IsIntersection() && !s.nonNullBound(tv):
			// As K2 prefers it, T <: V? gives T <: V, not T & Any <: V,
			// where V may take T and is invariant or contravariant in the
			// call's result: Bar(t) for class Bar<T>(t: T?) and a t: T is
			// a Bar<T>, but removeQuestionMark(t) for fun <K>
			// removeQuestionMark(x: K?): K is a T & Any.
		case b.P && a.N && a.K != types.Null && s.contra == 0:
			// As K2 does, Foo? <: V! gives Foo? <: V: a Java <T> T
			// transform(T) called with a String? returns a String?. Not
			// in an in-projected type argument, where K2's equality constraints of
			// explicit type arguments, which kc substitutes, make it
			// flexible: S of Renderer.create { it.length } for a
			// Renderer<in T?> of comboBox<String> is a String!.
		case b.P && !a.N && a.K != types.TypeVar && a.K != types.Nothing:
			// And Foo <: V! gives Foo! <: V, a flexible lower bound,
			// which V is then fixed before others for: Key<T> <: Key<K!>
			// makes K a T!, which takes the T? of select(value, null).
			a = a.Platform()
		case b.N:
			a = a.NonNull() // a <: V? gives a!! <: V
		case b.D && types.MayBeNull(a):
			s.ok = false
			return
		case pa.P:
			// T! <: V gives V = T!, as in Kotlin, which then also takes
			// null: every { prefs.getString(k, null) } returns null.
			a = pa
		}
		s.addLower(tv, a)
		if a.K == types.TypeVar && !a.N && s.byVar[a.Param()] != nil {
			// The constraint bounds both: the other variable is then
			// fixed after this one.
			s.addUpper(s.byVar[a.Param()], b)
		}
	case a.K == types.TypeVar && s.byVar[a.Param()] != nil:
		tv := s.byVar[a.Param()]
		if a.N && !types.AcceptsNull(b) && !(b.K == types.TypeVar && b.N) {
			s.ok = false
			return
		}
		switch {
		case dnn:
			b = b.Nullable() // V & Any <: b gives V <: b?
		case pa.P && !b.N:
			// As K2 does, V! <: b gives V <: b!: Key<T> <: Key<K!> lets
			// K take a T? too.
			b = b.Platform()
		}
		s.addUpper(tv, b)
	case a.K == types.Nothing:
	case a.K == types.Null:
		if !types.AcceptsNull(b) {
			s.ok = false
		}
	default:
		if types.MayBeNull(a) && !types.AcceptsNull(b) {
			s.ok = false
			return
		}
		switch b.K {
		case types.Any:
		case types.Class:
			if a.IsIntersection() && b.HasVars() {
				if sups := s.forkSups(a, b); sups != nil {
					s.forks = append(s.forks, fork{sups, b})
					return
				}
			}
			sup, ok := s.c.asSuper(a, b.C)
			if ro, flex := readOnlyView(b); !ok && flex {
				sup, ok = s.c.asSuper(a, ro.C)
				b = ro
			}
			if !ok {
				s.ok = false
				return
			}
			s.subArgs(sup, b)
		default:
			s.ok = s.c.subtype(a, b)
		}
	}
}

// subArgs constrains the arguments of sup, an instance of b's class, by
// the variance of b's arguments.
func (s *system) subArgs(sup, b types.Type) {
	if b.A == nil || sup.A == nil || sup.A.Len() != b.A.Len() {
		return
	}
	for i, ba := range b.A.List {
		if ba.V == types.Star {
			continue
		}
		v := ba.V
		if v == types.Inv && i < len(b.C.TypeParams) {
			v = b.C.TypeParams[i].Var
		}
		ta := sup.A.List[i]
		switch v {
		case types.Out:
			s.nest++
			switch ta.V {
			case types.Star:
				s.sub(starArgBound(sup, i), ba.T)
			case types.In:
				s.sub(types.AnyType.Nullable(), ba.T)
			default:
				s.sub(ta.T, ba.T)
			}
			s.nest--
		case types.In:
			s.nest++
			s.contra++
			if ta.V == types.Star || ta.V == types.Out {
				s.sub(ba.T, types.NothingType)
			} else {
				s.sub(ba.T, ta.T)
			}
			s.contra--
			s.nest--
		default:
			if ta.V != types.Inv {
				// A projection captured by a variable: T in Array<T> for
				// an Array<out X> is X's capture, which kc approximates
				// by its bound. A Java method's T! is the variable too.
				tv := s.unfixedVar(ba.T.Flex())
				if tv == nil {
					s.ok = false
					return
				}
				if s.nest == 0 && !ba.T.D && ta.V != types.Star && !ta.T.HasVars() {
					tv.capt = ta
				}
				switch ta.V {
				case types.Out:
					tv.proj = true
					s.addLower(tv, ta.T)
				case types.In:
					tv.proj = true
					s.addUpper(tv, ta.T)
				default:
					tv.star = true
					tv.starArg = true
					if s.nest == 0 && !ba.T.D && tv.capt.V == types.Inv && !selfBounded(sup.C, i) {
						// A star's capture, which a parameter of the
						// argument's supertype takes too: Fu<F> for a CF<*>.
						// A self type's is the variable's own star.
						tv.capt = types.Arg{V: types.Out, T: starArgBound(sup, i)}
					}
				}
				continue
			}
			s.nest++
			s.sub(ta.T, ba.T)
			s.sub(ba.T, ta.T)
			s.nest--
		}
	}
}

// markInvRet sets invRet on the variables in invariant or contravariant
// positions of t, a call's result type.
func (s *system) markInvRet(t types.Type) {
	var walk func(t types.Type, inv bool)
	walk = func(t types.Type, inv bool) {
		switch t.K {
		case types.TypeVar:
			if tv := s.byVar[t.Param()]; tv != nil && inv {
				tv.invRet = true
			}
		case types.Class:
			if t.A == nil {
				return
			}
			for i, a := range t.A.List {
				v := a.V
				if v == types.Inv && i < len(t.C.TypeParams) {
					v = t.C.TypeParams[i].Var
				}
				if v != types.Star {
					walk(a.T, inv || v != types.Out)
				}
			}
		}
	}
	walk(t, false)
}

// nonNullBound reports whether one of tv's declared bounds excludes null.
func (s *system) nonNullBound(tv *tvar) bool {
	for _, b := range tv.v.Bounds {
		if b = s.apply(b); !types.AcceptsNull(b) {
			return true
		}
	}
	return false
}

// unfixedVar returns the variable that t is, if it is an unfixed one of
// the system's, without nullability.
func (s *system) unfixedVar(t types.Type) *tvar {
	t = s.apply(t)
	if t.K != types.TypeVar || t.N {
		return nil
	}
	if tv := s.byVar[t.Param()]; tv != nil && !tv.fixed {
		return tv
	}
	return nil
}

func (s *system) addLower(tv *tvar, t types.Type) {
	for _, l := range tv.lower {
		if l == t {
			return
		}
	}
	tv.lower = append(tv.lower, t)
	for _, u := range tv.upper {
		s.sub(t, u)
	}
}

func (s *system) addUpper(tv *tvar, t types.Type) {
	for _, u := range tv.upper {
		if u == t {
			if s.declaring == 0 {
				// A constraint of the call's own, not only declared.
				tv.declUp = slices.DeleteFunc(tv.declUp, func(d types.Type) bool { return d == t })
			}
			return
		}
	}
	tv.upper = append(tv.upper, t)
	if s.declaring > 0 {
		tv.declUp = append(tv.declUp, t)
	}
	for _, l := range tv.lower {
		s.sub(l, t)
	}
	// Below another variable, it is below that one's declared bound too,
	// as in Kotlin: X of f<X>(g<Y : Any>()) for f's G<X> and g's G<Y> is
	// Any.
	if w := s.unfixedVar(t); w != nil && w != tv {
		for _, b := range w.v.Bounds {
			if b = s.apply(b); !b.HasVars() && !(b.K == types.Any && b.N) {
				if !slices.Contains(tv.upper, b) {
					tv.boundUp = append(tv.boundUp, b)
				}
				s.addUpper(tv, b)
			}
		}
	}
}

// addLit records an integer literal passed where variable v is expected.
func (s *system) addLit(v types.Type, lit *syntax.IntLit) bool {
	tv := s.byVar[v.Param()]
	if tv == nil || tv.fixed {
		return false
	}
	tv.lits = append(tv.lits, lit)
	return true
}

// freeLits reports whether an unfixed variable of s has integer literals
// among its bounds and no upper bound but Any: the literals' type is up
// to the call that the variable's call is an argument of. Not that of
// id(5) for <T : Long> id(t: T).
func (s *system) freeLits() bool {
	for _, tv := range s.vars {
		if tv.fixed || len(tv.lits) == 0 {
			continue
		}
		free := true
		for _, u := range append(slices.Clip(tv.upper), tv.v.Bounds...) {
			if u = s.apply(u); u.K != types.Any {
				free = false
			}
		}
		if free {
			return true
		}
	}
	return false
}

// litsFit reports whether integer literals can all have type t, a Long,
// Byte or Short.
func litsFit(lits []*syntax.IntLit, t types.Type) bool {
	for _, lit := range lits {
		if !fitsLit(lit, t) {
			return false
		}
	}
	return true
}

// litsFitUppers reports whether the integer literals of each variable
// can be of the types it is known to be below: not those of t in f(1)
// for f(t: T) once Box<String> is a Box<T>.
func (s *system) litsFitUppers() bool {
	for _, tv := range s.vars {
		if tv.fixed || len(tv.lits) == 0 {
			continue
		}
		for _, u := range tv.upper {
			if u = s.apply(u); !u.HasVars() && !s.c.subtype(types.IntType, u) && !litsFit(tv.lits, u) {
				return false
			}
		}
	}
	return true
}

// unfixed reports whether any variable is not fixed yet.
func (s *system) unfixed() bool {
	for _, tv := range s.vars {
		if !tv.fixed {
			return true
		}
	}
	return false
}

// complete fixes every variable, in an order that fixes a variable only
// once the bounds it depends on are known.
func (s *system) complete() {
	s.resolveForks()
	for {
		var pick *tvar
		best := -2
		for _, tv := range s.vars {
			if tv.fixed {
				continue
			}
			if len(tv.lower) == 0 && len(tv.upper) == 0 && len(tv.lits) == 0 && !(tv.star && s.isUpperOf(tv)) {
				// Without bounds, last: fixing the others may give it
				// some, as T's value Map<String, Int> does K and V in
				// stateIn(initialValue = emptyMap()) on a Flow<Map<..>>.
				// A captured star that another variable is below is
				// known: G of <G : Enum<G>, T : G> on an Enum<*> is
				// fixed before T. As an argument of another's bound, ER
				// in ED : EventData<ER>, it still waits.
				if best < -1 {
					pick, best = tv, -1
				}
				continue
			}
			// Prefer variables whose bounds are all known, then those with
			// a known lower bound.
			score := 0
			allProper := true
			nested := false
			for _, l := range tv.lower {
				if s.proper(l) {
					score = 2
				} else if !(tv.starArg && s.isVar(l)) {
					// A captured star is known whatever the variables it
					// is equal to: A of id's Inv<A> for a star() of Inv<*>
					// in hh(id(star())), which hh's A then is.
					allProper = false
					nested = nested || !s.isVar(l)
				}
			}
			// An upper bound that is another variable, as T2 in T1 <: T2,
			// is that variable's lower bound too, which is fixed first.
			varUppers, boundUppers := 0, 0
			for _, u := range tv.upper {
				switch {
				case slices.Contains(tv.boundUp, u) && s.proper(u):
					// Known, but not one to make the variable ready
					// before one with a constraint of the call's own.
					boundUppers++
					score = max(score, 1)
				case s.proper(u):
					score = max(score, 1)
				case s.isVar(u):
					varUppers++
				default:
					allProper = false
				}
			}
			if len(tv.lits) > 0 {
				score = max(score, 1)
			}
			if slices.ContainsFunc(tv.lower, func(l types.Type) bool { return l.P && s.proper(l) }) {
				// K2's HAS_PROPER_FLEXIBLE_LOWER_CONSTRAINT.
				score += 4
			}
			if allProper && (varUppers == 0 || len(tv.upper)-boundUppers > varUppers || len(tv.lower) > 0 || len(tv.lits) > 0) {
				score += 3
			}
			if nested && score > 0 {
				// A lower bound with variables in it, as B<T2> of
				// listOf(a(), b()), waits for them: its type then counts.
				score--
			}
			if score > best {
				pick, best = tv, score
			}
		}
		if pick == nil {
			return
		}
		if k2Readiness != "kc" {
			var k2 *tvar
			bestR := -1
			for _, tv := range s.vars {
				if !tv.fixed {
					if r := s.readiness(tv); r > bestR {
						k2, bestR = tv, r
					}
				}
			}
			if k2 != pick {
				if k2Readiness == "log" {
					fmt.Fprintf(os.Stderr, "READINESS %s: kc %s, K2 %s\n", pick.pos, pick.v.Name, k2.v.Name)
				} else {
					pick = k2
				}
			}
		}
		s.fix(pick)
	}
}

// k2Readiness, from $KC_READINESS, keeps complete's own order of fixing
// ("kc"), or keeps it and logs where K2's readiness order differs
// ("log"); otherwise complete fixes variables in K2's order.
var k2Readiness = os.Getenv("KC_READINESS")

// isUpperOf reports whether tv is itself an upper bound of another
// unfixed variable.
func (s *system) isUpperOf(tv *tvar) bool {
	for _, o := range s.vars {
		if o == tv || o.fixed {
			continue
		}
		for _, u := range o.upper {
			if u.K == types.TypeVar && u.Param() == tv.v {
				return true
			}
		}
	}
	return false
}

// fix chooses a value for a variable from its known bounds.
func (s *system) fix(tv *tvar) {
	if len(s.forks) > 0 {
		s.resolveForks()
		tv = s.byVar[tv.v] // a clone's, if a branch was taken
	}
	// An unfixed variable below it with only integer literals passes
	// them on, as K2 incorporates ILT <: T1 and T1 <: T: T of both(id(1),
	// id("")) has 1 and "" below.
	for _, l := range tv.lower {
		if w := s.unfixedVar(l); w != nil && w != tv && len(w.lits) > 0 && len(w.lower) == 0 {
			for _, lit := range w.lits {
				if !slices.Contains(tv.lits, lit) {
					tv.lits = append(tv.lits, lit)
				}
			}
		}
	}
	c := s.c
	var lowers, uppers []types.Type
	var stubs []types.Type
	for _, l := range tv.lower {
		if l = s.apply(l); !l.HasVars() {
			// listOf(set(key, null)) is a List<Nothing?>.
			lowers = append(lowers, types.ApproxFlexNothing(l))
		} else if s.c.onlyStubs(l) {
			stubs = append(stubs, l)
		}
	}
	if len(lowers) == 0 && len(tv.lits) == 0 && len(stubs) == 1 {
		// Only a builder's variable: see stubInference.
		lowers = stubs
	}
	var upperStubs []types.Type
	for _, u := range tv.upper {
		if u = s.apply(u); !u.HasVars() {
			uppers = append(uppers, u)
		} else if s.c.onlyStubs(u) {
			upperStubs = append(upperStubs, u)
		}
	}
	if len(lowers) == 0 && len(tv.lits) == 0 && len(uppers) == 0 && len(upperStubs) == 1 {
		// Or one it is below: cont.resumeWith(failure(e)) for a
		// Continuation<T> gives failure's Result<R> R <: T.
		uppers = upperStubs
	}
	if len(uppers) > 0 && len(tv.lits) == 0 && len(lowers) == 1 && lowers[0].K == types.Null {
		// As K2, Nothing? from a null alone yields to an upper bound:
		// bar(null, r) ?: x with T String? for an expected String?.
		lowers = nil
	}
	if len(lowers) > 0 && len(tv.lits) == 0 && tv.v.Origin != nil && tv.v.Origin.Reified &&
		!slices.ContainsFunc(lowers, func(l types.Type) bool { return l.K != types.Nothing && l.K != types.Null }) {
		// Nothing is no reified type argument: as K2, one from lower
		// bounds that are all Nothing yields to an upper bound, a
		// declared one too (T of takeReified<T : Bound>(null) is Bound).
		ups := uppers
		if len(ups) == 0 {
			for _, b := range tv.v.Bounds {
				if b = s.apply(b); !b.HasVars() && !(b.K == types.Class && b.C == types.AnyType.C && b.N) {
					ups = append(ups, b)
				}
			}
		}
		if len(ups) > 0 {
			lowers, uppers = nil, ups
		}
	}
	// Without other bounds, a declared bound that is a self type counts
	// as one (see selfTypeBounds).
	var selfStar []types.Type
	if len(lowers) == 0 && len(tv.lits) == 0 && len(uppers) == 0 && !tv.star && !tv.synth {
		if bs, own, ok := s.selfTypeBounds(tv); ok {
			if len(bs) > 0 {
				uppers = bs
			} else {
				selfStar = own
			}
		}
	}
	val := types.InvalidType
	captured := false
	declOnly := len(tv.lower) == 0 && len(tv.lits) == 0 && len(tv.upper) > 0 && !tv.star && !tv.synth &&
		!slices.ContainsFunc(tv.upper, func(u types.Type) bool { return !slices.Contains(tv.declUp, u) })
	if declOnly {
		// A builder's variable is inferred from its lambda's stubs.
		_, hinted := s.stubHint(tv)
		declOnly = !hinted && len(s.c.stubs) == 0 && len(stubs) == 0 && len(upperStubs) == 0 && len(s.lambdas) == 0
	}
	switch {
	case declOnly:
		// Only its declared bounds: R of <T : A, R : T> in a call that
		// gives neither.
		s.errs = append(s.errs, inferError{tv.pos, "cannot infer type for type parameter '" + tv.v.Name + "'. Specify it explicitly."})
	case len(lowers) > 0:
		val = lowers[0]
		for _, l := range lowers[1:] {
			val = c.lub(val, l)
		}
		if len(tv.lits) > 0 && !litsFit(tv.lits, val) {
			val = c.lub(val, s.litType(tv, nil))
		}
		if len(lowers) > 1 && len(tv.lits) == 0 {
			val = s.withinUppers(val, lowers, uppers)
		}
		// A nullable lower bound with variables makes it nullable too, as
		// in Kotlin: Out<String> and Out<M>? give Out<String>?, and M
		// String.
		if !types.MayBeNull(val) && slices.ContainsFunc(tv.lower, func(l types.Type) bool {
			l = s.apply(l)
			return l.HasVars() && l.N && l.K != types.TypeVar
		}) {
			val = val.Nullable()
		}
		if isUnsigned(val) && val.NonNull().C.Name == "kotlin/UInt" {
			// A UInt from unsigned literals is the variable's ULong,
			// UByte or UShort bound, as T of f<T : UByte>(1u): the call
			// retypes the literals that fit, and those that don't, or a
			// UInt that is no literal, are not passed.
			for _, b := range tv.v.Bounds {
				if b = s.apply(b); !b.HasVars() && isUnsigned(b) && !c.subtype(val, b) {
					val = b.NonNull()
				}
			}
		}
	case len(tv.lits) > 0:
		val = s.litType(tv, append(uppers, s.litUppers(tv)...))
	case len(uppers) > 0:
		val = uppers[0]
		for _, u := range uppers[1:] {
			switch {
			case c.subtype(u, val):
				val = u
			case !types.MayBeNull(val) && c.subtype(u.NonNull(), val):
				val = u.NonNull() // Any and List<Any>? give List<Any>
			case !types.MayBeNull(u) && c.subtype(val.NonNull(), u):
				val = val.NonNull()
			case u.IsParam() && !u.N && types.MayBeNull(val) && !c.subtype(val, u) && c.subtype(val.NonNull(), u):
				// Z? and an M that may be null, for Z : M, give Z, below
				// both, for K2's Z? & M.
				val = val.NonNull()
			}
		}
		// Bounds that none of the others is below give their
		// intersection: E <: Int and E <: String, of In<E> for an
		// In<Int> and an In<String>, give Int & String, of which kotlinc
		// only warns that it is empty, with declared bounds too (a
		// candidate with one is inapplicable among others: see
		// emptyUppers).
		comps := []types.Type{val}
		for _, u := range uppers {
			if !c.subtype(val, u) && intersectable(u) && intersectable(val) {
				comps = append(comps, u)
			}
		}
		if len(comps) > 1 {
			val = c.intersect(comps)
		}
		// A non-null bound intersects a nullable upper bound: T? <: X?
		// for T : Annotation gives T = X, and T <: A, for a type parameter
		// A with a nullable bound and T : Any, gives T = A & Any.
		if types.MayBeNull(val) {
			for _, b := range tv.v.Bounds {
				// A class bound's nullability doesn't depend on variables
				// in its arguments: T? <: TestEnum? for T : Enum<T>.
				if b = s.apply(b); (!b.HasVars() || b.K == types.Class) && !types.AcceptsNull(b) {
					val = val.NonNull()
					break
				}
			}
		}
	case tv.star:
		captured = true // within its bounds by construction
		val = types.AnyType.Nullable()
		for _, b := range tv.v.Bounds {
			// In its own bound, the variable is a star: E : Enum<E> gives
			// Enum<*>.
			if b = (types.Subst{tv.v: {V: types.Star}}).Apply(s.apply(b)); !b.HasVars() {
				val = b
			}
		}
	case len(selfStar) > 0:
		// Kotlin's captured star of the variable's own bounds, which kc
		// approximates by them: B of <B : Builder<B>> is Builder<*>.
		captured = true
		val = selfStar[0]
		if len(selfStar) > 1 {
			val = c.intersect(selfStar)
		}
	case tv.synth:
		val = types.InvalidType
	default:
		if h, ok := s.stubHint(tv); ok && len(s.c.stubs) == 0 {
			val = h
			break
		}
		s.errs = append(s.errs, inferError{tv.pos, "cannot infer type for type parameter '" + tv.v.Name + "'. Specify it explicitly."})
	}
	// A variable with only upper constraints is their intersection with
	// its declared bounds, as K2 infers it. kc has no intersection types:
	// it takes a bound below the constraints, and otherwise keeps them,
	// empty or not: S of fun <S : Out<B>> for Foo<in S> and a Foo<K> of
	// K : Out<L> is K & Out<B>.
	upperOnly := len(lowers) == 0 && len(tv.lits) == 0 && len(uppers) > 0
	inBounds := map[int]bool{}
	if upperOnly {
		for i, b := range tv.v.Bounds {
			if b = s.apply(b); b.HasVars() || c.subtype(val, b) {
				continue
			}
			switch {
			case c.subtype(b, val):
				val = b
				inBounds[i] = true
			case !types.MayBeNull(val) && c.subtype(b.NonNull(), val):
				val = b.NonNull()
				inBounds[i] = true
			case tv.v.Origin != nil && tv.v.Origin.Reified:
				// An intersection is no reified type argument.
				s.errs = append(s.errs, inferError{tv.pos, "type argument for reified type parameter '" + tv.v.Name +
					"' was inferred to the intersection of ['" + b.String() + "' & '" + val.String() + "']"})
				inBounds[i] = true
			default:
				// K2 infers the intersection, of which kotlinc only warns
				// if it is empty, as T of fun <T : Foo> foo(): T? for a
				// Bar? is Foo & Bar; kc keeps the constraints'.
				inBounds[i] = true
			}
		}
	}
	if !s.onlyInputOK(tv, val) {
		s.errs = append(s.errs, inferError{tv.pos, "type inference failed. The value of the type parameter '" + tv.v.Name +
			"' must be mentioned in input types (argument types, receiver type, or expected type). Try to specify it explicitly."})
	}
	tv.fixed = true
	if s.vals == nil {
		s.vals = types.Subst{}
	}
	s.vals[tv.v] = types.Arg{T: val}
	if val.K == types.Invalid {
		return
	}
	// A declared bound that mentions it is now the fixation's
	// constraint, which K2 counts: T <: G of <G : Enum<G>, T : G> once G
	// is known from the receiver.
	for _, o := range s.vars {
		if len(o.declUp) > 0 {
			o.declUp = slices.DeleteFunc(o.declUp, func(u types.Type) bool {
				return u.Has(func(w types.Type) bool { return w.K == types.TypeVar && w.Param() == tv.v })
			})
		}
	}
	switch {
	case tv.capt.V != types.Inv:
		defer func() { s.vals[tv.v] = tv.capt }()
	case len(selfStar) == 1 && len(tv.v.Bounds) == 1:
		// The captured star itself, as Klass<T> is Klass<*> for it, not
		// Klass<Klass<*>>; T alone is still Klass<*>.
		defer func() { s.vals[tv.v] = types.Arg{V: types.Star} }()
	}
	for _, l := range tv.lower {
		s.sub(l, val)
	}
	for _, u := range tv.upper {
		s.sub(val, u)
	}
	if tv.captBound && len(lowers) > 0 && !captured && !tv.proj && val.K != types.Nothing {
		s.errs = append(s.errs, inferError{tv.pos, "type argument is not within its bounds: inferred type " +
			val.String() + " of type parameter '" + tv.v.Name + "' is not a subtype of its bound's captured type"})
	}
	for i, b := range tv.v.Bounds {
		if b = s.apply(b); !captured && !tv.proj && !inBounds[i] && !b.HasVars() && !c.subtype(val, b) {
			s.errs = append(s.errs, inferError{tv.pos, "type argument is not within its bounds: inferred type " +
				val.String() + " of type parameter '" + tv.v.Name + "' is not a subtype of " + b.String()})
		}
	}
}

// selfTypeBounds reports whether tv, which has no other bounds, is
// inferred from its declared bounds, as K2 does when one of them is a
// self type (VariableFixationFinder.areAllProperConstraintsSelfTypeBased):
// T of <T : B> on a Builder<B : Builder<B>>, or on a Builder<K> in
// <K : Builder<K>>. A declared bound is no information otherwise: T of
// <T : CharSequence> needs another. None may depend on another unfixed
// variable. bs are the bounds without tv in them, which K2 infers tv
// from; own, where there are none, are those with tv, with a star for
// it (ResultTypeResolver.getDefaultTypeForSelfType): B of <B :
// Builder<B>> is Builder<*>.
func (s *system) selfTypeBounds(tv *tvar) (bs, own []types.Type, ok bool) {
	self := false
	for _, b := range tv.v.Bounds {
		b = s.apply(b)
		if b.Has(func(u types.Type) bool {
			if u.K != types.TypeVar || u.Param() == tv.v {
				return false
			}
			o := s.byVar[u.Param()]
			return o != nil && !o.fixed
		}) {
			return nil, nil, false
		}
		if b.Has(func(u types.Type) bool { return u.K == types.TypeVar && u.Param() == tv.v }) {
			if b.K == types.Class && isSelfType(b) {
				own = append(own, starredVar(b, tv.v))
				self = true
			}
			continue
		}
		self = self || isSelfType(b)
		bs = append(bs, b)
	}
	if !self || len(bs) == 0 && len(own) == 0 {
		return nil, nil, false
	}
	return bs, own, true
}

// isSelfType reports whether t is a self type, as K2's
// hasRecursiveTypeParametersWithGivenSelfType or isRecursiveTypeParameter
// has it: a class type with a type parameter bounded by the class with
// itself in it, as Enum<E : Enum<E>>, an intersection with one, or a
// type parameter whose bound has it.
func isSelfType(t types.Type) bool {
	switch {
	case t.IsIntersection():
		return slices.ContainsFunc(t.Param().Bounds, isSelfType)
	case t.K == types.TypeParam:
		p := t.Param()
		return slices.ContainsFunc(p.Bounds, func(b types.Type) bool { return mentionsParam(b, p) })
	case t.K == types.Class && t.C != nil:
		t.C.Ensure() // a Java class's type parameters' bounds
		for _, p := range t.C.TypeParams {
			for _, b := range p.Bounds {
				if b.K == types.Class && b.C == t.C && mentionsParam(b, p) {
					return true
				}
			}
		}
	}
	return false
}

func mentionsParam(t types.Type, p *types.TypeParamSym) bool {
	return t.Has(func(u types.Type) bool { return u.IsParam() && u.Param() == p })
}

// starredVar replaces the arguments that are v in t by stars, at any depth.
func starredVar(t types.Type, v *types.TypeParamSym) types.Type {
	if t.K != types.Class || t.A == nil {
		return t
	}
	args := make([]types.Arg, t.A.Len())
	for i, a := range t.A.List {
		switch {
		case a.V == types.Star:
			args[i] = a
		case a.T.K == types.TypeVar && a.T.Param() == v:
			args[i] = types.Arg{V: types.Star}
		default:
			args[i] = types.Arg{V: a.V, T: starredVar(a.T, v)}
		}
	}
	t.A = types.MakeArgs(args)
	return t
}

// litUppers returns the Long, Short or Byte types that the unfixed
// variables above tv are known to be above: the integer literals passed
// for tv take such a type, as 1 of has("a" to 1) for has(element:
// Pair<K, V>) once a Map<String, Long> gives V the Long.
func (s *system) litUppers(tv *tvar) []types.Type {
	var out []types.Type
	for _, u := range tv.upper {
		if u = s.apply(u); u.K != types.TypeVar || u.N {
			continue
		}
		uv := s.byVar[u.Param()]
		if uv == nil || uv.fixed || len(uv.lits) > 0 {
			continue
		}
		t := types.InvalidType
		for _, l := range uv.lower {
			if l = s.apply(l); l.HasVars() {
				continue // tv itself, or another variable below it
			}
			if t.K != types.Invalid && l != t {
				t = types.InvalidType
				break
			}
			t = l
		}
		if (t.K == types.Long || t.K == types.Short || t.K == types.Byte) && !t.N {
			out = append(out, t)
		}
	}
	return out
}

// litType is the type of the integer literals passed for a variable: Int,
// unless the variable must be a Long, Byte or Short that they fit, below
// one of uppers or by its declared bound, as T of C<T : Long>(1).
func (s *system) litType(tv *tvar, uppers []types.Type) types.Type {
	t := types.IntType
	for _, u := range uppers {
		if litsFit(tv.lits, u) {
			t = u.NonNull()
		}
	}
	if t != types.IntType {
		return t
	}
	for _, b := range tv.v.Bounds {
		if b = s.apply(b); !b.HasVars() && !s.c.subtype(types.IntType, b) && litsFit(tv.lits, b) {
			return b.NonNull()
		}
	}
	return types.IntType
}

// lub is the common supertype of two inferred bounds.
func (c *Checker) lub(a, b types.Type) types.Type {
	if t, ok := c.join(a, b); ok {
		return t
	}
	return joinNull(c.commonSuper(a.NonNull(), b.NonNull(), 0), a, b)
}

// finish completes the system, reports what could not be inferred and
// writes the results into the tree.
func (s *system) finish() {
	if r := s.c.joinRun(s); r != nil {
		if !slices.Contains(r.joined, s) {
			r.join(s)
		}
	} else {
		s.complete()
	}
	for _, e := range s.errs {
		s.c.errorf(e.pos, "%s", e.msg)
	}
	for _, f := range s.fixups {
		f(s)
	}
}

// final returns t with all variables replaced: those that could not be
// inferred become the error type. Those of a builder lambda's parameter
// types being inferred stay: see stubInference.
func (s *system) final(t types.Type) types.Type {
	t = s.apply(t)
	if t.HasVars() && !s.c.onlyStubs(t) {
		return types.InvalidType
	}
	return t
}

// withinUppers returns the least upper bound val of lowers, or, if it is not
// within uppers, an upper bound that all lowers are within, which stands
// for kotlinc's more precise common supertype: MyClass<out A & D> of
// MyClass<B> and MyClass<C>, for B and C both A and D, which kc's is
// MyClass<out A>, for an expected Set<MyClass<out D>>.
func (s *system) withinUppers(val types.Type, lowers, uppers []types.Type) types.Type {
	c := s.c
	fits := func(t types.Type) bool {
		for _, u := range uppers {
			if !c.subtype(t, u) {
				return false
			}
		}
		return true
	}
	if len(uppers) == 0 || fits(val) {
		return val
	}
	// Only where kc's is a less precise one: an intersection kc lacks,
	// at the top or as the type of a projection.
	shaped := func(u types.Type) bool {
		if val.A == nil || val.A.Len() == 0 {
			return true
		}
		if u.K != types.Class || u.C != val.C || u.A == nil || u.A.Len() != val.A.Len() {
			return false
		}
		for i, a := range val.A.List {
			if b := u.A.Get(i); a.V != b.V || a.V == types.Inv && !c.subtype(a.T, b.T) {
				return false
			}
		}
		return true
	}
	for _, u := range uppers {
		if !fits(u) || !shaped(u) {
			continue
		}
		ok := true
		for _, l := range lowers {
			if !c.subtype(l, u) {
				ok = false
				break
			}
		}
		if ok {
			return u
		}
	}
	return val
}

// finalArg is final for the type of a parameter that an argument is
// passed for: a variable fixed to a captured projection takes what can be
// written to the capture (see types.Subst.ApplyArg).
func (s *system) finalArg(t types.Type) types.Type {
	if t.HasVars() {
		t = s.vals.ApplyArg(t)
	}
	if t.HasVars() && !s.c.onlyStubs(t) {
		return types.InvalidType
	}
	return t
}

// fixVarsIn fixes the variables that t mentions, as a lambda needs before
// its parameter types are known. The variables their bounds mention are
// fixed first.
func (s *system) fixVarsIn(t types.Type) {
	seen := map[*tvar]bool{}
	var visit func(t types.Type, dep bool)
	visit = func(t types.Type, dep bool) {
		s.apply(t).Has(func(u types.Type) bool {
			if u.K != types.TypeVar {
				return false
			}
			tv := s.byVar[u.Param()]
			if tv == nil || tv.fixed || seen[tv] {
				return false
			}
			seen[tv] = true
			for _, b := range tv.lower {
				visit(b, true)
			}
			// A variable it is a lower bound of (T1 <: T2) is fixed first
			// only if T1 has no proper bound of its own: else T2's other
			// bounds would give T1 its value, Nothing from TODO() in
			// elvis(f { it.method() }, TODO()).
			// But a proper upper bound gives way to T2's proper lower one,
			// as kotlinc prefers fixing variables with lower bounds: VR of
			// fun <T, VR : T> foo(x: T, fn: (VR?, T) -> Unit) in foo(1) {
			// prev: Int?, new -> } is fixed after T, an Int.
			lower := len(tv.lits) > 0 || slices.ContainsFunc(tv.lower, s.proper)
			own := lower || slices.ContainsFunc(tv.upper, s.proper)
			for _, b := range tv.upper {
				if !own || !s.isVar(b) || !lower && s.hasLower(b) {
					visit(b, true)
				}
			}
			// A variable t doesn't mention waits for a bound if it has none
			// yet: box()'s T of List<T> in f(arrayOf("a"), box()) { } takes
			// f's T, which the lambda's Array<T> needs, as its bound.
			if !tv.fixed && (!dep || s.fixable(tv)) {
				s.fix(tv)
			}
			return false
		})
	}
	visit(t, false)
}

// hasLower reports whether t is an unfixed variable with a proper lower
// bound other than Nothing, which kotlinc deems trivial: TODO()'s.
func (s *system) hasLower(t types.Type) bool {
	if !s.isVar(t) {
		return false
	}
	tv := s.byVar[s.apply(t).Param()]
	return len(tv.lits) > 0 || slices.ContainsFunc(tv.lower, func(b types.Type) bool {
		return s.proper(b) && s.apply(b).K != types.Nothing
	})
}

// fixable reports whether fix finds a value for tv in its bounds.
func (s *system) fixable(tv *tvar) bool {
	if len(tv.lits) > 0 || tv.star || slices.ContainsFunc(tv.lower, s.proper) || slices.ContainsFunc(tv.upper, s.proper) {
		return true
	}
	stub := func(b types.Type) bool { return s.c.onlyStubs(s.apply(b)) }
	return slices.ContainsFunc(tv.lower, stub) || slices.ContainsFunc(tv.upper, stub)
}

// selfInferable reports whether completing the system on its own infers
// every variable of t without errors, as it does for maxOf(1, 2) but not
// for listOf(), which needs an expected type.
func (s *system) selfInferable(t types.Type) bool {
	if !s.ok {
		return false
	}
	trial := s.clone()
	trial.complete()
	return trial.ok && len(trial.errs) == 0 && trial.proper(t)
}

// isVar reports whether t is an unfixed variable of s, or its nullable.
func (s *system) isVar(t types.Type) bool {
	t = s.apply(t)
	if t.K != types.TypeVar {
		return false
	}
	tv := s.byVar[t.Param()]
	return tv != nil && !tv.fixed
}

// owns reports whether the variables in t are all variables of s.
func (s *system) owns(t types.Type) bool {
	return !t.Has(func(u types.Type) bool { return u.K == types.TypeVar && s.byVar[u.Param()] == nil })
}

// known reports whether each unfixed variable in t has a known bound, so
// that it can be fixed.
func (s *system) known(t types.Type) bool {
	return !s.apply(t).Has(func(u types.Type) bool {
		if u.K != types.TypeVar {
			return false
		}
		tv := s.byVar[u.Param()]
		if tv == nil || tv.fixed || len(tv.lits) > 0 {
			return false
		}
		for _, b := range append(tv.lower[:len(tv.lower):len(tv.lower)], tv.upper...) {
			if s.proper(b) {
				return false
			}
		}
		return true
	})
}

// fixFuncInputs fixes the variables in the parameter types of variable v's
// function-typed upper bounds, which a lambda passed for v needs: R of
// resumeImpl's (Throwable, R, CoroutineContext) -> Unit.
func (s *system) fixFuncInputs(v types.Type) {
	tv := s.byVar[v.Param()]
	if tv == nil {
		return
	}
	for _, u := range tv.upper {
		ps, _, ok := funcParts(s.apply(u).NonNull())
		if !ok {
			continue
		}
		for _, p := range ps {
			if s.known(p) {
				s.fixVarsIn(p)
			}
		}
	}
}

// upperHint returns a known upper bound of variable v, which a lambda's
// body can use as its expected type: Long in val x: Long = run { 1 }.
func (s *system) upperHint(v types.Type) types.Type {
	if tv := s.byVar[v.Param()]; tv != nil {
		for _, u := range tv.upper {
			if u = s.apply(u); !u.HasVars() {
				return u
			}
		}
	}
	return noWant
}

// unitBound reports whether v is an unfixed variable, not nullable, that
// a declared bound makes a Unit: as in Kotlin, a lambda returning it is
// coerced to Unit, as build { c -> s } for fun <BK : Unit> build(t:
// (Controller<BK>) -> BK).
func (s *system) unitBound(v types.Type) bool {
	tv := s.unfixedVar(v)
	if tv == nil || v.N {
		return false
	}
	return slices.ContainsFunc(tv.v.Bounds, func(b types.Type) bool { b = s.apply(b); return b.K == types.Unit && !b.N })
}

// boundExcludes reports whether t is an unfixed variable whose declared
// bounds u can't be within.
func (s *system) boundExcludes(t, u types.Type) bool {
	tv := s.unfixedVar(t)
	if tv == nil {
		return false
	}
	for _, b := range tv.v.Bounds {
		if b = s.apply(b); !b.HasVars() && !s.c.subtype(u, b) {
			return true
		}
	}
	return false
}

// withinBounds reports whether the proper lower constraints of each
// unfixed variable are within its declared bounds that are proper.
func (s *system) withinBounds() bool {
	for _, tv := range s.vars {
		if tv.fixed {
			continue
		}
		for _, l := range tv.lower {
			if l = s.apply(l); l.HasVars() || l.K == types.Invalid {
				continue
			}
			for _, b := range tv.v.Bounds {
				if b = s.apply(b); !b.HasVars() && !s.c.subtype(l, b) {
					return false
				}
			}
		}
	}
	return true
}

// captured reports whether a is a variable inferred as a projection's
// capture, which K2 doesn't allow as a reified type argument.
func (s *system) captured(a types.Arg) bool {
	if a.T.K != types.TypeVar {
		return false
	}
	v := s.vals[a.T.Param()]
	return v.V == types.Out || v.V == types.In // a star is a variable's own
}

// selfBounded reports whether class c's i-th type parameter occurs in
// its own bounds, as E of Enum<E : Enum<E>>.
func selfBounded(c *types.ClassSym, i int) bool {
	if i >= len(c.TypeParams) {
		return false
	}
	p := c.TypeParams[i]
	return slices.ContainsFunc(p.Bounds, func(b types.Type) bool {
		return b.Has(func(u types.Type) bool { return u.IsParam() && u.Param() == p })
	})
}
