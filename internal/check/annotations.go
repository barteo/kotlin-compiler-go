package check

import (
	"slices"

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
	seen := map[repeatKey]bool{}
	for i := range as {
		a := &as[i]
		if a.Name == "" {
			continue // @[A B]
		}
		cls := c.lookupClass(a.Name)
		if cls == nil {
			continue
		}
		cls.Ensure()
		if cls.Retention != types.NoRetention {
			c.annotationTarget(a, cls, site)
			c.repeated(a, cls, site, seen)
		}
		if ps := cls.TypeParams; cls.Annotation && len(a.TypeArgs) != len(ps) && c.lookupAlias(a.Name) == nil {
			// Unlike a call, an annotation entry doesn't infer its
			// type arguments; a typealias may give them.
			if len(ps) == 0 {
				c.errorf(a.P, "no type arguments expected for annotation class %s", cls.DisplayName())
			} else {
				c.errorf(a.P, "%s expected for annotation class %s%s", countArgs(len(ps)), cls.DisplayName(), types.TypeParamsString(ps))
			}
		}
		source := cls.Retention == types.RetentionSource
		if source {
			a.Source = cls
		}
		if cls.Retention == types.NoRetention || source && !cls.Annotation {
			continue
		}
		if !c.u.ext.kotlin && !cls.Annotation {
			// Without the standard library, @Deprecated would be
			// java.lang.Deprecated: only the program's are written.
			continue
		}
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
			p := a.P
			if len(arg.Toks) > 0 {
				p = arg.Toks[0].Pos
			}
			args = append(args, annotArg{name: arg.Name, elems: elems, array: array, pos: p})
		}
		if !ok {
			continue
		}
		saved := c.state
		c.state = state{scope: &scope{}, staticCls: owner}
		values, ok := c.annotationValues(cls, args, a.P)
		c.state = saved
		if ok && !source {
			// Class files keep it.
			a.Class, a.Values = cls, values
		}
	}
}

// annotationParams returns the parameters of annotation class cls: its
// constructor's for a Kotlin class, its methods' for a Java one, whose
// value method takes a positional argument. A Java method's Class<T> is
// a KClass<T> to Kotlin.
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
				ps = append(ps, &types.VarSym{Name: f.Name, Type: c.kclassOfJava(f.Ret), Kind: types.Param})
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
	if t.C.Name != "java/lang/Class" || t.A == nil {
		return t
	}
	return types.GenericType(kclass, t.A.List).Platform()
}

// annotationValues maps the arguments of an annotation of class cls to its
// parameters, and checks them: positional ones in order, until a vararg
// parameter takes the rest, then named ones.
func (c *Checker) annotationValues(cls *types.ClassSym, args []annotArg, p syntax.Pos) ([]syntax.AnnotationValue, bool) {
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
				values = append(values, syntax.AnnotationValue{Name: v.Name, Type: v.Type, Array: true})
			case !v.Default && cls.Annotation:
				c.errorf(p, "no value passed for parameter '%s'", v.Name)
				ok = false
			}
			continue
		}
		t := star.Apply(v.Type)
		val := syntax.AnnotationValue{Name: v.Name, Type: t, Array: t.IsArray() || t.K == types.Class && types.IsUnsignedArray(t.C)}
		elem := t
		if val.Array {
			elem = types.VarargElem(t)
		}
		for _, arg := range byParam[j] {
			elems := arg.elems
			if val.Array && !arg.array && (!v.Vararg || arg.name != "") && len(elems) == 1 {
				// arrayOf(a, b) or [a, b] for an array parameter, or a
				// vararg one by name: ReplaceWith(imports = ["a.b"])
				if inner, ok := syntax.ArrayElems(elems[0]); ok {
					elems = inner
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
		}
		values = append(values, val)
	}
	return values, ok
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
			values, ok := c.annotationValues(t.C, args, call.Pos())
			return syntax.AnnotationElem{Nested: &syntax.Annotation{P: call.Pos(), Class: t.C, Values: values}}, ok
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
			c.noReceiver(p.Annotations)
			c.annotations(p.Annotations, owner, paramSite)
		}
	}
	fun := func(f *syntax.FunDecl, site annotSite) {
		if f == nil {
			return
		}
		c.annotations(f.Annotations, owner, site)
		c.annotations(f.RecvAnnotations, owner, annotSite{})
		if f.Property == "" {
			params(f.Contexts) // an accessor's are its property's
		}
		params(f.Params)
	}
	switch d := d.(type) {
	case *syntax.FunDecl:
		c.noReceiver(d.Annotations)
		fun(d, funSite(owner))
	case *syntax.PropDecl:
		c.noReceiver(d.Annotations)
		c.annotations(d.Annotations, owner, propSite(owner))
		c.annotations(d.RecvAnnotations, owner, annotSite{})
		params(d.Contexts)
		fun(d.Getter, annotSite{siteUnchecked, types.TargetGetter, "getter"})
		fun(d.Setter, annotSite{siteUnchecked, types.TargetSetter, "setter"})
	case *syntax.TypeAliasDecl:
		c.annotations(d.Annotations, owner, typealiasSite)
	case *syntax.ClassDecl:
		cls := d.Sym
		c.noReceiver(d.Annotations)
		c.noReceiver(d.CtorMods.Annotations)
		c.annotations(d.Annotations, cls, classSite(d))
		c.annotations(d.CtorMods.Annotations, cls, ctorSite)
		for _, cp := range d.CtorParams {
			c.noReceiver(cp.Modifiers.Annotations)
			c.annotations(cp.Modifiers.Annotations, cls, ctorParamSite(cls, cp))
		}
		for _, e := range d.Entries {
			c.annotations(e.Annotations, cls, enumEntrySite)
		}
		for _, m := range d.Members {
			switch m := m.(type) {
			case *syntax.FunDecl, *syntax.PropDecl:
				c.declAnnotations(m, cls)
			case *syntax.SecondaryCtor:
				c.noReceiver(m.Annotations)
				c.annotations(m.Annotations, cls, ctorSite)
				for _, p := range m.Params {
					c.noReceiver(p.Annotations)
					c.annotations(p.Annotations, cls, paramSite)
				}
			}
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
