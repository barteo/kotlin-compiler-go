package types

import (
	"slices"
	"strings"
	"sync"
	"sync/atomic"
)

// Variance is the variance of a type parameter or of a type argument.
type Variance uint8

const (
	Inv  Variance = iota
	Out           // out T, or ? extends T in Java
	In            // in T, or ? super T in Java
	Star          // *, or ? in Java
)

func (v Variance) String() string {
	switch v {
	case Out:
		return "out"
	case In:
		return "in"
	case Star:
		return "*"
	}
	return ""
}

// Arg is a type argument: a type with a use-site projection. T is unused
// for Star.
type Arg struct {
	V Variance
	T Type
}

func (a Arg) String() string {
	switch a.V {
	case Star:
		return "*"
	case Out, In:
		return a.V.String() + " " + a.T.String()
	}
	return a.T.String()
}

// Args is an interned list of type arguments. Equal lists share one
// *Args, so types that carry arguments can still be compared with ==.
type Args struct {
	List  []Arg
	param *TypeParamSym // set instead of List in a parameter's own Args
	// local is set if the list holds a symbol of one compilation: see
	// MakeArgs.
	local bool
	// lists holds the interned lists whose table is this one's: see
	// MakeArgs.
	lists argsTable
}

// A list of arguments is its first argument, the head, before the list of
// the others, the tail (nil for a list of one).
type argsKey struct {
	head Arg
	tail *Args
}

// Interning. Equal lists must share one *Args for as long as either is in
// use, but a table of every list that lives as long as the process keeps
// every compilation's symbols after it is done, and through their loaders
// the classes and metadata it read: a process that compiles many times,
// as the tests do, grows without bound.
//
// So a list is kept in a table that belongs to one of its parts, which
// the list can't outlive: its tail, its head type's arguments and its
// head type's class. The table is that of the first of these that is
// local, a symbol of one compilation or a list holding one. Equal lists
// have the same parts, so they are found in the same table, and a list is
// collected with the compilation it belongs to. Symbols are local unless
// they are among the few that package-level variables declare for all
// compilations (see ClassSym.Global and TypeParamSym.Global). Lists of
// only those and of types without symbols, such as Int, are in a table
// for the process, globalArgs.

// argsTable interns lists of arguments: argsKey -> *Args. Checking in
// parallel looks lists up far more often than it adds them, which
// sync.Map does without locking. It is allocated on first use: most
// symbols and lists hold no table.
type argsTable struct{ m atomic.Pointer[sync.Map] }

func (t *argsTable) get() *sync.Map {
	if m := t.m.Load(); m != nil {
		return m
	}
	t.m.CompareAndSwap(nil, new(sync.Map))
	return t.m.Load()
}

// globalArgs interns the lists that hold no local symbols.
var globalArgs sync.Map

func (a *Args) isLocal() bool { return a != nil && a.local }

// table returns the table that interns the list k.
func (k *argsKey) table() *sync.Map {
	switch {
	case k.tail.isLocal():
		return k.tail.lists.get()
	case k.head.T.A.isLocal():
		return k.head.T.A.lists.get()
	case k.head.T.C != nil && !k.head.T.C.Global:
		return k.head.T.C.lists.get()
	}
	return &globalArgs
}

// MakeArgs returns the interned list for args, or nil if it is empty.
func MakeArgs(args []Arg) *Args {
	var tail *Args
	for i := len(args) - 1; i >= 0; i-- {
		a := args[i]
		if a.V == Star {
			a.T = Type{}
		}
		k := argsKey{a, tail}
		tab := k.table()
		n, ok := tab.Load(k)
		if !ok {
			list := make([]Arg, 0, len(args)-i)
			list = append(list, a)
			if tail != nil {
				list = append(list, tail.List...)
			}
			n, _ = tab.LoadOrStore(k, &Args{List: list, local: tab != &globalArgs})
		}
		tail = n.(*Args)
	}
	return tail
}

// Len returns the number of arguments; a nil list has none.
func (a *Args) Len() int {
	if a == nil {
		return 0
	}
	return len(a.List)
}

// Get returns the i-th argument.
func (a *Args) Get(i int) Arg { return a.List[i] }

