package check

import (
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/classfile"
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// Annotation classes are declared, and their parameters checked, but kc
// ignores annotations: an annotation class compiles to an annotation
// interface whose abstract methods are its parameters.

// throws returns the classes that @Throws(A::class, ...) among
// modifiers m names, by internal name, for the method's Exceptions
// attribute. A class it can't find is left out.
func (c *Checker) throws(m *syntax.Modifiers) []string {
	a := m.Find("", "Throws")
	if a == nil {
		return nil
	}
	var out []string
	for _, arg := range a.Args {
		var name strings.Builder
		for _, t := range arg.Toks {
			if t.Kind == syntax.COLON_COLON {
				break
			}
			name.WriteString(t.Text)
		}
		if cls := c.lookupClass(name.String()); cls != nil {
			out = append(out, cls.Name)
		}
	}
	return out
}

// declaredAnnotation returns the retention and targets of an annotation
// class of the program, from its @Retention and @Target: RUNTIME and the
// default targets without them.
func declaredAnnotation(m *syntax.Modifiers) (types.Retention, types.Targets) {
	ret, targets := types.RetentionRuntime, types.DefaultTargets
	names := func(a *syntax.Annotation) []string {
		var out []string
		for _, arg := range a.Args {
			for _, t := range arg.Toks {
				if t.Kind == syntax.IDENT {
					out = append(out, t.Text)
				}
			}
		}
		return out
	}
	for i := range m.Annotations {
		a := &m.Annotations[i]
		switch {
		case a.Target != "":
		case a.Name == "Retention" || a.Name == "kotlin.annotation.Retention":
			for _, n := range names(a) {
				if r := types.RetentionOf(n); r != types.NoRetention {
					ret = r
				}
			}
		case a.Name == "Target" || a.Name == "kotlin.annotation.Target":
			targets = types.KotlinTargets(names(a))
		}
	}
	return ret, targets
}

// annotationClass checks the declaration of an annotation class, with
// kotlinc's messages.
func (c *Checker) annotationClass(d *syntax.ClassDecl) {
	switch {
	case d.Interface:
		c.errorf(d.P, "modifier 'annotation' is not applicable to 'interface'")
	case d.Object:
		c.errorf(d.P, "modifier 'annotation' is not applicable to 'object'")
	}
	for _, m := range []struct {
		set  bool
		name string
	}{{d.Open, "open"}, {d.Abstract, "abstract"}, {d.Sealed, "sealed"}, {d.Data, "data"}, {d.Enum, "enum"}} {
		if m.set {
			c.errorf(d.P, "modifier '%s' is not applicable to 'annotation class'", m.name)
		}
	}
	if len(d.Supers) > 0 {
		c.errorf(d.Supers[0].Type.P, "annotation class cannot have supertypes")
	}
	for _, m := range d.Members {
		var p syntax.Pos
		switch m := m.(type) {
		case *syntax.ClassDecl:
			continue // nested classes are allowed
		case *syntax.FunDecl:
			p = m.P
		case *syntax.PropDecl:
			p = m.P
		case *syntax.InitBlock:
			p = m.P
		case *syntax.SecondaryCtor:
			p = m.P
		}
		c.errorf(p, "members are prohibited in annotation classes")
	}
}

// annotationParam checks a parameter of an annotation class's
// constructor, which becomes one of its abstract methods.
func (c *Checker) annotationParam(cls *types.ClassSym, cp *syntax.CtorParam) {
	switch {
	case !cp.IsProp:
		c.errorf(cp.P, "'val' keyword is missing in annotation parameter")
	case cp.Mutable:
		c.errorf(cp.P, "an annotation parameter cannot be 'var'")
	}
	switch t := cp.Sym.Type; {
	case t.K == types.Invalid:
	case t.N:
		c.errorf(cp.Type.P, "annotation parameters cannot be nullable")
	case !annotationType(t, true):
		c.errorf(cp.Type.P, "invalid type of annotation member")
	}
	cls.AnnotationParams = append(cls.AnnotationParams, cp.Sym)
}

// annotationType reports whether an annotation parameter can have type
// t: a primitive or unsigned integer type, String, an enum, KClass, an
// annotation, or an array of one of them (an unsigned array, as UIntArray,
// of unsigned integers).
func annotationType(t types.Type, arrays bool) bool {
	if t.N {
		return false
	}
	switch t.K {
	case types.Bool, types.Int, types.Long, types.Double, types.Char, types.Float, types.Byte, types.Short, types.String:
		return true
	case types.Class:
		cls := t.C
		switch {
		case cls.Array:
			return arrays && annotationType(types.ArrayElem(t), false) && !isUnsigned(types.ArrayElem(t))
		case types.IsUnsignedArray(cls):
			return arrays
		case isUnsigned(t):
			return true
		case cls.Annotation, cls.Entries != nil, cls.KotlinName == "kotlin/reflect/KClass":
			return true
		}
		cls.Ensure()
		return cls.Super != nil && cls.Super.Name == "java/lang/Enum"
	}
	return false
}

// annotationConstruct checks an instantiation of a Kotlin annotation
// class, A(x): its constructor takes the annotation's parameters, and
// codegen writes a class that implements the annotation, as kotlinc
// does. A library's annotation's omitted arguments are the
// AnnotationDefault values of its class file.
func (c *Checker) annotationConstruct(x *syntax.Call, cls *types.ClassSym, p syntax.Pos, want types.Type, sink *system) types.Type {
	if cls.Java != nil {
		c.unsupported(p, "annotation instantiation", "creating instances of Java annotation classes is not supported")
		c.args(x, nil)
		return types.InvalidType
	}
	params := c.annotationParams(cls)
	f := &types.FunSym{Name: "<init>", Owner: cls, Params: params, Ret: cls.This(), Resolved: true}
	x.Ctor = cls
	saved := c.ctorArgs
	c.ctorArgs = true
	defer func() { c.ctorArgs = saved }()
	t := c.resolve(x, []callee{{f: f, tparams: cls.TypeParams}}, want, sink)
	if x.Sym == f && cls.External {
		passed := make([]bool, len(params))
		for i := range x.Args {
			j := i
			if x.ArgMap != nil {
				j = x.ArgMap[i]
			}
			if j >= 0 && j < len(passed) {
				passed[j] = true
			}
		}
		for j, q := range params {
			if !passed[j] && q.Default && !q.Vararg && !completeDefault(cls.AnnotationDefaults[q.Name], q.Type) {
				c.unsupported(p, "library annotation default", "omitting arguments of a library's annotation class is not supported")
				break
			}
		}
	}
	return t
}

// completeDefault reports whether v, the AnnotationDefault of a class-path
// annotation's parameter of type t, gives every value codegen creates: the
// annotations in it have values, or default values, for all their
// parameters.
func completeDefault(v *classfile.ElementValue, t types.Type) bool {
	if v == nil {
		return false
	}
	switch v.Tag {
	case '@':
		if t.K != types.Class || v.Ann == nil || !t.C.KotlinAnnotation() {
			return false
		}
		for _, p := range t.C.AnnotationParams {
			pv := t.C.AnnotationDefaults[p.Name]
			for i := range v.Ann.Values {
				if v.Ann.Values[i].Name == p.Name {
					pv = &v.Ann.Values[i].Value
				}
			}
			if !completeDefault(pv, p.Type) {
				return false
			}
		}
	case '[':
		for i := range v.Elems {
			if !completeDefault(&v.Elems[i], types.VarargElem(t)) {
				return false
			}
		}
	}
	return true
}
