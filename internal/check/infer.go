package check

import (
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
	// in <C : Iterable<T>> gives T from C's argument, as in onEach.
	for i := range ps {
		for _, b := range vs[i].Bounds {
			other := b.Has(func(u types.Type) bool {
				return u.K == types.TypeVar && u.Param() != vs[i] && s.byVar[u.Param()] != nil
			})
			if other {
				s.addUpper(s.byVar[vs[i]], b)
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
		lambdas: append([]pendingLambda{}, s.lambdas...)}
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
	s.ok = s.ok && o.ok
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
		if dnn {
			b = b.Nullable() // V & Any <: b gives V <: b?
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
			if ta.V == types.Star || ta.V == types.Out {
				s.sub(ba.T, types.NothingType)
			} else {
				s.sub(ba.T, ta.T)
			}
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
			return
		}
	}
	tv.upper = append(tv.upper, t)
	for _, l := range tv.lower {
		s.sub(l, t)
	}
	// Below another variable, it is below that one's declared bound too,
	// as in Kotlin: X of f<X>(g<Y : Any>()) for f's G<X> and g's G<Y> is
	// Any.
	if w := s.unfixedVar(t); w != nil && w != tv {
		for _, b := range w.v.Bounds {
			if b = s.apply(b); !b.HasVars() && !(b.K == types.Any && b.N) {
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
	for {
		var pick *tvar
		best := -2
		for _, tv := range s.vars {
			if tv.fixed {
				continue
			}
			if len(tv.lower) == 0 && len(tv.upper) == 0 && len(tv.lits) == 0 {
				// Without bounds, last: fixing the others may give it
				// some, as T's value Map<String, Int> does K and V in
				// stateIn(initialValue = emptyMap()) on a Flow<Map<..>>.
				if best < -1 {
					pick, best = tv, -1
				}
				continue
			}
			// Prefer variables whose bounds are all known, then those with
			// a known lower bound.
			score := 0
			allProper := true
			for _, l := range tv.lower {
				if s.proper(l) {
					score = 2
				} else {
					allProper = false
				}
			}
			// An upper bound that is another variable, as T2 in T1 <: T2,
			// is that variable's lower bound too, which is fixed first.
			varUppers := 0
			for _, u := range tv.upper {
				switch {
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
			if allProper && (varUppers == 0 || len(tv.upper) > varUppers || len(tv.lower) > 0 || len(tv.lits) > 0) {
				score += 3
			}
			if score > best {
				pick, best = tv, score
			}
		}
		if pick == nil {
			return
		}
		s.fix(pick)
	}
}

// fix chooses a value for a variable from its known bounds.
func (s *system) fix(tv *tvar) {
	c := s.c
	var lowers, uppers []types.Type
	var stubs []types.Type
	for _, l := range tv.lower {
		if l = s.apply(l); !l.HasVars() {
			lowers = append(lowers, l)
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
	val := types.InvalidType
	captured := false
	switch {
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
		val = s.litType(tv, uppers)
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
			}
		}
		// Bounds that none of the others is below give their
		// intersection: E <: Int and E <: String, of In<E> for an
		// In<Int> and an In<String>, give Int & String, of which kotlinc
		// only warns that it is empty. Where the variable has declared
		// bounds, an empty one stays an error, as S <: Long? and S <:
		// Double? of S : T? are (see the bounds below).
		comps := []types.Type{val}
		for _, u := range uppers {
			if !c.subtype(val, u) && intersectable(u) && intersectable(val) &&
				(len(tv.v.Bounds) == 0 || !c.emptyIntersection(val, u)) {
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
				if b = s.apply(b); !b.HasVars() && !types.AcceptsNull(b) {
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
	default:
		s.errs = append(s.errs, inferError{tv.pos, "cannot infer type for type parameter '" + tv.v.Name + "'. Specify it explicitly."})
	}
	// A variable with only upper constraints is their intersection with
	// its declared bounds, as K2 infers it. kc has no intersection types:
	// it takes a bound below the constraints, and otherwise keeps them,
	// unless the intersection is empty: S of fun <S : Out<B>> for
	// Foo<in S> and a Foo<K> of K : Out<L> is K & Out<B>.
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
			case !c.emptyIntersection(val, b):
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
	if tv.capt.V != types.Inv {
		defer func() { s.vals[tv.v] = tv.capt }()
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
	t := c.commonSuper(a.NonNull(), b.NonNull(), 0)
	if types.MayBeNull(a) || types.MayBeNull(b) {
		t = t.Nullable()
	}
	return t
}

// finish completes the system, reports what could not be inferred and
// writes the results into the tree.
func (s *system) finish() {
	if r := s.c.joinRun(s); r != nil {
		r.join(s)
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