// TypeParamSym is a type parameter of a class or function. Inference
// creates a copy of each parameter of a call's callee as a type variable,
// with Origin pointing to the parameter.
type TypeParamSym struct {
	Name   string
	Var    Variance // declaration-site variance: Inv, Out or In
	Bounds []Type   // upper bounds; none means Any?
	Index  int
	Owner  any // *ClassSym or *FunSym
	Origin *TypeParamSym
	// Reified is set on a reified type parameter of an inline function,
	// which a copy of its body knows.
	Reified bool
	// OnlyInput is set on a parameter annotated @kotlin.internal.OnlyInputTypes,
	// as T of contains(element: T): what a call infers for it must be the
	// type of one of its inputs.
	OnlyInput bool
	// Global is set on a parameter of a symbol that a package-level
	// variable declares for all compilations in the process, such as
	// kotlin.Array's T: see MakeArgs.
	Global bool

	once sync.Once
	self *Args // what types of this parameter point to
}

// Redeclare makes p the type parameter q declares, keeping p, which the
// types of p refer to: see check.trial.
func (p *TypeParamSym) Redeclare(q *TypeParamSym) {
	p.Name, p.Var, p.Bounds, p.Index, p.Owner, p.Origin, p.Reified = q.Name, q.Var, q.Bounds, q.Index, q.Owner, q.Origin, q.Reified
	p.OnlyInput = q.OnlyInput
}

// Param returns the parameter or variable of a TypeParam or TypeVar type.
func (t Type) Param() *TypeParamSym { return t.A.param }

func (p *TypeParamSym) args() *Args {
	p.once.Do(func() { p.self = &Args{param: p, local: !p.Global} })
	return p.self
}

// NullableBound reports whether a value of type T can be null: true unless
// one of its bounds is non-null.
func (p *TypeParamSym) NullableBound() bool {
	for _, b := range p.Bounds {
		if !MayBeNull(b) {
			return false
		}
	}
	return true
}

// ErasureBound returns the bound that T erases to, as with kotlinc: the
// first that is a class, not an interface, or else the first.
func (p *TypeParamSym) ErasureBound() Type {
	if len(p.Bounds) > 1 {
		for _, b := range p.Bounds {
			if b.K == Any {
				return b // kotlin.Any is a class
			}
			if b.K == Class {
				b.C.Ensure()
				if !b.C.Interface {
					return b
				}
			}
		}
	}
	return p.Bound()
}

// Bound returns the first upper bound, or Any? if there is none.
func (p *TypeParamSym) Bound() Type {
	if len(p.Bounds) == 0 {
		return AnyType.Nullable()
	}
	return p.Bounds[0]
}

// ParamType returns the type T for a type parameter T.
func ParamType(p *TypeParamSym) Type { return Type{K: TypeParam, A: p.args()} }

// VarType returns the type of an inference variable.
func VarType(p *TypeParamSym) Type { return Type{K: TypeVar, A: p.args()} }

// GenericType returns c with the given type arguments.
func GenericType(c *ClassSym, args []Arg) Type {
	return Type{K: Class, C: c, A: MakeArgs(args)}
}

// IsParam reports whether t is a type parameter or an inference variable.
func (t Type) IsParam() bool { return t.K == TypeParam || t.K == TypeVar }

// MayBeNull reports whether a value of type t can be null. A type
// parameter can be null unless it is bounded by a non-null type.
func MayBeNull(t Type) bool {
	if t.N {
		return true
	}
	return t.IsParam() && !t.D && t.Param().NullableBound()
}

// AcceptsNull reports whether null can be assigned to type t: a nullable
// or platform type, but not a type parameter such as T, which might stand
// for a non-null type.
func AcceptsNull(t Type) bool { return t.N || t.K == Invalid }

