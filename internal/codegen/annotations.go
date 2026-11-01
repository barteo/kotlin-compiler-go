package codegen

import (
	"fmt"
	"slices"

	cf "github.com/barteo/kotlin-compiler-go/internal/classfile"
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// Annotations, as kotlinc writes them: the program's annotations that
// class files keep (resolved by the checker), on their targets, and
// org.jetbrains.annotations.NotNull and Nullable on the types of fields,
// and of the results and parameters of methods that aren't private, for
// Java code.

const (
	notNull  = "Lorg/jetbrains/annotations/NotNull;"
	nullable = "Lorg/jetbrains/annotations/Nullable;"
)

// adder adds an annotation to a class, member or parameter.
type adder func(a cf.Annotation, visible bool)

// placement selects, for a target, the annotations written there: those
// with its use-site target, and, if dflt is set, those without one whose
// class dflt accepts.
type placement struct {
	site string
	dflt func(*types.ClassSym) bool
}

// all is the placement of annotations without a use-site target, on a
// class, function or parameter.
var all = placement{dflt: func(*types.ClassSym) bool { return true }}

func site(s string) placement { return placement{site: s} }

// writeAnnotations adds the annotations among as that p places. Those of
// a repeatable annotation class with a container, as Compose's
// @Preview's Preview.Container, go in one of its container, as kotlinc
// writes them.
func writeAnnotations(add adder, as []syntax.Annotation, p placement) bool {
	var placed []*syntax.Annotation
	count := map[*types.ClassSym]int{}
	for i := range as {
		a := &as[i]
		if a.Class == nil {
			continue
		}
		switch {
		case a.Target == "" && p.dflt != nil && p.dflt(a.Class):
		case a.Target != "" && a.Target == p.site:
		default:
			continue
		}
		placed = append(placed, a)
		count[a.Class]++
	}
	contained := map[*types.ClassSym]bool{}
	for _, a := range placed {
		c := a.Class.Container
		if count[a.Class] < 2 || c == nil {
			add(annotationOf(a), a.Class.Retention == types.RetentionRuntime)
			continue
		}
		if contained[a.Class] {
			continue
		}
		contained[a.Class] = true
		all := cf.ElementValue{Tag: '['}
		for _, b := range placed {
			if b.Class == a.Class {
				ann := annotationOf(b)
				all.Elems = append(all.Elems, cf.ElementValue{Tag: '@', Ann: &ann})
			}
		}
		c.Ensure()
		add(cf.Annotation{Type: "L" + c.Name + ";", Values: []cf.ElementPair{{Name: "value", Value: all}}}, c.Retention == types.RetentionRuntime)
	}
	return len(placed) > 0
}

// annotationOf converts a checked annotation.
func annotationOf(a *syntax.Annotation) cf.Annotation {
	out := cf.Annotation{Type: "L" + a.Class.Name + ";"}
	for _, v := range a.Values {
		var ev cf.ElementValue
		if v.Array {
			ev = cf.ElementValue{Tag: '['}
			elem := types.VarargElem(v.Type)
			for _, e := range v.Elems {
				ev.Elems = append(ev.Elems, elementOf(elem, e))
			}
		} else {
			ev = elementOf(v.Type, v.Elems[0])
		}
		out.Values = append(out.Values, cf.ElementPair{Name: v.Name, Value: ev})
	}
	return out
}

// elementOf converts a value of an annotation's argument, of type t.
func elementOf(t types.Type, e syntax.AnnotationElem) cf.ElementValue {
	if e.Nested != nil {
		a := annotationOf(e.Nested)
		return cf.ElementValue{Tag: '@', Ann: &a}
	}
	x := e.X
	t = t.NonNull()
	if cl, ok := x.(*syntax.ClassLit); ok {
		return cf.ElementValue{Tag: 'c', Class: desc(types.Erase(cl.Class.NonNull()))}
	}
	if t.K == types.Class && isEnum(t.C) {
		name := ""
		switch x := x.(type) {
		case *syntax.Member:
			name = x.Name
		case *syntax.Ident:
			name = x.Name
			if x.Member != nil {
				name = x.Member.Name
			}
		}
		return cf.ElementValue{Tag: 'e', Enum: [2]string{"L" + t.C.Name + ";", name}}
	}
	if t.K == types.Class && types.IsValue(t) {
		// An unsigned integer, as its signed bits: UInt is an int.
		v, ok := unsignedConst(x, 0)
		if !ok {
			panic(fmt.Sprintf("%d:%d: kc can't compute this annotation argument", x.Pos().Line, x.Pos().Col))
		}
		switch t.C.Name {
		case "kotlin/UByte":
			return cf.ElementValue{Tag: 'B', Const: int64(int8(v))}
		case "kotlin/UShort":
			return cf.ElementValue{Tag: 'S', Const: int64(int16(v))}
		case "kotlin/UInt":
			return cf.ElementValue{Tag: 'I', Const: int64(int32(v))}
		case "kotlin/ULong":
			return cf.ElementValue{Tag: 'J', Const: v}
		}
	}
	v, ok := constOf(x, 0)
	if !ok {
		panic(fmt.Sprintf("%d:%d: kc can't compute this annotation argument", x.Pos().Line, x.Pos().Col))
	}
	num := func() int64 {
		switch v := v.(type) {
		case int64:
			return v
		case float64:
			return int64(v)
		case float32:
			return int64(v)
		}
		panic(fmt.Sprintf("%d:%d: annotation argument %v isn't a number", x.Pos().Line, x.Pos().Col, v))
	}
	switch t.K {
	case types.String:
		s, _ := v.(string)
		return cf.ElementValue{Tag: 's', Const: s}
	case types.Bool:
		return cf.ElementValue{Tag: 'Z', Const: num()}
	case types.Byte:
		return cf.ElementValue{Tag: 'B', Const: num()}
	case types.Char:
		return cf.ElementValue{Tag: 'C', Const: num()}
	case types.Short:
		return cf.ElementValue{Tag: 'S', Const: num()}
	case types.Int:
		return cf.ElementValue{Tag: 'I', Const: num()}
	case types.Long:
		return cf.ElementValue{Tag: 'J', Const: num()}
	case types.Float:
		switch v := v.(type) {
		case float32:
			return cf.ElementValue{Tag: 'F', Const: v}
		case float64:
			return cf.ElementValue{Tag: 'F', Const: float32(v)}
		}
		return cf.ElementValue{Tag: 'F', Const: float32(num())}
	case types.Double:
		switch v := v.(type) {
		case float64:
			return cf.ElementValue{Tag: 'D', Const: v}
		case float32:
			return cf.ElementValue{Tag: 'D', Const: float64(v)}
		}
		return cf.ElementValue{Tag: 'D', Const: float64(num())}
	}
	panic(fmt.Sprintf("%d:%d: unsupported annotation argument of type %s", x.Pos().Line, x.Pos().Col, t))
}

// isEnum reports whether cls is an enum class, of the program or of the
// class path.
func isEnum(cls *types.ClassSym) bool {
	if cls.Entries != nil {
		return true
	}
	cls.Ensure()
	return cls.Super != nil && cls.Super.Name == "java/lang/Enum"
}

// nullability adds kotlinc's NotNull or Nullable annotation for a value
// of type t, if it is a reference that isn't of a platform type or of a
// type parameter that may be nullable.
func nullability(add adder, t types.Type) {
	if t.P || t.K == types.Unit || t.K == types.Invalid {
		return
	}
	if t.K == types.Nothing && !t.N {
		add(cf.Annotation{Type: notNull}, false) // a java.lang.Void that never is
		return
	}
	d, ok := types.Desc(t)
	if !ok || d[0] != 'L' && d[0] != '[' {
		return
	}
	switch {
	case t.IsParam() && types.MayBeNull(t) && !t.N:
		// T, which may stand for a nullable type: neither
	case types.MayBeNull(t):
		add(cf.Annotation{Type: nullable}, false)
	default:
		add(cf.Annotation{Type: notNull}, false)
	}
}

// methodAdder returns the adder of a method, and paramAdder that of its
// parameter i.
func methodAdder(m cf.MemberRef) adder { return m.Annotate }

func paramAdder(m cf.MemberRef, i int) adder {
	return func(a cf.Annotation, visible bool) { m.AnnotateParam(i, a, visible) }
}

// annotateFun writes the annotations of function fd on its method m,
// and of its parameters, and the nullability annotations of a method
// that isn't private. extra are more annotations for the method, from a
// property's use-site targets.
func (g *gen) annotateFun(m cf.MemberRef, fd *syntax.FunDecl, private bool, extra ...[]syntax.Annotation) {
	add := methodAdder(m)
	writeAnnotations(add, fd.Annotations, all)
	for _, as := range extra {
		writeAnnotations(add, as, all)
	}
	sym := fd.Sym
	if deprecated(fd.Annotations) {
		m.Deprecate()
	}
	if !private {
		nullability(add, sym.Ret)
	}
	i := 0
	if g.implsThis {
		// A DefaultImpls method, which takes the instance first.
		nullability(paramAdder(m, 0), g.owner.This())
		i = 1
	}
	if fd.RecvSym != nil {
		writeAnnotations(paramAdder(m, i), fd.RecvAnnotations, site("receiver"))
		if !private {
			nullability(paramAdder(m, i), fd.RecvSym.Type)
		}
		i++
	}
	for j, p := range sym.Params {
		if j < len(fd.Params) {
			writeAnnotations(paramAdder(m, i+j), fd.Params[j].Annotations, all)
		}
		if !private {
			nullability(paramAdder(m, i+j), p.Type)
		}
	}
}

// deprecated reports whether annotations include @Deprecated, which
// kotlinc also marks with the Deprecated attribute.
func deprecated(as []syntax.Annotation) bool {
	for i := range as {
		if c := as[i].Class; c != nil && c.Name == "kotlin/Deprecated" && as[i].Target == "" {
			return true
		}
	}
	return false
}

// Property annotations without a use-site target go to the property,
// else to its field, as the annotation class's targets allow.
func onProperty(c *types.ClassSym) bool { return c.Targets&types.TargetProperty != 0 }
func onField(c *types.ClassSym) bool {
	return c.Targets&types.TargetProperty == 0 && c.Targets&types.TargetField != 0
}

// A constructor's property parameter's annotations without a use-site
// target go to the parameter and the property, as Kotlin 2.2 places
// them, or else to the field.
func onCtorParam(c *types.ClassSym) bool    { return c.Targets&types.TargetValueParameter != 0 }
func onCtorProperty(c *types.ClassSym) bool { return onProperty(c) }
func onCtorField(c *types.ClassSym) bool {
	return c.Targets&(types.TargetValueParameter|types.TargetProperty) == 0 && c.Targets&types.TargetField != 0
}

// propertyAnnotations writes the annotations of a property that go to its
// property target on its synthetic method getter$annotations, named after
// its getter's JVM name, getter, which takes an extension property's
// receiver, as kotlinc does: also if they are all SOURCE annotations,
// which it doesn't keep. It returns the method's name and descriptor,
// which metadata refers to, or "".
func (g *gen) propertyAnnotations(c *cf.Class, getter string, recv *types.Type, as []syntax.Annotation, p placement, private bool) (string, string) {
	n := 0
	for i := range as {
		a := &as[i]
		cls := a.Class
		if cls == nil {
			cls = a.Source
		}
		if cls != nil && (a.Target == p.site || a.Target == "" && p.dflt != nil && p.dflt(cls)) {
			n++
		}
	}
	if n == 0 {
		return "", ""
	}
	mname := getter + "$annotations"
	d := "()V"
	if recv != nil {
		d = "(" + desc(*recv) + ")V"
	}
	access := uint16(cf.AccPublic | cf.AccStatic | cf.AccSynthetic)
	if private {
		access = cf.AccPrivate | cf.AccStatic | cf.AccSynthetic
	}
	if c.Access&cf.AccInterface != 0 {
		access = cf.AccPublic | cf.AccStatic | cf.AccSynthetic
	}
	code := c.NewMethod(access, mname, d)
	if recv != nil {
		code.AllocLocal(desc(*recv))
	}
	code.Op(cf.RETURN)
	if err := code.Err(); err != nil {
		panic(err)
	}
	writeAnnotations(methodAdder(c.LastMethod()), as, p)
	c.LastMethod().Deprecate() // as kotlinc marks them
	return mname, d
}

// annotationClassMeta writes the Java meta-annotations of an annotation
// class of the program, as kotlinc does: java.lang.annotation.Retention
// from its retention, Target from its @Target, and Documented for
// @MustBeDocumented.
func annotationClassMeta(c *cf.Class, cd *syntax.ClassDecl) {
	cls := cd.Sym
	policy := map[types.Retention]string{types.RetentionSource: "SOURCE", types.RetentionBinary: "CLASS", types.RetentionRuntime: "RUNTIME"}[cls.Retention]
	c.Annotate(cf.Annotation{Type: "Ljava/lang/annotation/Retention;", Values: []cf.ElementPair{{Name: "value",
		Value: cf.ElementValue{Tag: 'e', Enum: [2]string{"Ljava/lang/annotation/RetentionPolicy;", policy}}}}}, true)
	if cd.Find("", "Target") != nil {
		var elems []cf.ElementValue
		add := func(name string) {
			for _, e := range elems {
				if e.Enum[1] == name {
					return
				}
			}
			elems = append(elems, cf.ElementValue{Tag: 'e', Enum: [2]string{"Ljava/lang/annotation/ElementType;", name}})
		}
		for _, t := range []struct {
			t    types.Targets
			java string
		}{
			// In ElementType's order, as kotlinc writes them.
			{types.TargetClass, "TYPE"}, {types.TargetField, "FIELD"}, {types.TargetFunction, "METHOD"},
			{types.TargetGetter, "METHOD"}, {types.TargetSetter, "METHOD"}, {types.TargetValueParameter, "PARAMETER"},
			{types.TargetConstructor, "CONSTRUCTOR"}, {types.TargetLocalVariable, "LOCAL_VARIABLE"},
			{types.TargetAnnotationClass, "ANNOTATION_TYPE"}, {types.TargetTypeParameter, "TYPE_PARAMETER"},
			{types.TargetType, "TYPE_USE"},
		} {
			if cls.Targets&t.t != 0 {
				add(t.java)
			}
		}
		c.Annotate(cf.Annotation{Type: "Ljava/lang/annotation/Target;", Values: []cf.ElementPair{{Name: "value",
			Value: cf.ElementValue{Tag: '[', Elems: elems}}}}, true)
	}
	if cd.Find("", "MustBeDocumented") != nil {
		c.Annotate(cf.Annotation{Type: "Ljava/lang/annotation/Documented;"}, true)
	}
}

// annotationDefault returns the AnnotationDefault of an annotation class's
// parameter with a default value: its value as an element.
func annotationDefault(t types.Type, x syntax.Expr) (cf.ElementValue, bool) {
	defer func() { recover() }()
	if t.IsArray() || t.K == types.Class && types.IsUnsignedArray(t.C) {
		ev := cf.ElementValue{Tag: '['}
		elems, ok := syntax.ArrayElems(x)
		if !ok {
			elems = []syntax.Expr{x}
		}
		for _, e := range syntax.FlattenSpreads(elems) {
			v, ok := annotationDefault(types.VarargElem(t), e)
			if !ok {
				return cf.ElementValue{}, false
			}
			ev.Elems = append(ev.Elems, v)
		}
		return ev, true
	}
	if call, ok := x.(*syntax.Call); ok && call.Ctor != nil && call.Ctor.Retention != types.NoRetention {
		a, ok := nestedDefault(call)
		return cf.ElementValue{Tag: '@', Ann: &a}, ok
	}
	return elementOf(t, syntax.AnnotationElem{X: x}), true
}

// nestedDefault returns the annotation that call, an annotation class's
// instantiation in a default value, creates: its arguments, as kotlinc
// writes them, without the omitted ones.
func nestedDefault(call *syntax.Call) (cf.Annotation, bool) {
	a := cf.Annotation{Type: "L" + call.Ctor.Name + ";"}
	if call.Sym == nil {
		return a, false
	}
	params := call.Sym.Params
	args := make([][]syntax.Expr, len(params))
	for i, arg := range call.Args {
		j := i
		if call.ArgMap != nil {
			j = call.ArgMap[i]
		}
		if j < 0 || j >= len(params) {
			return a, false
		}
		if sp, ok := arg.(*syntax.Spread); ok && params[j].Vararg {
			elems, ok := syntax.ArrayElems(sp.X)
			if !ok {
				return a, false
			}
			args[j] = append(args[j], elems...)
			continue
		}
		args[j] = append(args[j], arg)
	}
	for j, p := range params {
		if args[j] == nil {
			continue
		}
		var v cf.ElementValue
		ok := true
		if p.Vararg {
			v, ok = annotationDefault(p.Type, &syntax.Call{Literal: true, Args: args[j]})
		} else {
			v, ok = annotationDefault(p.Type, args[j][0])
		}
		if !ok {
			return a, false
		}
		a.Values = append(a.Values, cf.ElementPair{Name: p.Name, Value: v})
	}
	return a, true
}

// propAnnotations writes the $annotations method of property p of class
// c, if it has annotations for its property target, and records it for
// the property's metadata.
func (g *gen) propAnnotations(c *cf.Class, p *syntax.PropDecl, recv *types.Type, private bool) {
	getter := types.GetterName(p.Name)
	switch {
	case p.Getter != nil && p.Getter.Sym != nil:
		getter = p.Getter.Sym.JVM()
	case p.Sym != nil:
		getter = types.GetterJVM(p.Sym)
	}
	name, d := g.propertyAnnotations(c, getter, recv, p.Annotations, placement{site: "property", dflt: onProperty}, private)
	if name != "" && p.Sym != nil {
		g.shared.annotated[p.Sym] = [2]string{name, d}
	}
}

// accessorAnnotations writes the annotations of property p for its getter
// (site "get") or setter ("set") on the accessor method m, which
// function generated from the accessor's own declaration, for a
// setter's parameter ("setparam") and for an extension's receiver.
func (g *gen) accessorAnnotations(m cf.MemberRef, p *syntax.PropDecl, site string) {
	writeAnnotations(methodAdder(m), p.Annotations, placement{site: site})
	i := 0
	if p.Recv != nil {
		writeAnnotations(paramAdder(m, 0), p.RecvAnnotations, placement{site: "receiver"})
		i = 1
	}
	if site == "set" {
		writeAnnotations(paramAdder(m, i), p.Annotations, placement{site: "setparam"})
	}
	if deprecated(p.Annotations) {
		m.Deprecate()
	}
}

// defaultAccessorAnnotations writes the annotations of a default getter
// (site "get") or setter ("set") m of a property of type t, with its
// nullability annotations.
func (g *gen) defaultAccessorAnnotations(m cf.MemberRef, as []syntax.Annotation, site string, t types.Type) {
	add := methodAdder(m)
	writeAnnotations(add, as, placement{site: site})
	if deprecated(as) {
		m.Deprecate()
	}
	if site == "get" {
		nullability(add, t)
		return
	}
	writeAnnotations(paramAdder(m, 0), as, placement{site: "setparam"})
	nullability(paramAdder(m, 0), t)
}

// fieldAnnotations writes the annotations of a property's field that p
// places there, and its nullability annotation.
// fieldModifiers returns the access flags that a field's annotations as
// stand for: ACC_VOLATILE for @Volatile (kotlin.jvm's, or
// kotlin.concurrent's, an alias of it) and ACC_TRANSIENT for @Transient.
// Both have SOURCE retention, so only the flags keep them.
func fieldModifiers(as []syntax.Annotation) uint16 {
	acc := uint16(0)
	for _, a := range as {
		cls := a.Source
		if cls == nil || a.Target != "" && a.Target != "field" {
			continue
		}
		switch cls.Name {
		case "kotlin/jvm/Volatile":
			acc |= cf.AccVolatile
		case "kotlin/jvm/Transient":
			acc |= cf.AccTransient
		}
	}
	return acc
}

// methodModifiers returns the access flags that a function's annotations
// as stand for: ACC_SYNCHRONIZED for @Synchronized and ACC_STRICT for
// @Strictfp, which have SOURCE retention.
func methodModifiers(as []syntax.Annotation) uint16 {
	acc := uint16(0)
	for _, a := range as {
		cls := a.Source
		if cls == nil || a.Target != "" {
			continue
		}
		switch cls.Name {
		case "kotlin/jvm/Synchronized":
			acc |= cf.AccSynchronized
		case "kotlin/jvm/Strictfp":
			acc |= cf.AccStrict
		}
	}
	return acc
}

func (g *gen) fieldAnnotations(f cf.MemberRef, as []syntax.Annotation, t types.Type, p placement) {
	writeAnnotations(f.Annotate, as, p)
	if deprecated(as) {
		f.Deprecate()
	}
	nullability(f.Annotate, t)
}

// stubAnnotations writes the annotations of function fd on a stub that
// calls it (a @JvmOverloads overload or a @JvmStatic method), as kotlinc
// copies them, and the nullability annotations of the stub's result and
// parameters: params are those it takes, after a receiver if recv.
func stubAnnotations(m cf.MemberRef, fd *syntax.FunDecl, recv bool, params []*types.VarSym) {
	add := methodAdder(m)
	writeAnnotations(add, fd.Annotations, all)
	if deprecated(fd.Annotations) {
		m.Deprecate()
	}
	if synthetic(&fd.Modifiers) != 0 || reifiedInline(fd) {
		return // no nullability annotations on synthetic methods
	}
	nullability(add, fd.Sym.Ret)
	i := 0
	if recv {
		nullability(paramAdder(m, 0), fd.Sym.Recv)
		i = 1
	}
	for j, p := range params {
		for k, q := range fd.Sym.Params {
			if q == p && k < len(fd.Params) {
				writeAnnotations(paramAdder(m, i+j), fd.Params[k].Annotations, all)
			}
		}
		nullability(paramAdder(m, i+j), p.Type)
	}
}

// classPropAnnotations returns the annotations of a class's property, and
// their placements on its field and property targets: a constructor's
// property parameter places those without a use-site target otherwise.
func classPropAnnotations(p *types.VarSym) (as []syntax.Annotation, field, prop placement) {
	switch d := p.Decl.(type) {
	case *syntax.PropDecl:
		return d.Annotations, placement{"field", onField}, placement{"property", onProperty}
	case *syntax.CtorParam:
		return d.Modifiers.Annotations, placement{"field", onCtorField}, placement{"property", onCtorProperty}
	}
	return nil, site("field"), site("property")
}

// annotateCtor writes the annotations of a constructor m and of its
// parameters, the first of which is after skip synthetic ones, with
// their nullability annotations if it isn't private.
func (g *gen) annotateCtor(m cf.MemberRef, mods *syntax.Modifiers, params []*types.VarSym, decls []*syntax.Param, ctorParams []*syntax.CtorParam, skip int, private bool) {
	m.SyntheticParams(skip)
	writeAnnotations(methodAdder(m), mods.Annotations, all)
	if deprecated(mods.Annotations) {
		m.Deprecate()
	}
	for i, p := range params {
		add := paramAdder(m, skip+i)
		switch {
		case i < len(ctorParams):
			cp := ctorParams[i]
			pl := placement{"param", onCtorParam}
			if !cp.IsProp {
				pl = placement{"param", all.dflt}
			}
			writeAnnotations(add, cp.Modifiers.Annotations, pl)
		case i < len(decls):
			writeAnnotations(add, decls[i].Annotations, all)
		}
		if !private {
			nullability(add, p.Type)
		}
	}
}

// bridgeAnnotations writes the annotations of a function on a bridge
// method that calls it, as kotlinc copies them, without nullability
// annotations, a bridge being synthetic, and @Composable, which Compose's
// plugin leaves to the function.
func bridgeAnnotations(m cf.MemberRef, as []syntax.Annotation) {
	as = slices.DeleteFunc(slices.Clone(as), func(a syntax.Annotation) bool {
		return a.Class != nil && a.Class.Name == "androidx/compose/runtime/Composable"
	})
	writeAnnotations(methodAdder(m), as, all)
	if deprecated(as) {
		m.Deprecate()
	}
}

// unsignedConst returns the bits of a constant of an unsigned integer
// type: an unsigned literal, or a const val of one.
func unsignedConst(e syntax.Expr, depth int) (int64, bool) {
	if depth > 32 {
		return 0, false
	}
	ref := func(p *types.VarSym) (int64, bool) {
		if p == nil {
			return 0, false
		}
		if v, ok := p.Constant.(int64); ok {
			return v, true // a compiled Kotlin const val
		}
		if pd, ok := p.Decl.(*syntax.PropDecl); ok && p.Const {
			return unsignedConst(pd.Init, depth+1)
		}
		return 0, false
	}
	switch e := e.(type) {
	case *syntax.IntLit:
		return e.Val, e.Unsigned
	case *syntax.Ident:
		if e.Sym == nil && e.Member != nil {
			return unsignedConst(e.Member, depth+1)
		}
		return ref(e.Sym)
	case *syntax.Member:
		if e.Static != nil && e.Field != nil {
			return ref(e.Field)
		}
		return ref(e.Prop)
	}
	return 0, false
}
