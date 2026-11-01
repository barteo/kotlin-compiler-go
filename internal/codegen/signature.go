package codegen

import (
	"strings"

	cf "github.com/barteo/kotlin-compiler-go/internal/classfile"
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// Generic signatures: the Signature attributes that Java compilers, and
// reflection, read classes' type parameters and generic supertypes from,
// and those of methods and fields, as annotation processors such as
// Dagger's read them. Kotlin code reads them from Kotlin metadata instead.

// classSignature returns a class's generic signature, as kotlinc writes
// it: its own type parameters, then its superclass and interfaces, those
// of the class file, super and interfaces, in their order. It is "" for a
// class without type parameters or generic supertypes.
//
// As with kotlinc, a supertype's own type arguments have no wildcards,
// which Java doesn't allow there, while the arguments within them, and
// those of bounds, get the declaration-site variance of the parameters
// they are for: List<List<String>> is List<? extends List<? extends
// String>> in Java.
func classSignature(cls *types.ClassSym, super string, interfaces []string) string {
	var own []*types.TypeParamSym
	if len(cls.TypeParams) > cls.OuterParams {
		own = cls.TypeParams[:len(cls.TypeParams)-cls.OuterParams]
	}
	var w sigWriter
	w.typeParams(own)
	supertype := func(name string) {
		for _, t := range cls.Supers {
			if t.K == types.Class && t.C.Name == name {
				w.typ(t, nil, useSite)
				return
			}
		}
		w.WriteString("L" + name + ";")
	}
	supertype(super)
	for _, i := range interfaces {
		supertype(i)
	}
	s := w.String()
	if len(own) == 0 && !strings.Contains(s, "<") {
		return ""
	}
	return s
}

// wildMode says which type arguments get wildcards, ? extends T and
// ? super T, for the declaration-site variance of their parameters, as
// kotlinc's TypeMappingMode.
type wildMode int

const (
	// declared: all, as the arguments of bounds and of supertypes'
	// arguments.
	declared wildMode = iota
	// useSite: none but use-site projections, as a supertype's own
	// arguments, where Java doesn't allow wildcards.
	useSite
	// inParam: those that widen the type, as a parameter's: not an out
	// argument whose type has no subtypes, String in List<String>, nor
	// an in argument of type Any, Comparable<Any>.
	inParam
	// inResult: none, but within contravariant arguments, which are
	// inParam, as a result's type: Comparable<List<? extends T>>.
	inResult
	// suppressed: none, under @JvmSuppressWildcards.
	suppressed
)

// nested returns the mode of the arguments of an argument of effective
// variance v: an invariant argument's are a result's, a contravariant
// one's a parameter's.
func (m wildMode) nested(v types.Variance) wildMode {
	switch m {
	case useSite:
		return declared
	case inParam:
		if v == types.Inv {
			return inResult
		}
	case inResult:
		if v == types.In {
			return inParam
		}
	}
	return m
}

type sigWriter struct{ strings.Builder }

// typeParams writes a list of type parameters with their bounds: a class
// bound, which is empty if the first bound is an interface, then
// interface bounds.
func (w *sigWriter) typeParams(ps []*types.TypeParamSym) {
	if len(ps) == 0 {
		return
	}
	w.WriteByte('<')
	for _, p := range ps {
		w.WriteString(p.Name)
		if len(p.Bounds) == 0 {
			w.WriteString(":Ljava/lang/Object;")
		}
		for i, b := range p.Bounds {
			w.WriteByte(':')
			if i == 0 && !isClassBound(b) {
				w.WriteByte(':') // no class bound
			}
			w.typ(b, nil, declared)
		}
	}
	w.WriteByte('>')
}

// isClassBound reports whether kotlinc writes a bound as a type
// parameter's class bound: a class, not an interface or a type parameter.
func isClassBound(b types.Type) bool {
	switch b.K {
	case types.TypeParam, types.TypeVar:
		return false
	case types.Class:
		b.C.Ensure()
		return !b.C.Interface
	}
	return true
}

// typ writes a reference type, whose arguments get wildcards as mode m
// says. ref is the type as written, if known, whose annotations
// @JvmSuppressWildcards and @JvmWildcard change that.
func (w *sigWriter) typ(t types.Type, ref *syntax.TypeRef, m wildMode) {
	if box, ok := types.Boxes[t.K]; ok {
		w.WriteString("L" + box + ";")
		return
	}
	switch t.K {
	case types.TypeParam, types.TypeVar:
		w.WriteString("T" + t.Param().Name + ";")
	case types.Any:
		w.WriteString("Ljava/lang/Object;")
	case types.String:
		w.WriteString("Ljava/lang/String;")
	case types.Unit:
		w.WriteString("Lkotlin/Unit;")
	case types.Class:
		switch {
		case t.C == types.ArrayClass:
			// An array's element has no wildcard, which Java doesn't
			// allow, but its arguments may.
			w.WriteByte('[')
			elem, v := types.AnyType, types.Inv
			if t.A != nil {
				if a := t.A.Get(0); a.V == types.Inv || a.V == types.Out {
					elem, v = a.T, a.V
				}
			}
			w.typ(elem, argRef(ref, t, 0), m.nested(v))
		case t.C.Array:
			w.WriteString(t.C.Name) // a primitive array
		default:
			w.WriteByte('L')
			w.classRef(t, ref, m)
			w.WriteByte(';')
		}
	default:
		// Nothing, which only a raw type's argument can be: see classRef.
		w.WriteString("Ljava/lang/Void;")
	}
}

// argRef returns the type written for argument i of type t, if ref is t
// as written: a function type's are its receiver, parameters and result.
func argRef(ref *syntax.TypeRef, t types.Type, i int) *syntax.TypeRef {
	if ref == nil {
		return nil
	}
	if ref.Ret != nil {
		refs := ref.Contexts[:len(ref.Contexts):len(ref.Contexts)]
		if ref.Recv != nil {
			refs = append(refs, ref.Recv)
		}
		refs = append(append(refs, ref.Params...), ref.Ret)
		if t.K == types.Class && t.C.Func && len(refs) == t.A.Len() {
			return refs[i]
		}
		return nil
	}
	if len(ref.Args) != t.A.Len() || i >= len(ref.Args) {
		return nil // an alias's, which needn't match
	}
	return ref.Args[i].Type
}

// classRef writes a class type without its L and ;. An inner class's type
// is its outer class's type, then its simple name and its own arguments.
// As with kotlinc, a type with a Nothing argument is raw.
func (w *sigWriter) classRef(t types.Type, ref *syntax.TypeRef, m wildMode) {
	c := t.C
	n := t.A.Len()
	for i := range n {
		if a := t.A.Get(i); a.V != types.Star && (a.T.K == types.Nothing || a.T.K == types.Null) {
			n = 0
		}
	}
	own := n
	if c.Inner && c.Outer != nil && c.OuterParams > 0 && n == len(c.TypeParams) {
		own = n - c.OuterParams
		w.classRef(types.GenericType(c.Outer, t.A.List[own:]), nil, m)
		w.WriteString("." + c.Name[len(c.Outer.Name)+1:])
	} else {
		w.WriteString(c.Name)
	}
	if own == 0 {
		return
	}
	w.WriteByte('<')
	if c.Func && c.Suspend {
		// suspend (P1, ..., Pn) -> R is Function<n+1><P1, ...,
		// Pn, Continuation<in R>, Any?>.
		for i := range own - 1 {
			w.arg(t.A.Get(i), types.In, argRef(ref, t, i), m)
		}
		w.continuation(t.A.Get(own-1).T, argRef(ref, t, own-1), m)
		w.variance(w.wildcard(types.Arg{T: types.AnyType.Nullable()}, types.Out, m, false))
		w.WriteString("Ljava/lang/Object;")
	} else if c.Func && c.Composable {
		// @Composable (P1, ..., Pn) -> R is Function<n+1+k><P1, ...,
		// Pn, Composer, Integer..., R>, with k Integers of change flags.
		for i := range own - 1 {
			w.arg(t.A.Get(i), types.In, argRef(ref, t, i), m)
		}
		in := ""
		if m == declared || m == inParam {
			in = "-"
		}
		w.WriteString(in + types.ComposerDesc)
		for range c.Methods["invoke"][0].Changed {
			w.WriteString(in + "Ljava/lang/Integer;")
		}
		w.arg(t.A.Get(own-1), types.Out, argRef(ref, t, own-1), m)
	} else if c.VarArity() || c.KFunctionN() {
		// (P1, ..., Pn) -> R is FunctionN<R>, and KFunctionN<P1, ...,
		// Pn, R> KFunction<R>.
		w.arg(t.A.Get(own-1), types.Out, argRef(ref, t, own-1), m)
	} else {
		c.Ensure()
		for i := range own {
			v := types.Inv
			if i < len(c.TypeParams) {
				v = c.TypeParams[i].Var
			}
			w.arg(t.A.Get(i), v, argRef(ref, t, i), m)
		}
	}
	w.WriteByte('>')
}

// continuation writes the type of a suspend function's continuation,
// Continuation<in R>, as an argument of a parameter or function type of
// mode m.
func (w *sigWriter) continuation(r types.Type, ref *syntax.TypeRef, m wildMode) {
	if m == declared || m == inParam {
		w.WriteByte('-') // a Continuation isn't Any
	}
	w.WriteString("Lkotlin/coroutines/Continuation<")
	w.arg(types.Arg{T: r}, types.In, ref, m.nested(types.In))
	w.WriteString(">;")
}

// arg writes type argument a for a parameter of variance v: its use-site
// projection, or the wildcard of v that mode m gives it, or * for a star
// projection. ref is a's type as written, if known.
func (w *sigWriter) arg(a types.Arg, v types.Variance, ref *syntax.TypeRef, m wildMode) {
	if a.V == types.Star {
		w.WriteByte('*')
		return
	}
	force := false
	if ref != nil {
		switch ref.Wildcards {
		case syntax.SuppressWildcards:
			m = suppressed
		case syntax.ForceWildcards:
			force = true
		}
	}
	wv := w.wildcard(a, v, m, force)
	if wv == types.Star {
		w.WriteByte('*') // In<out T>, Out<in T>
		return
	}
	w.variance(wv)
	eff := a.V
	if eff == types.Inv {
		eff = v
	}
	w.typ(a.T, ref, m.nested(eff))
}

// wildcard returns the variance of the wildcard of argument a for a
// parameter of variance v, Inv for none: a use-site projection is kept,
// and a declaration-site variance written where mode m says, or if
// forced.
func (w *sigWriter) wildcard(a types.Arg, v types.Variance, m wildMode, force bool) types.Variance {
	if v == types.Inv || m == useSite || m == declared && a.V != types.Inv {
		return a.V
	}
	if a.V != types.Inv && a.V != v {
		return types.Star
	}
	switch {
	case force || m == declared:
		return v
	case m != inParam:
		return types.Inv
	case v == types.Out && !hasSubtypes(a.T), v == types.In && a.T.K == types.Any:
		return types.Inv // List<String>, Comparable<Any>: as precise
	}
	return v
}

func (w *sigWriter) variance(v types.Variance) {
	switch v {
	case types.Out:
		w.WriteByte('+')
	case types.In:
		w.WriteByte('-')
	}
}

// hasSubtypes reports whether a type may have subtypes, so that an out
// argument of it gets a wildcard: kotlinc's canHaveSubtypes, nullability
// aside. A final class's type has none, unless an argument of it can
// vary: an out one that has subtypes, an in one that isn't Any, or *.
func hasSubtypes(t types.Type) bool {
	switch t.K {
	case types.Class:
	case types.TypeParam, types.TypeVar, types.Any:
		return true
	default:
		return false // String, Unit and the primitives' boxes are final
	}
	c := t.C
	if c != types.ArrayClass && !c.Array {
		c.Ensure()
		if !c.Final || c.Interface || c.Abstract || c.Sealed || c.Entries != nil {
			return true // as kotlinc, an enum class's entries are its subclasses
		}
	}
	for i := range t.A.Len() {
		a := t.A.Get(i)
		v := a.V
		if v == types.Star {
			return true
		}
		if v == types.Inv && i < len(c.TypeParams) {
			v = c.TypeParams[i].Var
		}
		switch {
		case v == types.Out && hasSubtypes(a.T), v == types.In && a.T.K != types.Any:
			return true
		}
	}
	return false
}

// sigPart is a parameter or the result of a method, for its signature:
// its type, as written if ref is set, or for a JVM-only parameter, such
// as a Composer, none: the descriptor's type then stands.
type sigPart struct {
	t       types.Type
	ref     *syntax.TypeRef
	cont    bool // a suspend function's continuation: Continuation<in t>
	anyColl bool // Collection<*>
	erase   bool // Object, if the descriptor's is
}

// typedPart returns the part of a value of type t, written ref.
func typedPart(t types.Type, ref *syntax.TypeRef) sigPart { return sigPart{t: t, ref: ref} }

// member writes the type of a parameter, result or field whose
// descriptor is d, in mode m: d itself if it is a primitive's, or if the
// part's type isn't known or doesn't erase to it.
func (w *sigWriter) member(p sigPart, d string, m wildMode) {
	if p.erase && d == "Ljava/lang/Object;" {
		w.WriteString(d)
		return
	}
	if p.anyColl && d == "Ljava/util/Collection;" {
		w.WriteString("Ljava/util/Collection<*>;")
		return
	}
	if p.cont && d == types.ContinuationDesc {
		w.WriteString("Lkotlin/coroutines/Continuation<")
		w.arg(types.Arg{T: p.t}, types.In, p.ref, m)
		w.WriteString(">;")
		return
	}
	t, ref := p.t, p.ref
	if types.Unboxed(t) {
		t, ref = types.Repr(t), nil // a value class's underlying value
	}
	if len(d) == 1 || t.K == types.Invalid || t.K == types.Unit {
		w.WriteString(d)
		return
	}
	if pd, ok := types.ParamDesc(t); !ok || pd != d {
		w.WriteString(d)
		return
	}
	if ref != nil {
		switch ref.Wildcards {
		case syntax.SuppressWildcards:
			m = suppressed
		case syntax.ForceWildcards:
			m = declared
		}
	}
	w.typ(t, ref, m)
}

// methodSig returns the generic signature of a method of descriptor desc
// with type parameters tps, whose parameters are params, those of desc
// but for a leading skip, and result ret, or "" if it has no generic
// types and skips none. Parameters get wildcards as mode m says, and the
// result as the mode's for results. It is also "" if params don't match
// desc's.
func methodSig(tps []*types.TypeParamSym, params []sigPart, ret sigPart, desc string, skip int, m wildMode) string {
	ds := descParams(desc)
	if len(ds) != skip+len(params) {
		return ""
	}
	var w sigWriter
	w.typeParams(tps)
	start := w.Len()
	w.WriteByte('(')
	for i, p := range params {
		w.member(p, ds[skip+i], m)
	}
	w.WriteByte(')')
	r := desc[strings.IndexByte(desc, ')')+1:]
	rm := inResult
	if m == suppressed {
		rm = suppressed
	}
	w.member(ret, r, rm)
	s := w.String()
	if len(tps) == 0 && skip == 0 && s[start:] == desc {
		return ""
	}
	return s
}

// fieldSig returns the generic signature of a field of type t, written
// ref, whose descriptor is d, or "" if it has no generic type: a var's
// has the wildcards of its setter's parameter, a val's those of its
// getter's result.
func fieldSig(t types.Type, ref *syntax.TypeRef, d string, mutable bool, m wildMode) string {
	if m != suppressed && !mutable {
		m = inResult
	}
	var w sigWriter
	w.member(typedPart(t, ref), d, m)
	if s := w.String(); s != d {
		return s
	}
	return ""
}

// wildcards returns the mode of the parameters of a declaration with
// annotations as, in class cls of the program, if any, whose
// @JvmSuppressWildcards apply to it, as do those of the file's.
func (g *gen) wildcards(as []syntax.Annotation, cls *types.ClassSym) wildMode {
	mode := func(w syntax.Wildcards) (wildMode, bool) {
		switch w {
		case syntax.SuppressWildcards:
			return suppressed, true
		case syntax.ForceWildcards:
			return inParam, true
		}
		return inParam, false
	}
	if m, ok := mode(syntax.WildcardsOf(as)); ok {
		return m
	}
	for c := cls; c != nil; c = c.Outer {
		if cd, ok := c.Decl.(*syntax.ClassDecl); ok {
			if m, ok := mode(syntax.WildcardsOf(cd.Annotations)); ok {
				return m
			}
		}
	}
	var file []syntax.Annotation
	for _, a := range g.file.Annotations {
		if a.Target == "file" {
			a.Target = ""
			file = append(file, a)
		}
	}
	m, _ := mode(syntax.WildcardsOf(file))
	return m
}

// signFun gives method mr of function f, whose descriptor is desc and
// declaration fd, if any, its generic signature. this is the type of a
// DefaultImpls method's first parameter, the interface, or invalid.
func (g *gen) signFun(mr cf.MemberRef, f *types.FunSym, fd *syntax.FunDecl, desc string, this types.Type) {
	g.signParams(mr, f, fd, desc, this, f.Params)
}

// signParams is signFun for a method that takes some of f's parameters,
// params, as an overload of @JvmOverloads.
func (g *gen) signParams(mr cf.MemberRef, f *types.FunSym, fd *syntax.FunDecl, desc string, this types.Type, fparams []*types.VarSym) {
	var as []syntax.Annotation
	var recvRef, retRef *syntax.TypeRef
	refs := map[*types.VarSym]*syntax.TypeRef{}
	if fd != nil {
		as, recvRef, retRef = fd.Annotations, fd.Recv, fd.Ret
		for _, p := range fd.Params {
			if !p.Vararg {
				refs[p.Sym] = p.Type
			}
		}
		if fd.AccessorOf != nil {
			// A custom accessor: the property's type and annotations.
			if pd, ok := fd.AccessorOf.Decl.(*syntax.PropDecl); ok {
				as, recvRef, retRef = pd.Annotations, pd.Recv, pd.Type
				for _, p := range fd.Params {
					refs[p.Sym] = pd.Type
				}
			}
		}
	}
	tps := f.TypeParams
	var params []sigPart
	if this.K != types.Invalid {
		params = append(params, sigPart{t: this})
		if len(this.C.TypeParams) > 0 {
			tps = append(append([]*types.TypeParamSym(nil), this.C.TypeParams...), tps...)
		}
	}
	for _, p := range f.Contexts {
		var ref *syntax.TypeRef
		if d, ok := p.Decl.(*syntax.Param); ok {
			ref = d.Type
		}
		params = append(params, typedPart(p.Type, ref))
	}
	if f.Ext {
		params = append(params, typedPart(f.Recv, recvRef))
	}
	special := specialGeneric(f, fparams)
	for i, p := range fparams {
		params = append(params, sigPart{t: p.Type, ref: refs[p], erase: special[i] == erasedParam, anyColl: special[i] == anyCollection})
	}
	ret := typedPart(types.JVMRet(f), retRef)
	if f.Suspend {
		params = append(params, sigPart{t: f.Ret, ref: retRef, cont: true})
		ret = sigPart{t: types.AnyType.Nullable()}
	}
	if f.Composable {
		for range 1 + f.Changed + f.Defaults {
			params = append(params, sigPart{t: types.InvalidType})
		}
	}
	if s := methodSig(tps, params, ret, desc, 0, g.wildcards(as, f.Owner)); s != "" {
		mr.SetSignature(s)
	}
}

// signField gives field mr of a property of type t, written ref, its
// generic signature: d is its descriptor, and as the property's
// annotations in class cls, if any.
func (g *gen) signField(mr cf.MemberRef, t types.Type, ref *syntax.TypeRef, d string, mutable bool, as []syntax.Annotation, cls *types.ClassSym) {
	if s := fieldSig(t, ref, d, mutable, g.wildcards(as, cls)); s != "" {
		mr.SetSignature(s)
	}
}

// signAccessor gives mr, the default getter or setter of descriptor d of
// a property of type t, written ref, its generic signature, as
// signField's.
func (g *gen) signAccessor(mr cf.MemberRef, t types.Type, ref *syntax.TypeRef, d string, as []syntax.Annotation, cls *types.ClassSym, ctx ...*types.VarSym) {
	m := g.wildcards(as, cls)
	var parts []sigPart // the context parameters', which come first
	for _, p := range ctx {
		var r *syntax.TypeRef
		if q, ok := p.Decl.(*syntax.Param); ok {
			r = q.Type
		}
		parts = append(parts, typedPart(p.Type, r))
	}
	var s string
	if len(descParams(d)) == len(ctx) { // a getter's
		s = methodSig(nil, parts, typedPart(t, ref), d, 0, m)
	} else {
		s = methodSig(nil, append(parts, typedPart(t, ref)), sigPart{t: types.UnitType}, d, 0, m)
	}
	if s != "" {
		mr.SetSignature(s)
	}
}

// propRef returns property p's type as written: a declaration's or a
// constructor parameter's.
func propRef(p *types.VarSym) *syntax.TypeRef {
	switch d := p.Decl.(type) {
	case *syntax.PropDecl:
		return d.Type
	case *syntax.CtorParam:
		if !d.Param.Vararg && !d.Modifiers.Vararg {
			return d.Type
		}
	}
	return nil
}

// signCtor gives mr, the constructor f of class cls, of descriptor d, its
// generic signature: as kotlinc's, and javac's, it leaves out an inner
// class's outer instance and an enum's name and ordinal. A local class's
// gets none. params are its parameters' declarations, and as its
// annotations.
func (g *gen) signCtor(mr cf.MemberRef, cls *types.ClassSym, f *types.FunSym, params []*syntax.Param, d string, as []syntax.Annotation) {
	if cls.Local || cls.Anonymous || len(f.Params) != len(params) {
		return
	}
	skip := 0
	switch cd, _ := cls.Decl.(*syntax.ClassDecl); {
	case cls.Inner:
		skip = 1
	case cls.Entries != nil || cd != nil && cd.EnumEntry:
		skip = 2
	}
	var ps []sigPart
	for i, p := range f.Params {
		var ref *syntax.TypeRef
		if !params[i].Vararg {
			ref = params[i].Type
		}
		ps = append(ps, typedPart(p.Type, ref))
	}
	if s := methodSig(nil, ps, sigPart{t: types.UnitType}, d, skip, g.wildcards(as, cls)); s != "" {
		mr.SetSignature(s)
	}
}

// How specialGeneric says a parameter's signature is Java's.
const (
	ownParam      = iota
	erasedParam   // Object, as the descriptor's
	anyCollection // Collection<?>
)

// specialGeneric returns, for each of the parameters of f, a member of a
// collection class, whether Java's method that it implements has another
// type for it: as kotlinc, the members of Java's collections that take an
// Object or a Collection<?>, contains(E) and containsAll(Collection<E>)
// in Kotlin, have their signatures, which overriding needs.
func specialGeneric(f *types.FunSym, params []*types.VarSym) []int {
	if f.Owner == nil || f.Ext || f.Static || len(mappedSupers(f.Owner)) == 0 {
		return make([]int, len(params))
	}
	return specialParams(f.Name, f.Ret, params)
}

// specialParams is specialGeneric for a member of a collection type
// named name, with result r and parameters params.
func specialParams(name string, r types.Type, params []*types.VarSym) []int {
	out := make([]int, len(params))
	if len(params) == 0 || len(params) > 2 {
		return out
	}
	// Java's methods, by their results: a member with another result,
	// as PersistentList.remove(E): PersistentList<E>, doesn't override.
	var value, coll bool
	switch name {
	case "contains", "containsKey", "containsValue":
		value = r.K == types.Bool
	case "remove":
		value = r.K == types.Bool || len(params) == 1 && r.K == types.TypeParam
	case "indexOf", "lastIndexOf":
		value = r.K == types.Int
	case "get", "getOrDefault":
		value = r.K == types.TypeParam
	case "containsAll", "removeAll", "retainAll":
		coll = len(params) == 1 && r.K == types.Bool
	}
	if !value && !coll {
		return out
	}
	for i, p := range params {
		switch d, _ := types.ParamDesc(p.Type); {
		case value && p.Type.K == types.TypeParam && (name != "getOrDefault" || i == 0):
			out[i] = erasedParam
		case coll && d == "Ljava/util/Collection;":
			out[i] = anyCollection
		}
	}
	return out
}