// Erase returns the type t has on the JVM: a type parameter becomes its
// first bound, or kotlin.Unit's class for Unit, and type arguments are
// dropped.
func Erase(t Type) Type {
	switch t.K {
	case TypeParam, TypeVar:
		b := Erase(t.Param().ErasureBound())
		if b.K == Unit {
			b = ClassType(unitObject) // R : Unit holds the kotlin.Unit object
		}
		if IsValue(b) {
			b = Boxed(b) // a type parameter's values are boxed
		}
		return b.Nullable()
	case Class:
		if t.C == ArrayClass && t.A != nil {
			// An array keeps its element type: its erasure is a JVM array.
			elem := AnyType.Nullable()
			if a := t.A.Get(0); a.V == Inv || a.V == Out {
				elem = Erase(a.T)
			}
			t.A = MakeArgs([]Arg{{T: elem}})
			break
		}
		t.A = nil
	}
	return t
}

// Has reports whether f holds for t or any type nested in its arguments.
func (t Type) Has(f func(Type) bool) bool {
	if f(t) {
		return true
	}
	if t.A != nil {
		for _, a := range t.A.List {
			if a.V != Star && a.T.Has(f) {
				return true
			}
		}
	}
	return false
}

// HasVars reports whether t mentions an inference variable.
func (t Type) HasVars() bool {
	if t.K != TypeVar && t.A == nil {
		return false
	}
	return t.Has(func(u Type) bool { return u.K == TypeVar })
}

// HasParams reports whether t mentions a type parameter.
func (t Type) HasParams() bool {
	if t.K != TypeParam && t.A == nil {
		return false
	}
	return t.Has(func(u Type) bool { return u.K == TypeParam })
}

// Subst maps type parameters (or inference variables) to type arguments.
type Subst map[*TypeParamSym]Arg

// Apply substitutes s in a type read from a declaration, such as a return
// type or a property's type (an out position).
func (s Subst) Apply(t Type) Type { return s.apply(t, Out) }

// ApplyIn substitutes s in a type written to, such as a parameter's type
// (an in position). Projections there give Nothing: nothing can be passed
// to Box<out T>.set(T).
func (s Subst) ApplyIn(t Type) Type { return s.apply(t, In) }

// ApplyArg substitutes s in the type of a parameter that an argument is
// passed for, which takes a value written to it as ApplyIn does, except
// where a parameter is an invariant argument of the type itself: there
// the projection replacing it is the argument's own captured type, so
// Array<T> with T := out Int takes the Array<out Int> it was captured
// from, but Array<Array<T>> no Array<Array<out Int>>.
func (s Subst) ApplyArg(t Type) Type {
	if len(s) == 0 || t.K != Class || t.A == nil {
		return s.apply(t, In)
	}
	r := s.apply(t, In)
	if r.K != Class || r.A == nil || r.A.Len() != t.A.Len() {
		return r
	}
	var args []Arg
	for i, a := range t.A.List {
		if a.V != Inv || !a.T.IsParam() {
			continue
		}
		if c, ok := s[a.T.Param()]; ok && (c.V == Out || c.V == In) {
			if args == nil {
				args = slices.Clone(r.A.List)
			}
			args[i] = Arg{c.V, withFlags(c.T, a.T)}
		}
	}
	if args != nil {
		r.A = MakeArgs(args)
	}
	return r
}

func (s Subst) apply(t Type, pos Variance) Type {
	t, _ = s.applyA(t, pos, false)
	return t
}

// ApplyExact substitutes without approximating projections, as a type
// alias's arguments are: Array2D<out Number> for Array<Array<T>> is
// Array<Array<out Number>>.
func (s Subst) ApplyExact(t Type) Type {
	t, _ = s.applyA(t, Out, true)
	return t
}

