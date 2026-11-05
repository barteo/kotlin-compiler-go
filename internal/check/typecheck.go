package check

import (
	"slices"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// This file checks type checks and casts, x is T and x as T, and the smart
// casts that follow from them.

func (c *Checker) isExpr(x *syntax.Is) types.Type {
	st := c.value(x.X, noWant)
	declared := types.InvalidType
	if v := c.narrowSym(x.X); v != nil {
		declared = v.Type
	}
	x.Target = c.checkTarget(x.Ref, st, declared, true)
	return types.BoolType
}

func (c *Checker) asExpr(x *syntax.As) types.Type {
	st := c.value(x.X, noWant)
	t := c.checkTarget(x.Ref, st, types.InvalidType, false)
	x.Target = t
	if t.K == types.Invalid || st.K == types.Invalid {
		return types.InvalidType
	}
	if x.Safe {
		return t.Nullable()
	}
	if t.K == types.Class && t.C.Name == "kotlin/Unit" && !t.N {
		return types.UnitType // x as Unit, a cast to kotlin.Unit
	}
	return t
}

// checkTarget resolves the type T of x is T or x as T, where x has type
// subject. On the JVM, type arguments are erased: is can only check them
// when the subject's type determines them, or the type x is declared with
// if a smart cast has changed it, which kc, without intersection types,
// no longer sees in subject; as doesn't check them.
func (c *Checker) checkTarget(ref *syntax.TypeRef, subject, declared types.Type, is bool) types.Type {
	c.bareRef, c.bareSubject = ref, subject
	t := c.resolveType(ref)
	c.bareRef = nil
	switch {
	case t.K == types.Invalid:
		return t
	case t.K == types.Unit:
		// The kotlin.Unit object's class.
		if u := c.u.ext.class("kotlin/Unit"); u != nil {
			return types.ClassType(u)
		}
		c.errorf(ref.P, "cannot check for an instance of %s", t)
		return types.InvalidType
	case t.K == types.Range:
		c.errorf(ref.P, "cannot check for an instance of %s", t)
		return types.InvalidType
	case t.IsParam():
		if is && !t.Param().Reified {
			c.errorf(ref.P, "cannot check for instance of erased type: %s", t)
			return types.InvalidType
		}
		return t
	case t.K != types.Class:
		return t
	case t.C.Func && t.C.Composable:
		c.unsupported(ref.P, "compose", "is and as with composable function types are not supported")
		return types.InvalidType
	}
	if is && t.A != nil && !allStar(t) && !c.argsKnown(t, subject, ref) && (declared.K == types.Invalid || !c.argsKnown(t, declared, ref)) {
		c.errorf(ref.P, "cannot check for instance of erased type: %s", t)
		return types.InvalidType
	}
	if is && t.A != nil && allStar(t) {
		// x is C<*> narrows x to C<*> and its own type, whose arguments
		// it has where they follow from it: a Flow<T> that is a
		// CancellableFlow<*> is a CancellableFlow<T>. Not if they are
		// outside C's bounds, as a DeserializationStrategy<T> that is an
		// AbstractPolymorphicSerializer<*>, whose T : Any.
		if d := c.bareType(t.C, ref, subject); c.argsInBounds(d) {
			return d
		}
	}
	return t
}

// argsInBounds reports whether the type arguments of class type t are
// within the bounds of its class's type parameters.
func (c *Checker) argsInBounds(t types.Type) bool {
	if t.K != types.Class || t.A == nil {
		return true
	}
	ps := t.C.TypeParams
	s := classSubst(t)
	for i, a := range t.A.List {
		if a.V == types.Star || i >= len(ps) {
			continue
		}
		for _, b := range ps[i].Bounds {
			if !c.subtype(a.T, s.Apply(b)) {
				return false
			}
		}
	}
	return true
}

// bareType returns the type of class cls, written without its type
// arguments in x is C or x as C, where x has type subject: as in Kotlin,
// with the arguments that follow from the subject's type, x is List for
// an x: Collection<String> giving List<String>, and * for the others.
func (c *Checker) bareType(cls *types.ClassSym, ref *syntax.TypeRef, subject types.Type) types.Type {
	args := make([]types.Arg, len(cls.TypeParams))
	for i := range args {
		args[i] = types.Arg{V: types.Star}
	}
	if s := subject.NonNull(); s.K == types.Class {
		if sup, ok := c.asSuper(s, cls); ok {
			return c.nullable(sup, ref) // a supertype of the subject's
		}
		if sup, ok := c.asSuper(cls.This(), s.C); ok && sup.A != nil && s.A != nil {
			for i, a := range sup.A.List {
				if a.V != types.Inv || !a.T.IsParam() || i >= s.A.Len() {
					continue
				}
				for j, p := range cls.TypeParams {
					if a.T.Param() == p {
						args[j] = s.A.Get(i)
					}
				}
			}
		}
	}
	return c.nullable(types.GenericType(cls, args), ref)
}

func allStar(t types.Type) bool {
	for _, a := range t.A.List {
		if a.V != types.Star {
			return false
		}
	}
	return true
}

// argsKnown reports whether the type arguments of t, written as ref,
// follow from the subject's type: a Collection<String> that is a List is
// a List<String>. As with kotlinc, they do if the type with the arguments
// that follow from it is a subtype of t, and of the subject's type: a
// KFunction1<Int, Int> is a KFunction1<Int, Any?>, as a suspend (Int) ->
// Int that is a KSuspendFunction1 is a KSuspendFunction1<Int, Any?>.
func (c *Checker) argsKnown(t, subject types.Type, ref *syntax.TypeRef) bool {
	s := subject.NonNull()
	if c.subtype(s, t) {
		return true
	}
	if s.K != types.Class {
		return false
	}
	if sup, ok := c.asSuper(t, s.C); ok && c.sameType(sup, s) {
		return true
	}
	d := c.bareType(t.C, ref, subject)
	if hasStar(d) || !c.subtype(d, t) {
		return false
	}
	sup, ok := c.asSuper(d, s.C) // not if the subject's arguments conflict, as Base<D, B> for a Derived<S> : Base<S, S>
	return ok && c.subtype(sup, s)
}

func hasStar(t types.Type) bool {
	return t.A != nil && slices.ContainsFunc(t.A.List, func(a types.Arg) bool { return a.V == types.Star })
}

// stable reports whether a variable's value can't change between a check
// and a use, so smart casts apply: a val local or parameter, or a val
// property of the program without a getter, read through this, or a
// top-level val without a getter.
func (c *Checker) stable(v *types.VarSym) bool {
	if v.Mutable {
		return isVar(v) && c.varStable(v, types.Type{})
	}
	switch v.Kind {
	case types.Local, types.Param, types.BackingField:
		return true
	case types.Prop:
		return v.Final && v.Getter == nil && v.HasField() && v.Owner != nil && !v.Owner.External
	case types.Global:
		return v.Getter == nil
	}
	return false
}

// narrowSym returns the variable that a smart cast of x narrows: a stable
// variable, or this. It returns nil if x can't be smart cast.
func (c *Checker) narrowSym(x syntax.Expr) *types.VarSym {
	switch x := x.(type) {
	case *syntax.Ident:
		if x.Sym != nil && x.Member == nil && c.stable(x.Sym) {
			return x.Sym
		}
	case *syntax.This:
		if x.Recv != nil {
			return x.Recv // a lambda's receiver, a parameter
		}
		return c.this
	case *syntax.Member:
		// a.b.c is stable if a.b is: a stable property of a val.
		if x.Safe || x.Prop == nil || !c.stableMember(x.Prop) {
			return nil
		}
		if r := c.narrowSym(x.X); r != nil && !r.Mutable {
			k := [2]*types.VarSym{r, x.Prop}
			if c.paths == nil {
				c.paths = map[[2]*types.VarSym]*types.VarSym{}
			}
			if p := c.paths[k]; p != nil {
				return p
			}
			p := &types.VarSym{Name: r.Name + "." + x.Prop.Name, Type: x.Prop.Type, Kind: types.Prop}
			c.paths[k] = p
			return p
		}
	}
	return nil
}

// isNarrowed returns the type that a value of type cur has once it is
// known to be a T. kc has no intersection types: if cur isn't a subtype
// of T, the value is seen as a T, non-null if cur is: ktor's first !is T
// for a first: Node and a reified T leaves a T & Any (see declaredIfNull).
func (c *Checker) isNarrowed(cur, t types.Type) types.Type {
	if c.subtype(cur.NonNull(), t) {
		if types.MayBeNull(t) {
			return cur
		}
		return cur.NonNull()
	}
	if t.IsParam() && !types.MayBeNull(cur) {
		return t.NonNull()
	}
	return t
}

// needsCast reports whether a value read as type from needs a cast to be
// used as type to: whether a smart cast changes its class, beyond making
// it non-null.
func needsCast(from, to types.Type) bool {
	d1, ok1 := types.Desc(from.Nullable())
	d2, ok2 := types.Desc(to.Nullable())
	return ok1 && ok2 && d1 != d2 && to.K != types.Nothing && to.K != types.Null
}

// newThis returns the symbol that smart casts of this in a class's code
// narrow, or nil outside classes.
func newThis(cls *types.ClassSym) *types.VarSym {
	if cls == nil {
		return nil
	}
	return &types.VarSym{Name: "this", Type: cls.This(), Kind: types.Param}
}

// narrowedThis returns the type of this if a smart cast has changed it.
func (c *Checker) narrowedThis() (types.Type, bool) {
	if c.this == nil {
		return types.InvalidType, false
	}
	t := c.scope.narrowed(c.this)
	return t, t != c.this.Type
}

// thisMember checks a name as a member of this once a smart cast has
// changed this's type: in when (this) { is Leaf -> value }, value is
// this.value. It reports false if the smart-cast type has no such member.
func (c *Checker) thisMember(x *syntax.Ident) (types.Type, bool) {
	if _, ok := c.narrowedThis(); !ok {
		return types.InvalidType, false
	}
	this := &syntax.This{Class: true} // not a lambda's receiver
	this.P = x.P
	m := &syntax.Member{X: this, Name: x.Name}
	m.P = x.P
	n := len(c.errs)
	t := c.expr(m, noWant)
	if len(c.errs) > n {
		c.errs = c.errs[:n]
		return types.InvalidType, false
	}
	x.Member = m
	return t, true
}

// thisMethods returns the methods named name of this's smart-cast type.
func (c *Checker) thisMethods(name string) bool {
	t, ok := c.narrowedThis()
	if !ok {
		return false
	}
	cls := c.classOf(t)
	return cls != nil && len(c.javaMethods(cls, name)) > 0
}

// coverage is what the branches of a when with a subject cover, for its
// exhaustiveness.
type coverage struct {
	null, isTrue, isFalse bool
	entries               map[string]bool   // enum entries
	classes               []*types.ClassSym // is checks, and objects compared with
}

// add records a condition that compares the subject with x.
func (cv *coverage) add(x syntax.Expr) {
	switch x := x.(type) {
	case *syntax.NullLit:
		cv.null = true
	case *syntax.BoolLit:
		cv.isTrue = cv.isTrue || x.Val
		cv.isFalse = cv.isFalse || !x.Val
	case *syntax.Ident:
		if x.Member != nil {
			cv.add(x.Member) // an imported entry
			return
		}
		if x.Object != nil {
			cv.classes = append(cv.classes, x.Object)
		}
		if x.Sym != nil && x.Sym.Kind == types.Static {
			cv.entry(x.Sym.Name)
		}
	case *syntax.Member:
		if x.Object != nil {
			cv.classes = append(cv.classes, x.Object)
		}
		if x.Field != nil {
			cv.entry(x.Field.Name)
		}
	}
}

func (cv *coverage) entry(name string) {
	if cv.entries == nil {
		cv.entries = map[string]bool{}
	}
	cv.entries[name] = true
}

// covers reports whether the branches cover every value of a subject of
// type t: true and false, every enum entry, or every subclass of a sealed
// class, and null if t is nullable.
func (c *Checker) covers(t types.Type, cv *coverage) bool {
	if types.MayBeNull(t) && !t.P && !cv.null {
		return false
	}
	t = t.NonNull()
	switch {
	case t.K == types.Bool:
		return cv.isTrue && cv.isFalse
	case t.K != types.Class:
		return false
	}
	cls := t.C
	cls.Ensure()
	if cls.Entries != nil {
		for _, e := range cls.Entries {
			if !cv.entries[e] {
				return false
			}
		}
		return true
	}
	return c.sealedCovered(cls, cv.classes)
}

// sealedCovered reports whether the classes that is checks name cover
// every instance of cls: cls or one of its supertypes is among them, or
// cls is sealed and each of its subclasses is covered.
func (c *Checker) sealedCovered(cls *types.ClassSym, classes []*types.ClassSym) bool {
	for _, k := range classes {
		if cls.IsSubclassOf(k) {
			return true
		}
	}
	if !cls.Sealed {
		return false
	}
	for _, s := range cls.Subclasses {
		if !c.sealedCovered(s, classes) {
			return false
		}
	}
	return true
}

// enumerable reports whether a when over a subject of type t must be
// exhaustive even as a statement: t is Boolean, an enum or a sealed class.
func enumerable(t types.Type) bool {
	t = t.NonNull()
	switch {
	case t.K == types.Bool:
		return true
	case t.K == types.Class:
		t.C.Ensure()
		return t.C.Entries != nil || t.C.Sealed
	}
	return false
}
