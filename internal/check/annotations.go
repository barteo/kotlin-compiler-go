package check

import (
	"cmp"
	"slices"
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// The annotations that class files keep, those of annotation classes
// whose retention isn't SOURCE, are resolved and their arguments checked
// against the annotation class's parameters, for codegen to write. An
// annotation whose class isn't found is left out, as kc ignored all
// annotations before it wrote them.

// annotArg is an argument of an annotation: its name, if it is named,
// and its value, or the elements of an array literal [a, b].
type annotArg struct {
	name  string
	elems []syntax.Expr
	array bool
	pos   syntax.Pos
}

// annotations resolves and checks the annotations among as, of a
// declaration in class owner (nil at top level), written on site: where
// they are, and whether they repeat. Those that class files keep, and
// those of the program's SOURCE annotation classes, have their arguments
// checked.
func (c *Checker) annotations(as []syntax.Annotation, owner *types.ClassSym, site annotSite) {
	c.checkAnnotations(as, owner, site, false)
}

// localAnnotations checks the annotations as of a local declaration, a
// statement or an expression, written on site, as annotations does, but
// in the current scope, and without keeping them: kc doesn't write them.
// A statement can be checked more than once, as in a lambda's body, so
// the errors are reported once.
func (c *Checker) localAnnotations(as []syntax.Annotation, site annotSite) {
	if len(as) == 0 {
		return
	}
	n := len(c.errs)
	c.checkAnnotations(as, c.cls, site, true)
	if len(c.errs) == n {
		return
	}
	news := c.errs[n:]
	c.errs = c.errs[:n]
	for _, e := range news {
		if !slices.ContainsFunc(c.errs, func(o *syntax.Error) bool { return o.Pos == e.Pos && o.Msg == e.Msg && o.File == e.File }) {
			c.errs = append(c.errs, e)
		}
	}
}

// annotatedNode checks the annotations the parser recorded of statement
// or expression n (see syntax.File.Annotated), written on site.
func (c *Checker) annotatedNode(n any, site annotSite) {
	if c.file == nil || c.file.Annotated == nil {
		return
	}
	if as, ok := c.file.Annotated[n]; ok {
		c.localAnnotations(as, site)
	}
}

// annotatedExpr checks the annotations of expression x.
func (c *Checker) annotatedExpr(x syntax.Expr) {
	if c.file == nil || c.file.Annotated == nil {
		return
	}
	site := exprSite
	switch x := x.(type) {
	case *syntax.Lambda:
		if x.Ref == nil { // not ::f
			site = lambdaSite
		}
	case *syntax.ObjectExpr:
		site = objectExprSite
	}
	c.annotatedNode(x, site)
}

// javaAnnotationReplacements are the Java annotations that kotlinc
// deems deprecated on Kotlin declarations (DEPRECATED_JAVA_ANNOTATION),
// by class, and the Kotlin annotations that replace them.
var javaAnnotationReplacements = map[string]string{
	"java/lang/Deprecated":            "kotlin.Deprecated",
	"java/lang/annotation/Target":     "kotlin.annotation.Target",
	"java/lang/annotation/Retention":  "kotlin.annotation.Retention",
	"java/lang/annotation/Documented": "kotlin.annotation.MustBeDocumented",
}

// checkAnnotations is annotations, or localAnnotations if local is set.
func (c *Checker) checkAnnotations(as []syntax.Annotation, owner *types.ClassSym, site annotSite, local bool) {
	seen := map[repeatKey]bool{}
	var applied []appliedAnnotation
	defer func() { c.repeatedWithContainer(applied) }()
	// A class with Java's @Target but not Kotlin's targets everything in
	// Kotlin, of which kotlinc warns (ANNOTATION_TARGETS_ONLY_IN_JAVA).
	var javaTarget *syntax.Annotation
	kotlinTarget := false
	if site.kind == siteClass {
		defer func() {
			if javaTarget != nil && !kotlinTarget {
				c.warnf(javaTarget.P, "ANNOTATION_TARGETS_ONLY_IN_JAVA", "No Kotlin '@Target' annotation specified (implicitly targeting everything), but one exists for Java.")
			}
		}()
	}
	for i := range as {
		a := &as[i]
		if local && site.kind == siteLocal {
			c.localUseSite(a, site)
		}
		if a.Name == "" {
			continue // @[A B]
		}
		cls := c.annotationClassOf(a, owner, site, local)
		if cls == nil {
			continue
		}
		if rec := c.u.rec; rec != nil {
			rec.Annotation(c.file, a, cls)
		}
		c.dslMarkerTarget(a, cls, site)
		switch cls.Name {
		case "java/lang/annotation/Target":
			if javaTarget == nil && a.Target == "" {
				javaTarget = a
			}
		case "kotlin/annotation/Target":
			kotlinTarget = true
		}
		if k := javaAnnotationReplacements[cls.Name]; k != "" && cls.External {
			c.warnf(a.P, "DEPRECATED_JAVA_ANNOTATION", "This annotation is deprecated in Kotlin. Use '@%s' instead.", k)
		}
		if cls.Retention != types.NoRetention {
			applied = append(applied, appliedAnnotation{a, cls})
			c.annotationTarget(a, cls, site)
			c.repeated(a, cls, site, seen)
			c.jvmAnnotation(a, cls, site)
			c.javaPositionalArgs(a, cls)
		}
		if ps := cls.TypeParams; cls.Annotation && len(a.TypeArgs) != len(ps) && c.lookupAlias(a.Name) == nil {
			// Unlike a call, an annotation entry doesn't infer its
			// type arguments; a typealias may give them.
			if len(ps) == 0 {
				c.errorf(a.P, "no type arguments expected for annotation class %s", cls.DisplayName())
			} else {
				c.errorf(a.P, "%s expected for annotation class %s%s", countArgs(len(ps)), cls.DisplayName(), types.TypeParamsString(ps))
			}
		} else if c.u.rec != nil {
			// The type arguments, @A<String>, are only reported: a
			// generic annotation's arguments use them only in
			// collection literals (see literalAnnotations).
			for _, ta := range a.TypeArgs {
				if ta.Type != nil {
					c.resolveType(ta.Type)
				}
			}
		}
		source := cls.Retention == types.RetentionSource
		if source && !local {
			a.Source = cls
		}
		if cls.Retention == types.NoRetention {
			continue
		}
		if !c.u.ext.kotlin && !cls.Annotation {
			// Without the standard library, @Deprecated would be
			// java.lang.Deprecated: only the program's are written.
			continue
		}
		// A library's SOURCE-retention annotation, @Suppress, isn't
		// checked; its values are evaluated, without errors, only for a
		// ValueRecorder.
		vrec, _ := c.u.rec.(ValueRecorder)
		quiet := source && !cls.Annotation
		if quiet && vrec == nil {
			continue
		}
		before := len(c.errs)
		args := make([]annotArg, 0, len(a.Args))
		ok := true
		for j := range a.Args {
			arg := &a.Args[j]
			elems, array, err := syntax.ParseAnnotationArg(c.file.Path, arg)
			if err != nil {
				c.errorf(err.Pos, "%s", err.Msg)
				ok = false
				continue
			}
			if c.u.experimentalWarnings {
				for _, e := range elems {
					c.prefixWarnings(e)
				}
			}
			p := a.P
			if len(arg.Toks) > 0 {
				p = arg.Toks[0].Pos
			}
			args = append(args, annotArg{name: arg.Name, elems: elems, array: array, pos: p})
		}
		if !ok {
			if quiet {
				c.errs = c.errs[:before]
			}
			continue
		}
		if local {
			values, _ := c.annotationValues(cls, a.TypeArgs, args, a.P)
			if vrec != nil {
				vrec.AnnotationValues(c.file, a, values)
			}
			if quiet {
				c.errs = c.errs[:before]
			}
			continue
		}
		saved, savedLookup := c.state, c.lookupCls
		c.state = state{scope: &scope{}, staticCls: owner}
		if site.header && site.kind != siteCtor {
			for _, p := range c.headerParams {
				c.scope.declare(p)
			}
		}
		if this := annotationThis(owner, site); this != nil {
			c.cls, c.this, c.staticCls = this, newThis(this), nil
			c.inAnon(this) // an inner or local class's outer receivers
		}
		c.lookupCls = annotationScope(owner, c.lookupCls, site)
		values, ok := c.annotationValues(cls, a.TypeArgs, args, a.P)
		c.state, c.lookupCls = saved, savedLookup
		if !quiet {
			c.annotationSpreads(cls, args, a.Args)
		}
		if vrec != nil {
			vrec.AnnotationValues(c.file, a, values)
		}
		if quiet {
			c.errs = c.errs[:before]
			continue
		}
		if ok && !source {
			// Class files keep it.
			a.Class, a.Values = cls, values
		}
	}
}

// annotationParams returns the parameters of annotation class cls: its
// constructor's for a Kotlin class, its methods' for a Java one, whose
// value method takes a positional argument, and, if it is an array, any
// number of them, as a vararg parameter. A Java method's Class<T> is a
// KClass<T> to Kotlin.
func (c *Checker) annotationParams(cls *types.ClassSym) []*types.VarSym {
	if cls.Annotation || cls.AnnotationParams != nil {
		return cls.AnnotationParams
	}
	if len(cls.Ctors) > 0 {
		return cls.Ctors[0].Params
	}
	var ps []*types.VarSym
	for _, fs := range cls.Methods {
		for _, f := range fs {
			if f.Abstract && len(f.Params) == 0 {
				t := c.kclassOfJava(f.Ret)
				ps = append(ps, &types.VarSym{Name: f.Name, Type: t, Kind: types.Param, Vararg: f.Name == "value" && t.IsArray(), Default: cls.AnnotationDefaults[f.Name] != nil})
			}
		}
	}
	slices.SortFunc(ps, func(a, b *types.VarSym) int {
		switch {
		case a.Name == b.Name:
			return 0
		case a.Name == "value" || b.Name != "value" && a.Name < b.Name:
			return -1
		}
		return 1
	})
	return ps
}

// kclassOfJava returns t with java.lang.Class, or an array's elements of
// it, as kotlin.reflect.KClass.
func (c *Checker) kclassOfJava(t types.Type) types.Type {
	kclass := c.u.ext.class("kotlin/reflect/KClass")
	if kclass == nil || !c.u.ext.kotlin || t.K != types.Class {
		return t
	}
	if t.C.Array && t.A != nil && t.A.Len() == 1 {
		e := t.A.Get(0)
		if et := c.kclassOfJava(e.T); et != e.T {
			return types.GenericType(t.C, []types.Arg{{V: e.V, T: et}}).Platform()
		}
		return t
	}
	if t.C.Name != "java/lang/Class" {
		return t
	}
	if t.A == nil { // a raw Class, KClass<*>
		return types.GenericType(kclass, []types.Arg{{V: types.Star}}).Platform()
	}
	return types.GenericType(kclass, t.A.List).Platform()
}

// annotationValues maps the arguments of an annotation of class cls to its
// parameters, and checks them: positional ones in order, until a vararg
// parameter takes the rest, then named ones.
func (c *Checker) annotationValues(cls *types.ClassSym, targs []*syntax.TypeArg, args []annotArg, p syntax.Pos) ([]syntax.AnnotationValue, bool) {
	params := c.annotationParams(cls)
	// The arguments of a generic annotation class's annotation, @A<T>(x),
	// are checked with its type parameters projected: KClass<*> for a
	// KClass<T>.
	var star types.Subst
	if len(cls.TypeParams) > 0 {
		star = types.Subst{}
		for _, tp := range cls.TypeParams {
			star[tp] = types.Arg{V: types.Star}
		}
	}
	byParam := make([][]annotArg, len(params))
	next := 0
	ok := true
	for _, arg := range args {
		j := -1
		switch {
		case arg.name != "":
			j = slices.IndexFunc(params, func(v *types.VarSym) bool { return v.Name == arg.name })
			if j < 0 {
				c.errorf(arg.pos, "cannot find a parameter with this name: %s", arg.name)
				ok = false
				continue
			}
			if j == next && !params[j].Vararg {
				// Named in its own place: a positional argument may
				// follow, @A(x = 1, 2).
				next++
			}
		case next < len(params):
			j = next
			if !params[j].Vararg {
				next++
			}
		default:
			c.errorf(arg.pos, "too many arguments for annotation %s", cls.Name)
			ok = false
			continue
		}
		byParam[j] = append(byParam[j], arg)
	}
	var values []syntax.AnnotationValue
	for j, v := range params {
		if len(byParam[j]) == 0 {
			switch {
			case v.Vararg:
				// No arguments for a vararg parameter: kotlinc writes [].
				values = append(values, syntax.AnnotationValue{Name: v.Name, Type: v.Type, Array: true, Omitted: true})
			case !v.Default && (cls.Annotation || javaAnnotation(cls) || cls.External && cls.FromKotlin && cls.AnnotationParams != nil):
				c.errorf(p, "no value passed for parameter '%s'", v.Name)
				ok = false
			}
			continue
		}
		t := star.Apply(v.Type)
		val := syntax.AnnotationValue{Name: v.Name, Type: t, Array: t.IsArray() || t.K == types.Class && types.IsUnsignedArray(t.C)}
		if t.K == types.Invalid && len(byParam[j]) == 1 {
			// An unresolved parameter type, as Array<KClass<*>> without
			// its import: [a, b] is still an array, as in FIR.
			a := byParam[j][0]
			val.Array = a.array
			if len(a.elems) == 1 {
				_, isArr := syntax.ArrayElems(a.elems[0])
				val.Array = val.Array || isArr
			}
		}
		elem := t
		if val.Array {
			elem = types.VarargElem(t)
		}
		for _, arg := range byParam[j] {
			elems := arg.elems
			if v.Vararg && arg.name == "" && arg.array && val.Array {
				// [a, b] in a vararg's place is one element, an array:
				// @A([""]) of vararg s: String, unlike @A(*[""]).
				c.errorf(arg.pos, "argument type mismatch: actual type is %s but %s was expected", types.ArrayOf(elem), elem)
				ok = false
				continue
			}
			if sp, spread := firstSpread(elems); spread && !v.Vararg {
				c.errorf(sp.P, "the spread operator (*foo) can only be applied in a vararg position")
				ok = false
				continue
			}
			if val.Array && !arg.array && (!v.Vararg || arg.name != "") && len(elems) == 1 {
				// arrayOf(a, b) or [a, b] for an array parameter, or a
				// vararg one by name: ReplaceWith(imports = ["a.b"])
				if inner, ok := syntax.ArrayElems(elems[0]); ok {
					if got := arrayFactoryType(elems[0]); got.K != types.Invalid && got.C != t.C && t.K != types.Invalid {
						// intArrayOf() for a vararg of Strings
						c.errorf(elems[0].Pos(), "argument type mismatch: actual type is %s but %s was expected", got, t)
						ok = false
						continue
					}
					elems = inner
				} else if _, spread := elems[0].(*syntax.Spread); !spread {
					if v.Vararg {
						c.errorf(elems[0].Pos(), "assigning single elements to varargs in named form is prohibited")
					} else {
						c.errorf(elems[0].Pos(), "argument type mismatch: a single value is passed, but %s was expected", t)
					}
					ok = false
				}
			}
			if val.Array {
				elems = syntax.FlattenSpreads(elems) // *arrayOf(a, b)
			}
			for _, x := range elems {
				e, eok := c.annotationElem(x, elem)
				ok = ok && eok
				val.Elems = append(val.Elems, e)
			}
			if ok && arg.array && targs != nil && !v.Vararg && val.Array {
				ok = c.literalAnnotations(cls, targs, v.Type, elems, arg.pos)
			}
		}
		values = append(values, val)
	}
	return values, ok
}

// firstSpread returns elems' element that spreads an array, *[a, b], if
// it is its only one.
func firstSpread(elems []syntax.Expr) (*syntax.Spread, bool) {
	if len(elems) != 1 {
		return nil, false
	}
	sp, ok := elems[0].(*syntax.Spread)
	return sp, ok
}

// arrayFactoryType returns the class of the array that x, a call of
// arrayOf, emptyArray or a primitive or unsigned array's factory such as
// intArrayOf, creates, as a type to report, or Invalid for a collection
// literal, whose class is the expected one's.
func arrayFactoryType(x syntax.Expr) types.Type {
	call, ok := x.(*syntax.Call)
	if !ok || call.Literal {
		return types.InvalidType
	}
	var name string
	switch f := call.Fun.(type) {
	case *syntax.Ident:
		name = f.Name
	case *syntax.Member:
		name = f.Name
	}
	switch name {
	case "arrayOf", "emptyArray":
		return types.ClassType(types.ArrayClass)
	}
	// intArrayOf makes an IntArray. An unsigned array's, uintArrayOf,
	// isn't one of these classes.
	if name != "" {
		if cls := types.ArrayByName(strings.ToUpper(name[:1]) + strings.TrimSuffix(name[1:], "Of")); cls != nil {
			return types.ClassType(cls)
		}
	}
	return types.InvalidType
}

// literalAnnotations checks the elements of a collection literal, the
// argument [B(x), ...] of a generic annotation class cls's parameter of
// type want, Array<B<T>>, written with type arguments targs: as in
// kotlinc (KT-82964), each nested annotation's type arguments are
// inferred from its own arguments, without the expected type, B<T> with
// cls's T substituted. Only class literals, KClass<T>, are inferred
// from; B() with nothing to infer its T from doesn't fit.
func (c *Checker) literalAnnotations(cls *types.ClassSym, targs []*syntax.TypeArg, want types.Type, elems []syntax.Expr, pos syntax.Pos) bool {
	if !slices.ContainsFunc(cls.TypeParams, func(tp *types.TypeParamSym) bool { return mentionsParam(want, tp) }) {
		return true // the literal has its expected type
	}
	if len(targs) != len(cls.TypeParams) {
		return true
	}
	sub := types.Subst{}
	for i, tp := range cls.TypeParams {
		ta := targs[i]
		if ta.Type == nil || ta.Variance != types.Inv {
			return true
		}
		t := c.resolveTypeRef(ta.Type)
		if t.K == types.Invalid {
			return true
		}
		sub[tp] = types.Arg{V: types.Inv, T: t}
	}
	want = sub.Apply(want)
	wantElem := types.VarargElem(want)
	if wantElem.K != types.Class || len(wantElem.C.TypeParams) == 0 || wantElem.C.Retention == types.NoRetention {
		return true
	}
	var actual []types.Type
	fits := true
	for _, x := range elems {
		call, ok := x.(*syntax.Call)
		if !ok || call.Literal {
			return true
		}
		t, ok := c.literalAnnotationType(wantElem.C, call)
		if !ok {
			return true
		}
		actual = append(actual, t)
		fits = fits && c.subtype(t, wantElem)
	}
	if fits || len(actual) == 0 {
		return true
	}
	elem := actual[0]
	for _, t := range actual[1:] {
		elem = c.lub(elem, t)
	}
	got := types.GenericType(want.C, []types.Arg{{V: types.Inv, T: elem}})
	c.errorf(pos, "argument type mismatch: actual type is %s but %s was expected", got, want)
	return false
}

// literalAnnotationType returns the type of the nested annotation call
// of class b, with its type arguments explicit or inferred from the
// class literals passed for its KClass<T> parameters; false if they are
// inferred from anything else. A type parameter with nothing to infer it
// from stays itself.
func (c *Checker) literalAnnotationType(b *types.ClassSym, call *syntax.Call) (types.Type, bool) {
	if call.TypeArgs != nil {
		ts := c.typeArgs(call)
		if len(ts) != len(b.TypeParams) {
			return types.Type{}, false
		}
		args := make([]types.Arg, len(ts))
		for i, t := range ts {
			if t.K == types.Invalid {
				return types.Type{}, false
			}
			args[i] = types.Arg{V: types.Inv, T: t}
		}
		return types.GenericType(b, args), true
	}
	params := c.annotationParams(b)
	lower := make(map[*types.TypeParamSym][]types.Type)
	next := 0
	for i, a := range call.Args {
		j := next
		if i < len(call.Names) && call.Names[i] != "" {
			j = slices.IndexFunc(params, func(v *types.VarSym) bool { return v.Name == call.Names[i] })
		} else if j < len(params) && !params[j].Vararg {
			next++
		}
		if j < 0 || j >= len(params) {
			return types.Type{}, false
		}
		v := params[j]
		t := v.Type
		if v.Vararg {
			t = types.VarargElem(t)
		}
		if !slices.ContainsFunc(b.TypeParams, func(tp *types.TypeParamSym) bool { return mentionsParam(t, tp) }) {
			continue
		}
		lit, ok := a.(*syntax.ClassLit)
		if !ok || lit.Bound || lit.Class.K == types.Invalid || t.K != types.Class || len(t.A.List) != 1 || t.A.List[0].V == types.In {
			return types.Type{}, false
		}
		tp := t.A.List[0].T
		if tp.K != types.TypeParam || !slices.Contains(b.TypeParams, tp.Param()) {
			return types.Type{}, false
		}
		lower[tp.Param()] = append(lower[tp.Param()], lit.Class)
	}
	args := make([]types.Arg, len(b.TypeParams))
	for i, tp := range b.TypeParams {
		ts := lower[tp]
		if len(ts) == 0 {
			args[i] = types.Arg{V: types.Inv, T: types.ParamType(tp)}
			continue
		}
		t := ts[0]
		for _, u := range ts[1:] {
			t = c.lub(t, u)
		}
		args[i] = types.Arg{V: types.Inv, T: t}
	}
	return types.GenericType(b, args), true
}

// annotationElem checks a value of an annotation's argument, of type t:
// a nested annotation if t is an annotation class, else a constant, an
// enum entry or a class literal.
func (c *Checker) annotationElem(x syntax.Expr, t types.Type) (syntax.AnnotationElem, bool) {
	if t.K == types.Class && !t.C.Array {
		t.C.Ensure()
		if t.C.Retention != types.NoRetention {
			call, ok := x.(*syntax.Call)
			if !ok {
				c.errorf(x.Pos(), "an annotation argument must be an annotation: %s", t)
				return syntax.AnnotationElem{}, false
			}
			args := make([]annotArg, len(call.Args))
			for i, a := range call.Args {
				args[i] = annotArg{elems: []syntax.Expr{a}, pos: a.Pos()}
				if i < len(call.Names) {
					args[i].name = call.Names[i]
				}
			}
			values, ok := c.annotationValues(t.C, nil, args, call.Pos())
			nested := &syntax.Annotation{P: call.Pos(), Class: t.C, Values: values}
			if rec := c.u.rec; rec != nil {
				rec.Annotation(c.file, nested, t.C)
			}
			return syntax.AnnotationElem{Nested: nested}, ok
		}
	}
	before := len(c.errs)
	c.assignable(c.value(x, t), t, x.Pos())
	if len(c.errs) == before {
		if bad := c.nonConst(x); bad != nil {
			c.errorf(bad.Pos(), "annotation argument must be a compile-time constant")
		}
	}
	return syntax.AnnotationElem{X: x}, len(c.errs) == before
}

// declAnnotations checks the annotations of declaration d, in class owner
// (nil at top level), and of its parameters, accessors and members.
func (c *Checker) declAnnotations(d syntax.Decl, owner *types.ClassSym) {
	params := func(ps []*syntax.Param) {
		for _, p := range ps {
			c.noAllTarget(p.Annotations, "value parameters, only properties are allowed")
			c.noReceiver(p.Annotations)
			c.noParamTarget(p.Annotations)
			c.noFileTarget(p.Annotations)
			c.annotations(p.Annotations, owner, paramSite)
		}
	}
	fun := func(f *syntax.FunDecl, site annotSite) {
		if f == nil {
			return
		}
		c.annotations(f.Annotations, owner, site)
		c.annotations(f.RecvAnnotations, owner, annotSite{})
		c.signatureTypeAnnotations(owner, f.Recv, f.Ret)
		c.paramTypeAnnotations(owner, f.Params)
		if f.Property == "" {
			params(f.Contexts) // an accessor's are its property's
		}
		params(f.Params)
	}
	c.jvmMemberAnnotations(d, owner)
	switch d := d.(type) {
	case *syntax.FunDecl:
		c.noAllTarget(d.Annotations, "elements other than properties")
		c.noReceiver(d.Annotations)
		c.noParamTarget(d.Annotations)
		c.noFileTarget(d.Annotations)
		c.publishedApi(&d.Modifiers)
		fun(d, funSite(owner))
		c.anyOverrideAnnotations(d, owner)
		c.typeParamAnnotations(d.TypeParams, owner)
	case *syntax.PropDecl:
		c.noReceiver(d.Annotations)
		c.noParamTarget(d.Annotations)
		c.noFileTarget(d.Annotations)
		c.onAccessor(d.Annotations)
		c.publishedApi(&d.Modifiers)
		c.annotations(d.Annotations, owner, propSite(owner))
		if d.Field != nil {
			c.noAllTarget(d.Annotations, "delegated properties")
		} else if d.Sym != nil {
			c.allTargets(d.Annotations, false, d.Mutable, d.Sym.HasField() || d.Lateinit || d.Const || d.BackingField != nil || d.Expect)
		}
		if !d.Mutable {
			c.setOnImmutable(d.Annotations)
		}
		c.fieldless(d)
		c.annotations(d.RecvAnnotations, owner, annotSite{})
		c.typeParamAnnotations(d.TypeParams, owner)
		c.signatureTypeAnnotations(owner, d.Recv, d.Type)
		params(d.Contexts)
		fun(d.Getter, annotSite{siteUnchecked, types.TargetGetter, "getter", false, false, false})
		fun(d.Setter, annotSite{siteUnchecked, types.TargetSetter, "setter", false, false, false})
		c.repeatedSetparam(d)
	case *syntax.TypeAliasDecl:
		c.noAllTarget(d.Annotations, "elements other than properties")
		c.noFileTarget(d.Annotations)
		c.annotations(d.Annotations, owner, typealiasSite)
		c.typeParamAnnotations(d.TypeParams, owner)
		c.signatureTypeAnnotations(owner, d.Type)
	case *syntax.ClassDecl:
		cls := d.Sym
		if d.Annotation {
			c.annotationCycles(d)
			c.projectedAnnotationParams(d)
		}
		c.noAllTarget(d.Annotations, "elements other than properties")
		c.noAllTarget(d.CtorMods.Annotations, "elements other than properties")
		c.noReceiver(d.Annotations)
		c.noReceiver(d.CtorMods.Annotations)
		c.noFileTarget(d.Annotations)
		c.noFileTarget(d.CtorMods.Annotations)
		c.publishedApi(&d.Modifiers)
		c.publishedApi(&d.CtorMods)
		c.annotations(d.Annotations, cls, classSite(d))
		if d.Annotation {
			c.repeatableContainer(d)
		}
		c.annotations(d.CtorMods.Annotations, cls, ctorSite)
		if !c.done[localHeader{d}] {
			c.typeParamAnnotations(d.TypeParams, cls)
			superSite := typeSite
			superSite.outerThis = cls.Inner
			for _, s := range d.Supers {
				c.typeAnnotationsAt(cls.Outer, superSite, s.Type) // outside it, as its annotations
			}
		}
		for _, cp := range d.CtorParams {
			c.signatureTypeAnnotations(cls, cp.Type)
		}
		c.headerParams = cls.CtorParams
		for _, cp := range d.CtorParams {
			c.noReceiver(cp.Modifiers.Annotations)
			c.noFileTarget(cp.Modifiers.Annotations)
			if cp.IsProp {
				c.publishedApi(&cp.Modifiers)
			}
			c.annotations(cp.Modifiers.Annotations, cls, ctorParamSite(cls, cp))
			if !cp.IsProp {
				c.noAllTarget(cp.Modifiers.Annotations, ctorParamNoProp)
			} else {
				c.allTargets(cp.Modifiers.Annotations, true, cp.Mutable, !cls.Annotation)
			}
			if cp.IsProp && !cp.Mutable {
				c.setOnImmutable(cp.Modifiers.Annotations)
			}
		}
		c.headerParams = nil
		c.annotations(d.Dangling, cls, annotSite{kind: siteUnchecked})
		for _, e := range d.Entries {
			c.noAllTarget(e.Annotations, "elements other than properties")
			c.noFileTarget(e.Annotations)
			c.annotations(e.Annotations, cls, enumEntrySite)
		}
		for _, m := range d.Members {
			switch m := m.(type) {
			case *syntax.FunDecl, *syntax.PropDecl:
				c.declAnnotations(m, cls)
			case *syntax.InitBlock:
				c.annotations(m.Annotations, cls, initSite)
			case *syntax.SecondaryCtor:
				c.noAllTarget(m.Annotations, "elements other than properties")
				c.noReceiver(m.Annotations)
				c.noFileTarget(m.Annotations)
				c.publishedApi(&m.Modifiers)
				c.annotations(m.Annotations, cls, ctorSite)
				if m.Sym != nil {
					c.headerParams = m.Sym.Params
				}
				for _, p := range m.Params {
					c.noAllTarget(p.Annotations, ctorParamNoProp)
					c.noReceiver(p.Annotations)
					c.noParamTarget(p.Annotations)
					c.noFileTarget(p.Annotations)
					c.annotations(p.Annotations, cls, inHeader(paramSite))
				}
				c.headerParams = nil
			}
		}
	}
}

// setOnImmutable reports the annotations among as of a val with the
// use-site targets of a setter.
func (c *Checker) setOnImmutable(as []syntax.Annotation) {
	for _, a := range as {
		if a.Target == "set" || a.Target == "setparam" {
			c.errorf(a.P, "'@%s:' annotations could be applied only to mutable properties", a.Target)
		}
	}
}

// ctorParamNoProp is why @all: can't be on a constructor's parameter that
// isn't a property.
const ctorParamNoProp = "constructor parameters without corresponding property (consider adding val/var)"

// noAllTarget reports the annotations among as with the use-site target
// all, of a declaration that isn't a property, for the reason why
// (INAPPLICABLE_ALL_TARGET). An accessor's are reported by onAccessor.
func (c *Checker) noAllTarget(as []syntax.Annotation, why string) {
	for _, a := range as {
		if a.Target == "all" && !a.OnAccessor {
			c.errorf(a.P, "'@all:' annotations cannot be applied to %s", why)
		}
	}
}

// allTargets checks the annotations among as, of a property, with the
// use-site target all, which go to each of the property's elements that
// their class's targets allow: its constructor parameter, if ctorParam
// is set, the property, its field, if hasField is set, its getter, and
// its setter's parameter, if mutable is set. One that goes to none is an
// error (WRONG_ANNOTATION_TARGET_WITH_USE_SITE_TARGET), as is one, as an
// annotation without a use-site target, for a field but not for a
// property, of a property without a field (WRONG_ANNOTATION_TARGET).
func (c *Checker) allTargets(as []syntax.Annotation, ctorParam, mutable, hasField bool) {
	for _, a := range as {
		cls := cmp.Or(a.Class, a.Source)
		if a.Target != "all" || a.OnAccessor || cls == nil || !cls.TargetsKnown {
			continue
		}
		t := cls.Targets
		if t&types.TargetField != 0 && t&types.TargetProperty == 0 && !hasField {
			c.errorf(a.P, "this annotation is not applicable to target 'member property without backing field or delegate'")
			continue
		}
		if t&types.TargetProperty == 0 && (t&types.TargetField == 0 || !hasField) && t&types.TargetGetter == 0 &&
			(t&types.TargetValueParameter == 0 || !ctorParam && !mutable) {
			c.errorf(a.P, "this annotation is not applicable to target 'member property' and use-site target '@all'")
		}
	}
}

// noReceiver reports the annotations among as of a declaration with the
// use-site target receiver, which can only be on a receiver type.
func (c *Checker) noReceiver(as []syntax.Annotation) {
	for _, a := range as {
		if a.Target == "receiver" {
			c.errorf(a.P, "this annotation is not applicable to target 'declaration' and use-site target '@receiver'")
		}
	}
}

// noParamTarget reports the annotations among as with the use-site
// target param, of a declaration other than a primary constructor's
// parameter (INAPPLICABLE_PARAM_TARGET).
func (c *Checker) noParamTarget(as []syntax.Annotation) {
	for _, a := range as {
		if a.Target == "param" {
			c.errorf(a.P, "'@param:' annotations could be applied only to primary constructor parameters")
		}
	}
}

// noFileTarget reports the annotations among as with the use-site target
// file, of a declaration rather than the file: written after its package
// directive or another annotation (INAPPLICABLE_FILE_TARGET).
func (c *Checker) noFileTarget(as []syntax.Annotation) {
	for _, a := range as {
		if a.Target == "file" {
			c.errorf(a.P, "'@file:' annotations could be applied only to the file")
		}
	}
}

// onAccessor reports the annotations among a property's annotations as
// that have a use-site target but are written on an accessor, @get:A
// get(): kotlinc takes them as the accessor's, which has no use-site
// targets (INAPPLICABLE_TARGET_ON_PROPERTY).
func (c *Checker) onAccessor(as []syntax.Annotation) {
	for _, a := range as {
		if !a.OnAccessor {
			continue
		}
		switch a.Target {
		case "get", "set", "property", "setparam":
			c.errorf(a.P, "'@%s:' annotations could be applied only to property declarations", a.Target)
		case "param":
			c.errorf(a.P, "'@param:' annotations could be applied only to primary constructor parameters")
		case "file":
			c.errorf(a.P, "'@file:' annotations could be applied only to the file")
		default:
			c.errorf(a.P, "this annotation is not applicable to target 'getter' and use-site target '@%s'", a.Target)
		}
	}
}

// localUseSite reports annotation a, of a local variable or a catch
// block's parameter, if it has a use-site target: kotlinc reports all of
// them there but @delegate:.
func (c *Checker) localUseSite(a *syntax.Annotation, site annotSite) {
	switch a.Target {
	case "", "delegate":
	case "get", "set", "property", "setparam":
		c.errorf(a.P, "'@%s:' annotations could be applied only to property declarations", a.Target)
	case "param":
		c.errorf(a.P, "'@param:' annotations could be applied only to primary constructor parameters")
	case "file":
		c.errorf(a.P, "'@file:' annotations could be applied only to the file")
	case "all":
		c.errorf(a.P, "'@all:' annotations cannot be applied to local properties, only member or top-level properties are allowed")
	default:
		c.errorf(a.P, "this annotation is not applicable to target '%s' and use-site target '@%s'", site.name, a.Target)
	}
}

// fieldless reports the annotations of property d with the use-site
// target field when it has no backing field: a delegated property's
// (INAPPLICABLE_TARGET_PROPERTY_HAS_NO_BACKING_FIELD), or one whose
// accessors don't use one (WRONG_ANNOTATION_TARGET_WITH_USE_SITE_TARGET).
func (c *Checker) fieldless(d *syntax.PropDecl) {
	if d.Sym == nil || d.Expect || d.BackingField != nil || d.Lateinit || d.Const {
		return
	}
	ext := d.Getter != nil && d.Getter.External || d.Setter != nil && d.Setter.External
	if ext {
		return
	}
	for _, a := range d.Annotations {
		if a.Target != "field" {
			continue
		}
		switch {
		case d.Field != nil:
			c.errorf(a.P, "'@field:' annotations could be applied only to properties with backing fields")
		case !d.Sym.HasField():
			c.errorf(a.P, "this annotation is not applicable to target 'member property without backing field or delegate' and use-site target '@field'")
		}
	}
}

// repeatedSetparam reports an annotation @setparam:A of property d whose
// setter's parameter is annotated with A too, unless A is repeatable
// (REPEATED_ANNOTATION, on the property's).
func (c *Checker) repeatedSetparam(d *syntax.PropDecl) {
	if d.Setter == nil || len(d.Setter.Params) == 0 {
		return
	}
	on := d.Setter.Params[0].Annotations
	for _, a := range d.Annotations {
		cls := cmp.Or(a.Class, a.Source)
		if a.Target != "setparam" || cls == nil || repeatable(cls) {
			continue
		}
		for _, b := range on {
			if b.Target == "" && cmp.Or(b.Class, b.Source) == cls {
				c.errorf(a.P, "this annotation is not repeatable")
				break
			}
		}
	}
}

// dslMarkerTarget warns, as kotlinc does, of annotation a of class cls on
// site if cls is a DSL marker and site isn't a class, a typealias or a
// type, where the marker has no effect (DSL_MARKER_APPLIED_TO_WRONG_TARGET).
// kc leaves alone the annotations of expressions, among which kotlinc
// warns of a lambda's.
func (c *Checker) dslMarkerTarget(a *syntax.Annotation, cls *types.ClassSym, site annotSite) {
	switch site.kind {
	case siteUnchecked, siteClass, siteTypealias, siteType, siteExpr, siteInit:
		return
	}
	if !isDslMarker(cls) && !declaredDslMarker(cls) {
		return
	}
	target := site.name
	switch {
	case a.Accessor && a.Target == "set", a.Target == "set":
		target = "setter"
	case a.Accessor, a.Target == "get":
		target = "getter"
	case a.Target == "param", a.Target == "setparam":
		target = "value parameter"
	case a.Target == "field", a.Target == "delegate":
		target = "field"
	case a.Target == "file":
		target = "file"
	}
	c.warnf(a.P, "DSL_MARKER_APPLIED_TO_WRONG_TARGET", "applying DSL marker annotation '%s' to target '%s' has no effect. DSL marker annotations must only be applied to types. See https://youtrack.jetbrains.com/issue/KT-81567 for further information.", simpleName(cls), target)
}

// anyOverrideAnnotations warns, as kotlinc does, of the deprecations and
// opt-in markers on d, an override of equals, hashCode or toString of
// Any in class owner, which calls through Any don't report
// (POTENTIALLY_NON_REPORTED_ANNOTATION).
func (c *Checker) anyOverrideAnnotations(d *syntax.FunDecl, owner *types.ClassSym) {
	if !d.Override || owner == nil || d.Sym == nil || d.Recv != nil || len(d.Contexts) > 0 {
		return
	}
	switch ps := d.Sym.Params; d.Name {
	case "equals":
		if len(ps) != 1 || ps[0].Type.K != types.Any || !ps[0].Type.N {
			return
		}
	case "hashCode", "toString":
		if len(ps) != 0 {
			return
		}
	default:
		return
	}
	for i := range d.Annotations {
		a := &d.Annotations[i]
		cls := a.Class
		if cls == nil {
			cls = a.Source
		}
		if cls == nil || a.Target != "" {
			continue
		}
		switch {
		case cls.Name == "kotlin/Deprecated" || cls.Name == "kotlin/SinceKotlin" || cls.Name == "java/lang/Deprecated":
		case optInMarkerClass(cls):
		default:
			continue
		}
		c.warnf(a.P, "POTENTIALLY_NON_REPORTED_ANNOTATION", "Deprecations and opt-ins on a method overridden from 'Any' may not be reported.")
	}
}

// optInMarkerClass reports whether annotation class cls is annotated
// @RequiresOptIn.
func optInMarkerClass(cls *types.ClassSym) bool {
	if d, ok := cls.Decl.(*syntax.ClassDecl); ok {
		return d.Annotation && d.Find("", "RequiresOptIn") != nil
	}
	if cls.External {
		cls.Ensure()
		return cls.OptInLevel != ""
	}
	return false
}