// applyA is apply, also reporting whether it approximated a captured
// projection, a parameter replaced by an out or in argument: in a type
// read, an invariant argument that holds one is then out projected, as
// Kotlin approximates Array<Array<C>> for C, the capture of out Int, by
// Array<out Array<out Int>>.
func (s Subst) applyA(t Type, pos Variance, exact bool) (Type, bool) {
	if len(s) == 0 {
		return t, false
	}
	switch t.K {
	case TypeParam, TypeVar:
		a, ok := s[t.Param()]
		if !ok {
			return t, false
		}
		var r Type
		switch {
		case a.V == Inv:
			r = a.T
		case pos == Out && a.V == Out:
			r = a.T
		case pos == In && a.V == In:
			r = a.T
		case pos == In:
			return withFlags(NothingType, t), true // T? takes null
		case a.V == Star && pos == Out && recursiveStar(t.Param()):
			// A recursive bound is approximated with a star for the
			// parameter, as kotlinc does: T of A<*> for A<T : A<T?>?> is
			// read as A<*>?, and of A<*, Int> for A<T : B<T, U>, U> as
			// B<*, Int>.
			p := t.Param()
			r = s.apply(starred(p.Bounds[0], map[*TypeParamSym]bool{p: true}), Out)
		default: // reading through in or *: the parameter's bound
			r = s.apply(t.Param().Bound(), Out)
		}
		return withFlags(r, t), a.V == Out || a.V == In
	case Class:
		if t.A == nil {
			return t, false
		}
		args := make([]Arg, len(t.A.List))
		changed, approx := false, false
		for i, a := range t.A.List {
			v := a.V
			if v == Inv && (pos == In || !a.T.IsParam() && a.T.A != nil || s.projected(a.T)) {
				v = declaredVar(t.C, i)
			}
			var ax bool
			args[i], ax = s.applyArgA(a, pos, v, exact)
			if ax && a.V == Inv && !exact {
				switch dv := declaredVar(t.C, i); {
				case args[i].V == Inv && dv == Inv && pos == Out:
					args[i].V = Out
				case args[i].V == dv:
					args[i].V = Inv // List<out X> is List<X>
				}
			}
			approx = approx || ax
			if pos == In && args[i].V == Star && v == Out {
				// Written to through a star projection, a covariant
				// argument is Kotlin's captured type, which only Nothing
				// is a subtype of: Continuation<*>.resumeWith takes a
				// Result<Nothing>.
				args[i] = Arg{Out, withFlags(NothingType, a.T)}
			}
			changed = changed || args[i] != a
		}
		if !changed {
			return t, approx
		}
		t.A = MakeArgs(args)
		return t, approx
	}
	return t, false
}

// projected reports whether t is a parameter that s replaces by an out or
// in projection.
func (s Subst) projected(t Type) bool {
	if !t.IsParam() {
		return false
	}
	r, ok := s[t.Param()]
	return ok && (r.V == Out || r.V == In)
}

// declaredVar returns the declaration-site variance of class c's i-th type
// parameter.
func declaredVar(c *ClassSym, i int) Variance {
	c.Ensure()
	if i < len(c.TypeParams) {
		return c.TypeParams[i].Var
	}
	return Inv
}

// applyArg substitutes inside a type argument, of a type in position
// pos, whose variance with the declaration's is v. A projection that
// replaces an invariant occurrence becomes the argument's projection: T
// in Box<T> with T := out X gives Box<out X>. Inside an in argument the
// position is the opposite: the Res<E> of a Cont<in T> read as a
// Cont<Res<E>> is written to.
// In a type written to, as a parameter's, an out projection is a type
// only Nothing is a subtype of, as Kotlin's captured type is: nothing but
// Box<Nothing> can be passed for a Box<T> or a Box<out T> then.
func (s Subst) applyArgA(a Arg, pos, v Variance, exact bool) (Arg, bool) {
	if a.V == Star {
		return a, false
	}
	if a.T.IsParam() {
		if r, ok := s[a.T.Param()]; ok {
			if a.V == Inv && v != Inv && pos == In && (r.V == Out || r.V == In) {
				// A capture in an argument of declared variance of a type
				// written to is approximated as a type: for C, the capture
				// of out Number, a parameter (C) -> Unit takes a
				// (Number) -> Unit, as a lambda sees it. Read, the
				// argument stays a projection.
				if v == In {
					pos = Out + In - pos
				}
				t, _ := s.applyA(a.T, pos, exact)
				return Arg{T: t}, true
			}
			switch {
			case r.V == Star && pos == Out && a.V != In:
				// Read through a star projection, the capture is within
				// the parameter's bound, as kotlinc approximates it:
				// through a Base<*> of Base<T : Base<T>>, a List<T> is a
				// List<out Base<*>>.
				if b, ok := starBound(a.T.Param()); ok {
					return Arg{Out, withFlags(b, a.T)}, r.V == Out || r.V == In
				}
				if p := a.T.Param(); a.T.D && len(p.Bounds) <= 1 {
					// T & Any is within the bound made not null.
					b := AnyType
					if len(p.Bounds) == 1 {
						b = p.Bounds[0].NonNull()
					}
					return Arg{Out, b}, true
				}
				if p := a.T.Param(); len(p.Bounds) > 1 {
					// Kotlin's is the intersection of its bounds, as
					// is the parameter itself: E of Exp<*> for an
					// Exp<E> where E : Enum<E>, E : Variant.
					return Arg{Out, a.T}, r.V == Out || r.V == In
				}
				return r, true
			case r.V == Star:
				// Read, a star in an invariant argument is a capture
				// approximated, which makes the argument out: T of
				// B<*, *> for B<R, T : B<List<R>, T>> is B<out List<*>, *>.
				return r, pos == Out
			case r.V == Inv:
				return Arg{a.V, withFlags(r.T, a.T)}, r.V == Out || r.V == In
			case pos == In && r.V == Out && a.V != In:
				return Arg{Out, withFlags(NothingType, a.T)}, r.V == Out || r.V == In
			case a.V == Inv || a.V == r.V:
				return Arg{r.V, withFlags(r.T, a.T)}, r.V == Out || r.V == In
			case pos == In:
				// Written to, an argument projected the other way than
				// its capture is within the projection's type: Optional<out
				// E>.map takes a Function<in E, …>.
				return Arg{a.V, withFlags(r.T, a.T)}, r.V == Out || r.V == In
			}
			return Arg{V: Star}, false
		}
		return a, false
	}
	if v == In {
		pos = Out + In - pos
	}
	t, approx := s.applyA(a.T, pos, exact)
	return Arg{a.V, t}, approx
}

// recursiveStar reports whether p has a single class bound that p occurs in.
func recursiveStar(p *TypeParamSym) bool {
	return len(p.Bounds) == 1 && p.Bounds[0].K == Class && hasParam(p.Bounds[0], p)
}

// hasParam reports whether p occurs in t.
func hasParam(t Type, p *TypeParamSym) bool {
	if t.IsParam() {
		return t.Param() == p
	}
	if t.A != nil {
		for _, a := range t.A.List {
			if a.V != Star && hasParam(a.T, p) {
				return true
			}
		}
	}
	return false
}

// starBound returns the bound of type parameter p that a star projection
// for it stands for, with stars for its class's type parameters, if it is
// a class type.
func starBound(p *TypeParamSym) (Type, bool) {
	if len(p.Bounds) != 1 || p.Bounds[0].K != Class {
		return Type{}, false
	}
	ps := map[*TypeParamSym]bool{p: true}
	if c, ok := p.Owner.(*ClassSym); ok {
		for _, q := range c.TypeParams {
			ps[q] = true
		}
	}
	return starred(p.Bounds[0], ps), true
}

// starred replaces the arguments of t that are the type parameters ps by
// star projections.
func starred(t Type, ps map[*TypeParamSym]bool) Type {
	if t.K != Class || t.A == nil {
		return t
	}
	args := make([]Arg, t.A.Len())
	for i, a := range t.A.List {
		switch {
		case a.V == Star:
			args[i] = a
		case a.T.IsParam() && ps[a.T.Param()]:
			args[i] = Arg{V: Star}
		default:
			args[i] = Arg{a.V, starred(a.T, ps)}
		}
	}
	t.A = MakeArgs(args)
	return t
}

// withFlags applies the nullability written on a type parameter's
// occurrence (T?, T & Any, or a Java T!) to the type that replaces it.
func withFlags(r, occ Type) Type {
	switch {
	case r.K == Invalid:
		return r
	case occ.N && !occ.P:
		return r.Nullable()
	case occ.D:
		return r.NonNull()
	case occ.P:
		if r.N || r.K == Unit || r.K == Nothing {
			return r
		}
		r.N, r.P, r.D = true, true, false
	}
	return r
}

// TypeParamsString formats type parameters as in a declaration: <K, V>.
func TypeParamsString(ps []*TypeParamSym) string {
	if len(ps) == 0 {
		return ""
	}
	names := make([]string, len(ps))
	for i, p := range ps {
		names[i] = p.Name
	}
	return "<" + strings.Join(names, ", ") + ">"
}
