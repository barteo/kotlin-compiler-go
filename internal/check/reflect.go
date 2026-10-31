package check

import (
	"slices"
	"strconv"
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// Class literals.

// builtinTypeNames are the Kotlin types resolveType knows by name.
var builtinTypeNames = map[string]bool{
	"Int": true, "Long": true, "Double": true, "Boolean": true, "Char": true, "Float": true,
	"Byte": true, "Short": true, "String": true, "Unit": true, "Nothing": true, "Any": true,
}

// classLit checks X::class or x::class, and with java, X::class.java,
// which is the java.lang.Class itself and needs no standard library.
func (c *Checker) classLit(x *syntax.ClassLit, java bool) types.Type {
	x.Java = java
	var t types.Type
	if x.Ref != nil {
		t = c.classLitType(x.Ref, false)
	} else if name, ok := c.typeName(x.X); ok {
		t = c.classLitType(&syntax.TypeRef{P: x.X.Pos(), Name: name}, false)
	} else {
		vt := c.value(x.X, noWant)
		if vt.K == types.Invalid {
			return vt
		}
		if types.MayBeNull(vt) && !vt.P {
			c.errorf(x.P, "expression in a class literal has a nullable type '%s', use !! to make the type non-nullable", vt)
			return types.InvalidType
		}
		t, x.Bound = vt.NonNull(), true
	}
	if t.K == types.Invalid {
		return t
	}
	x.Class = t
	var cls *types.ClassSym
	if java {
		cls = c.u.ext.class("java/lang/Class")
	} else if c.u.ext.kotlin {
		cls = c.u.ext.class("kotlin/reflect/KClass")
	} else {
		c.errorf(x.P, "class literals need the Kotlin standard library; use ::class.java for the Java class")
		return types.InvalidType
	}
	if cls == nil {
		c.errorf(x.P, "class literals need java.lang.Class and kotlin.reflect.KClass on the class path")
		return types.InvalidType
	}
	arg := types.Arg{T: t}
	if x.Bound {
		arg.V = types.Out // the value's class may be a subclass
	}
	return types.GenericType(cls, []types.Arg{arg})
}

// typeName returns the name x spells if it names a type rather than a
// value: String, a.b.C, Outer.Nested.
func (c *Checker) typeName(x syntax.Expr) (string, bool) {
	var parts []string
	for {
		switch e := x.(type) {
		case *syntax.Member:
			if e.Safe {
				return "", false
			}
			parts = append(parts, e.Name)
			x = e.X
			continue
		case *syntax.Ident:
			if c.isVar(e.Name) {
				return "", false
			}
			parts = append(parts, e.Name)
		default:
			return "", false
		}
		break
	}
	for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
		parts[i], parts[j] = parts[j], parts[i]
	}
	name := strings.Join(parts, ".")
	if len(parts) == 1 && (builtinTypeNames[name] || c.lookupTypeParam(name) != nil) {
		return name, true
	}
	return name, c.lookupClass(name) != nil
}

// classLitType resolves the class on the left of :: in a class literal or,
// if ref, a callable reference. A generic class without type arguments is
// star-projected: List::class is a KClass<List<*>>. In class literals,
// only arrays take other type arguments.
func (c *Checker) classLitType(ref *syntax.TypeRef, callable bool) types.Type {
	if tp := c.lookupTypeParam(ref.Name); tp != nil {
		if tp.Reified && !callable {
			return types.ParamType(tp) // known in each copy of the function
		}
		c.errorf(ref.P, "cannot use '%s' as reified type parameter", ref.Name)
		return types.InvalidType
	}
	if ref.Args == nil && !builtinTypeNames[ref.Name] {
		if cls := c.lookupClass(ref.Name); cls != nil && ensured(cls).TypeParams != nil {
			stars := make([]*syntax.TypeArg, len(cls.TypeParams))
			for i := range stars {
				stars[i] = &syntax.TypeArg{P: ref.P, Variance: types.Star}
			}
			ref = &syntax.TypeRef{P: ref.P, Name: ref.Name, Args: stars}
		}
	}
	t := c.resolveType(ref)
	if t.K != types.Invalid && !t.IsArray() && !callable {
		for _, a := range ref.Args {
			if a.Variance != types.Star {
				c.errorf(a.P, "only classes are allowed on the left hand side of a class literal")
				return types.InvalidType
			}
		}
	}
	return t
}

// ensured loads a class's declarations if they aren't yet, and returns it.
func ensured(cls *types.ClassSym) *types.ClassSym {
	cls.Ensure()
	return cls
}

// Callable references.
//
// A callable reference is parsed as a lambda whose parameters and body the
// checker supplies, once it knows the parameter types: ::f becomes
// { p0, p1 -> f(p0, p1) }, C::m { p0, p1 -> p0.m(p1) }, x::m, with x
// evaluated once, { p0 -> x.m(p0) }, and C::p { p0 -> p0.p }. Overload
// resolution, generic inference, default and vararg parameters, Java and
// standard library members, and SAM conversion are then those of calls
// and lambdas, and so are code generation and inlining. The parameter
// types are the expected function type's, or without one, those of the
// only function the reference can name.

// refForm is a way to read a reference: its lambda's parameter types and
// a builder of its body.
type refForm struct {
	params  []types.Type
	body    func() syntax.Expr
	suspend bool // a reference to a suspend function, without an expected type
}

// checkRef checks a callable reference, as checkLambda does a lambda.
func (c *Checker) checkRef(x *syntax.Lambda, w lambdaWant) ([]types.Type, types.Type, bool) {
	r := x.Ref
	x.KProp, x.KFun = nil, nil
	if w.recover {
		return nil, types.InvalidType, false
	}
	p := x.P
	ident := func(name string) syntax.Expr {
		id := &syntax.Ident{Name: name}
		id.P = p
		return id
	}
	member := func(recv syntax.Expr, name string) syntax.Expr {
		m := &syntax.Member{X: recv, Name: name}
		m.P = p
		return m
	}
	spread := -1 // pass this parameter, an array, to a vararg
	// call calls fn with the parameters ps[from:].
	call := func(fn syntax.Expr, ps []types.Type, from int) syntax.Expr {
		cl := &syntax.Call{Fun: fn, RefArgs: ps[from:]}
		cl.P = p
		for i := from; i < len(ps); i++ {
			a := ident(refParam(i))
			if i == spread {
				sp := &syntax.Spread{X: a}
				sp.P = p
				a = sp
			}
			cl.Args = append(cl.Args, a)
		}
		return cl
	}

	// The left side: none, a class (an unbound reference, whose receiver
	// is the first parameter), or a value (bound).
	var recvType types.Type // of an unbound reference
	var recv func() syntax.Expr
	var static *types.ClassSym // a Java class whose static methods can be named
	var nested *types.ClassSym // Outer::Nested, a nested class's constructor
	if r.Bound != nil {
		if v, _ := c.scope.find(r.Bound.Name); v != r.Bound {
			// Bound in a check of the code before, in another scope: see
			// stubInference.
			r.Bound, r.BoundExpr = nil, nil
		}
	}
	switch {
	case r.Type != nil:
		recvType = c.classLitType(r.Type, true)
	case r.X == nil:
	case r.Bound != nil:
		recv = func() syntax.Expr { return ident(r.Bound.Name) }
	default:
		if name, ok := c.typeName(r.X); ok {
			if cls := c.lookupClass(name); cls != nil && ensured(cls).Object {
				// An object's members are bound to it: Registry::find.
				c.bindRef(r, c.value(r.X, noWant))
				recv = func() syntax.Expr { return ident(r.Bound.Name) }
				break
			}
			if cls := c.lookupClass(name + "." + r.Name); cls != nil && !ensured(cls).Object && !cls.Inner {
				nested = cls
				break
			}
			if cls := c.lookupClass(name); cls != nil && cls.Companion != nil && !c.hasCallable(cls, r.Name) && c.hasCallable(cls.Companion, r.Name) {
				// A member of the companion, which the class doesn't
				// have, is bound to the companion: Level::parse.
				c.bindRef(r, c.value(r.X, noWant))
				recv = func() syntax.Expr { return ident(r.Bound.Name) }
				break
			}
			recvType = c.classLitType(&syntax.TypeRef{P: r.X.Pos(), Name: name}, true)
			if cls := c.lookupClass(name); cls != nil && (cls.External || cls.Entries != nil) {
				static = cls // and an enum's valueOf and values: E::valueOf
			}
			break
		}
		t := c.value(r.X, noWant)
		if t.K == types.Invalid {
			return nil, types.InvalidType, false
		}
		c.bindRef(r, t)
		recv = func() syntax.Expr { return ident(r.Bound.Name) }
	}
	if recvType.K == types.Invalid && (r.Type != nil || (r.X != nil && recv == nil && nested == nil)) {
		return nil, types.InvalidType, false
	}
	if r.X == nil && r.Type == nil {
		// Only a Kotlin fun interface has a constructor to refer to.
		if cls := c.lookupClass(r.Name); cls != nil && !cls.Fun && cls.KotlinName == "" && c.sam(cls) != nil {
			c.errorf(p, "Java SAM interface constructor references are prohibited: %s", refString(r))
			return nil, types.InvalidType, false
		}
	}
	unbound := recvType.K != types.Invalid

	// The forms the reference can take, tried in order: a property, then
	// a function (a Java class's static method last).
	var forms []refForm
	propForm := func(ps []types.Type) refForm {
		return refForm{params: ps, body: func() syntax.Expr {
			switch {
			case unbound:
				return member(ident(refParam(0)), r.Name)
			case recv != nil:
				return member(recv(), r.Name)
			}
			return ident(r.Name)
		}}
	}
	funForm := func(ps []types.Type) refForm {
		return refForm{params: ps, body: func() syntax.Expr {
			switch {
			case unbound:
				return call(member(ident(refParam(0)), r.Name), ps, 1)
			case recv != nil:
				return call(member(recv(), r.Name), ps, 0)
			}
			return call(ident(r.Name), ps, 0)
		}}
	}
	staticForm := func(ps []types.Type) refForm {
		return refForm{params: ps, body: func() syntax.Expr { return call(member(r.X, r.Name), ps, 0) }}
	}
	switch {
	case nested != nil:
		// Outer.Nested(...), with the expected parameters or else those of
		// its only constructor.
		ps := slices.Clone(w.params)
		if !w.known {
			ctors := nested.Ctors
			if nested.Ctor != nil {
				ctors = append([]*types.FunSym{nested.Ctor}, ctors...)
			}
			if len(ctors) != 1 || len(nested.TypeParams) > 0 {
				c.errorf(p, "not enough information to infer the constructor of %s: specify the expected function type", refString(r))
				return nil, types.InvalidType, false
			}
			ps = nil
			for _, q := range ctors[0].Params {
				ps = append(ps, q.Type)
			}
		}
		forms = append(forms, staticForm(ps))
	case w.prop:
		var ps []types.Type
		if unbound {
			ps = []types.Type{recvType}
		}
		return c.kpropLambda(x, propForm(ps), recv != nil)
	case w.known:
		ps := slices.Clone(w.params)
		switch {
		case !unbound:
			if len(ps) == 0 && !c.namesClass(r, recv != nil) {
				forms = append(forms, propForm(ps))
			}
			forms = append(forms, funForm(ps))
		case len(ps) > 0 && c.subtype(ps[0], recvType):
			if len(ps) == 1 {
				forms = append(forms, propForm(ps))
			}
			forms = append(forms, funForm(ps))
		}
		if static != nil {
			forms = append(forms, staticForm(ps))
		}
		if len(forms) == 0 {
			c.errorf(p, "type mismatch: %s needs a receiver of type %s, but (%s) -> %s was expected",
				refString(r), recvType, typesString(ps), w.ret)
			return nil, types.InvalidType, false
		}
	default:
		var ps []types.Type
		if unbound {
			ps = []types.Type{recvType}
		}
		if !c.namesClass(r, recv != nil) {
			if w.kprop {
				// A property's reference is a KProperty object, as in
				// Kotlin, if kc can make one, and otherwise a function.
				n := len(c.errs)
				if ps, rt, ok := c.kpropLambda(x, propForm(ps), recv != nil); ok && len(c.errs) == n {
					return ps, rt, ok
				}
				c.errs, x.KProp = c.errs[:n], nil
			}
			forms = append(forms, propForm(ps))
		}
		f, ok := c.refTarget(p, r, recvType, static, recv != nil)
		if !ok {
			return nil, types.InvalidType, false
		}
		if f != nil {
			fps := ps
			sub := c.memberSubst(recvType, f.Owner)
			switch {
			case recv != nil:
				sub = c.memberSubst(r.Bound.Type, f.Owner)
			case !unbound && r.X == nil && r.Type == nil:
				sub = c.implicitSubst(f.Owner)
			}
			for _, q := range f.Params {
				if q.Vararg {
					c.unsupported(p, "callable reference", "a reference to a function with a vararg parameter needs an expected function type: %s", refString(r))
					return nil, types.InvalidType, false
				}
				fps = append(fps, sub.ApplyIn(q.Type))
			}
			form := funForm(fps)
			if static != nil && f.Static && !objectStatic(f) {
				form = staticForm(fps[1:])
			}
			form.suspend = f.Suspend
			forms = append(forms, form)
		} else if rt := recvType; unbound || recv != nil {
			if recv != nil {
				rt = r.Bound.Type
			}
			if bps, ok := builtinRefParams(rt, r.Name); ok {
				// A builtin type's member: Boolean::not, String::plus.
				forms = append(forms, funForm(append(ps, bps...)))
			}
		} else if r.X == nil && r.Type == nil {
			// An implicit receiver's builtin member, the innermost first:
			// ::toString in with(1) { }.
			for i := len(c.recvs) - 1; i >= 0; i-- {
				if bps, ok := builtinRefParams(c.scope.narrowed(c.recvs[i].sym), r.Name); ok {
					forms = append(forms, funForm(bps))
					break
				}
			}
		}
	}

	if n := len(forms); w.known && n > 0 {
		// An array parameter may be a vararg's, the last one's first:
		// f(*a), f(*a, b = c).
		last := forms[n-1]
		for i := len(w.params) - 1; i >= 0; i-- {
			if w.params[i].NonNull().IsArray() {
				forms = append(forms, refForm{params: last.params, suspend: last.suspend, body: func() syntax.Expr {
					spread = i
					defer func() { spread = -1 }()
					return last.body()
				}})
			}
		}
	}

	// Try each form; report the errors of the last one if none fits.
	w.recv, w.known = false, true
	suspend := w.suspend
	for i, f := range forms {
		w.suspend = suspend || f.suspend
		n := len(c.errs)
		x.Params = make([]*syntax.LambdaParam, len(f.params))
		for k := range x.Params {
			x.Params[k] = &syntax.LambdaParam{P: p, Name: refParam(k)}
		}
		x.Body = &syntax.Block{P: p, Stmts: []syntax.Stmt{&syntax.ExprStmt{X: f.body()}}}
		x.Captures, x.This = nil, false
		w.params = f.params
		ps, rt, ok := c.checkLambda1(x, w)
		if len(c.errs) == n || i == len(forms)-1 {
			return ps, rt, ok
		}
		c.errs = c.errs[:n]
	}
	return nil, types.InvalidType, false
}

// typesString lists types for messages.
func typesString(ts []types.Type) string {
	names := make([]string, len(ts))
	for i, t := range ts {
		names[i] = t.String()
	}
	return strings.Join(names, ", ")
}

// namesClass reports whether a reference without a receiver names a
// class, ::C, which is its constructor even if the class has a companion
// object that the name could also denote.
func (c *Checker) namesClass(r *syntax.CallableRef, bound bool) bool {
	return r.X == nil && r.Type == nil && !bound && c.lookupClass(r.Name) != nil
}

// refParam names the i'th parameter of a reference's lambda.
func refParam(i int) string { return "$p" + strconv.Itoa(i) }

// refString returns a reference as written, for messages.
func refString(r *syntax.CallableRef) string {
	switch {
	case r.Type != nil:
		return r.Type.Name + "::" + r.Name
	case r.X != nil:
		if name, ok := dotted(r.X); ok {
			return name + "::" + r.Name
		}
	}
	return "::" + r.Name
}

// dotted returns the name a.b.c that x spells, if it does.
func dotted(x syntax.Expr) (string, bool) {
	switch x := x.(type) {
	case *syntax.Ident:
		return x.Name, true
	case *syntax.Member:
		s, ok := dotted(x.X)
		return s + "." + x.Name, ok
	}
	return "", false
}

// bindRef declares the local that holds a bound reference's receiver, of
// type t, in the enclosing scope, where it is evaluated once.
func (c *Checker) bindRef(r *syntax.CallableRef, t types.Type) {
	if t.K == types.Invalid {
		t = types.AnyType.Nullable()
	}
	r.Bound = &types.VarSym{Name: "$receiver" + strconv.Itoa(c.refCount), Type: t, Kind: types.Local}
	c.refCount++
	r.BoundExpr = r.X
	c.scope.declare(r.Bound)
}

// refTarget finds the only function a reference without an expected type
// can name, nil if it names none (it may name a property), and reports
// an error and false if it names several.
func (c *Checker) refTarget(p syntax.Pos, r *syntax.CallableRef, recvType types.Type, static *types.ClassSym, bound bool) (*types.FunSym, bool) {
	var fs []*types.FunSym
	switch {
	case recvType.K != types.Invalid || bound:
		rt := recvType
		if bound {
			rt = r.Bound.Type
		}
		if cls := c.classOf(rt.NonNull()); cls != nil {
			for _, f := range c.accessible(c.javaMethods(cls, r.Name)) {
				if !f.Static || objectStatic(f) {
					fs = append(fs, f)
				}
			}
			if n, _ := c.innerNamed(cls, r.Name); n != nil {
				// An inner class's constructor, which takes the outer
				// instance: Outer::Inner, outer::Inner.
				n.Ensure()
				fs = append(fs, userCtors(n)...)
			}
		}
		if static != nil && !bound && !static.External {
			fs = append(fs, static.Statics[r.Name]...) // not Enum.valueOf(Class, String)
		} else if static != nil && !bound {
			fs = append(fs, javaStatics(static, r.Name)...)
		}
		for _, cl := range c.extCallees(r.Name, rt) {
			fs = append(fs, cl.f)
		}
	default:
		fs, _ = c.scope.findFuns(r.Name) // local functions first
		if len(fs) == 0 {
			fs = append(fs, c.topFuns(r.Name)...)
		}
		if c.u.ext.kotlin && len(fs) == 0 {
			fs = append(fs, c.kotlinFuns(r.Name, false)...)
		}
		if cls := c.lookupClass(r.Name); cls != nil {
			cls.Ensure()
			if f := c.samCtorRef(cls); f != nil {
				fs = append(fs, f) // ::KRunnable
			} else {
				if cls.Ctor != nil {
					fs = append(fs, cls.Ctor)
				}
				fs = append(fs, cls.Ctors...)
			}
		}
		// A member of an implicit receiver, the innermost first, or of
		// this.
		for i := len(c.recvs) - 1; i >= 0 && len(fs) == 0; i-- {
			if cls := c.classOf(c.scope.narrowed(c.recvs[i].sym).NonNull()); cls != nil {
				fs = c.accessible(c.javaMethods(cls, r.Name))
			}
		}
		if len(fs) == 0 && c.cls != nil {
			fs = c.accessible(c.javaMethods(c.cls, r.Name))
		}
		// An extension of an implicit receiver, the innermost first, or of
		// this or an inner class's outer instances: ::f for fun A.f() in
		// A's code. A builtin member wins: ::toString in with(1) { }.
		for i := len(c.recvs) - 1; i >= 0 && len(fs) == 0; i-- {
			rt := c.scope.narrowed(c.recvs[i].sym)
			if _, ok := builtinRefParams(rt, r.Name); ok {
				break
			}
			fs = calleeFuns(c.extCallees(r.Name, rt))
		}
		for k := c.cls; k != nil && len(fs) == 0; k = k.Outer {
			fs = calleeFuns(c.extCallees(r.Name, k.This()))
			if !k.Inner {
				break
			}
		}
	}
	switch {
	case len(fs) == 0:
		return nil, true
	case len(fs) > 1:
		c.errorf(p, "overload resolution ambiguity for %s: specify the expected function type", refString(r))
		return nil, false
	case len(fs[0].TypeParams) > 0 || fs[0].Owner != nil && len(fs[0].Owner.TypeParams) > 0 && fs[0].Name == "<init>":
		c.errorf(p, "not enough information to infer type arguments of %s: specify the expected function type", refString(r))
		return nil, false
	}
	return fs[0], true
}

// implicitSubst gives the type arguments of owner, the class of a member
// that a reference without a receiver names, from the innermost implicit
// receiver of that class: ::yield in sequence<String> { } takes a String.
func (c *Checker) implicitSubst(owner *types.ClassSym) types.Subst {
	if owner == nil || len(owner.TypeParams) == 0 {
		return nil
	}
	for i := len(c.recvs) - 1; i >= 0; i-- {
		if sup, ok := c.asSuper(c.scope.narrowed(c.recvs[i].sym).NonNull(), owner); ok {
			return classSubst(sup)
		}
	}
	return nil
}

// objectStatic reports whether f is an object's @JvmStatic function: a
// static method, but a member of the object to Kotlin.
func objectStatic(f *types.FunSym) bool {
	return f.Static && f.Owner != nil && f.Owner.Object
}

// Property references as kotlin.reflect objects.
//
// A reference to a property whose expected type is KProperty1<T, V>, or
// another of kotlin.reflect's property types, is a KProperty object, as
// property(Money::currencyCode) needs, rather than a function, and so is
// one without an expected function type, val p = ::x, where kc can make
// one (see checkRef). It is still checked as a lambda, { p0 -> p0.p } for
// C::p, { x.p } for x::p and { p } for ::p, which is its object's get,
// and for a var, with a second one, which is its set. Its type is KProperty1<C, V> or
// KMutableProperty1<C, V> for C::p, KProperty0<V> or KMutableProperty0<V>
// for the others.

// kpropTypes are the expected types that make a reference a KProperty.
var kpropTypes = map[string]bool{
	"kotlin/reflect/KProperty0": true, "kotlin/reflect/KProperty1": true,
	"kotlin/reflect/KMutableProperty0": true, "kotlin/reflect/KMutableProperty1": true,
	"kotlin/reflect/KProperty": true, "kotlin/reflect/KMutableProperty": true,
	"kotlin/reflect/KCallable": true,
}

// wantsKProp reports whether a callable reference with expected type t
// is a property reference object.
func wantsKProp(x *syntax.Lambda, t types.Type) bool {
	t = t.Flex().NonNull()
	return x.Ref != nil && t.K == types.Class && kpropTypes[t.C.Name]
}

// kpropRef checks a property reference whose expected type is one of
// kotlin.reflect's, and returns its type.
func (c *Checker) kpropRef(x *syntax.Lambda) types.Type {
	ps, rt, ok := c.checkRef(x, lambdaWant{ret: noWant, prop: true})
	if !ok || rt.K == types.Invalid {
		return types.InvalidType
	}
	return c.kpropType(x, ps, rt)
}

// kpropType returns the type of property reference x, whose lambda takes
// ps and returns rt.
func (c *Checker) kpropType(x *syntax.Lambda, ps []types.Type, rt types.Type) types.Type {
	name := "kotlin/reflect/KProperty"
	if x.KProp.Set != nil {
		name = "kotlin/reflect/KMutableProperty"
	}
	args := []types.Arg{{T: rt}}
	if len(ps) == 1 {
		name += "1"
		args = append([]types.Arg{{T: ps[0]}}, args...)
	} else {
		name += "0"
	}
	cls := c.u.ext.class(name)
	if cls == nil {
		c.errorf(x.P, "property references need kotlin.reflect: compile with the standard library")
		return types.InvalidType
	}
	x.Ret = rt
	return types.GenericType(cls, args)
}

// kpropLambda checks a property reference's lambda, f, and its setter's,
// and sets x.KProp. bound is set on x::p.
func (c *Checker) kpropLambda(x *syntax.Lambda, f refForm, bound bool) ([]types.Type, types.Type, bool) {
	p := x.P
	lambdaOf := func(params []types.Type, body syntax.Stmt) *syntax.Lambda {
		l := &syntax.Lambda{Arrow: true, Body: &syntax.Block{P: p, Stmts: []syntax.Stmt{body}}}
		l.P = p
		for k := range params {
			l.Params = append(l.Params, &syntax.LambdaParam{P: p, Name: refParam(k)})
		}
		return l
	}
	get := lambdaOf(f.params, &syntax.ExprStmt{X: f.body()})
	x.Arrow, x.Params, x.Body, x.Captures, x.This = true, get.Params, get.Body, nil, false
	ps, rt, ok := c.checkLambda1(x, lambdaWant{params: f.params, known: true, ret: noWant})
	if !ok || rt.K == types.Invalid {
		return ps, rt, false
	}
	k := &syntax.KProp{Bound: bound}
	mutable := false
	switch e := x.Body.Stmts[0].(*syntax.ExprStmt).X.(type) {
	case *syntax.Member:
		switch {
		case e.Dispatch != nil:
		case e.Prop != nil && e.Prop.Kind == types.Prop:
			k.Prop, mutable = e.Prop, e.Prop.Mutable && (!e.Prop.SetterPrivate || c.inside(e.Prop.Owner))
		case e.Getter != nil && e.Getter.Ext && e.Getter.Owner == nil:
			k.Getter, mutable = e.Getter, e.Getter.ExtSetter != nil
		case e.Getter != nil && !e.Getter.Ext && e.Getter.Owner != nil:
			// A property of a Kotlin class of the class path, or a Java
			// getter seen as one, as Enum's name and ordinal.
			if kp := e.Getter.Owner.KProps[x.Ref.Name]; kp != nil && kp.Getter == e.Getter {
				k.Getter, mutable = e.Getter, kp.Setter != nil
			} else if e.Getter.Owner.External {
				k.Getter, mutable = e.Getter, e.Setter != nil
			}
		case e.Builtin == types.StrLength:
			if s := c.u.ext.class("java/lang/String"); s != nil {
				k.Getter = &types.FunSym{Name: "length", Owner: s, Desc: "()I", Ret: types.IntType, Resolved: true}
			}
		}
	case *syntax.Ident:
		switch {
		case e.Sym != nil && e.Sym.Kind == types.Global && e.Getter == nil && e.Member == nil:
			k.Prop, mutable = e.Sym, e.Sym.Mutable
		case !bound && e.Member == nil && c.cls != nil &&
			(e.Sym != nil && e.Sym.Kind == types.Prop && c.cls.IsSubclassOf(e.Sym.Owner) || e.Getter != nil && e.Getter.Owner != nil && c.cls.IsSubclassOf(e.Getter.Owner) ||
				e.Sym != nil && e.Sym.Kind == types.Param && c.userProp(c.cls, e.Name) != nil && c.userProp(c.cls, e.Name).Kind == types.Prop):
			// ::p for a property of this class is this::p, also where p
			// is a constructor's parameter, which a reference can't name.
			this := &syntax.This{Class: true}
			this.P = p
			x.Ref.X = this
			return c.checkRef(x, lambdaWant{ret: noWant, prop: true})
		}
	}
	if k.Prop == nil && k.Getter == nil {
		c.unsupported(p, "property reference", "%s as a kotlin.reflect object: only references to properties of classes, top-level properties and top-level extension properties are supported", refString(x.Ref))
		return ps, rt, false
	}
	if mutable {
		// Its set: the value is the last parameter.
		value := &syntax.Ident{Name: refParam(len(f.params))}
		value.P = p
		set := lambdaOf(append(slices.Clone(f.params), rt), &syntax.Assign{P: p, Op: syntax.ASSIGN, Target: f.body(), Value: value})
		n := len(c.errs)
		_, _, ok := c.checkLambda1(set, lambdaWant{params: append(slices.Clone(f.params), rt), known: true, ret: types.UnitType, check: true})
		if ok && len(c.errs) == n {
			set.Ret, k.Set = types.UnitType, set
		} else {
			c.errs = c.errs[:n] // a setter it can't call: a KProperty
		}
	}
	x.KProp = k
	return ps, rt, true
}

// Function references as kotlin.reflect objects.
//
// A reference to a function whose expected type is KFunctionN<P1, ...,
// Pn, R> is checked as one whose expected type is the function type
// (P1, ..., Pn) -> R, which KFunctionN extends, and its object is a
// FunctionReference with the function's name, as kotlinc's
// createSegmentFunction(): KFunction2<Long, S, S> = ::createSegment.

// kfunWant returns the function type that a callable reference with
// expected type t is checked against, if t is a KFunctionN or a
// KSuspendFunctionN.
func (c *Checker) kfunWant(t types.Type) (types.Type, bool) {
	t = t.Flex().NonNull()
	if t.K != types.Class || t.A == nil {
		return types.Type{}, false
	}
	n, suspend, ok := kfunctionArity(t.C)
	if !ok {
		return types.Type{}, false
	}
	if suspend {
		return c.asSuper(t, c.u.ext.funcs.SuspendClass(n))
	}
	return c.asSuper(t, c.u.ext.funcs.Class(n))
}

// kfunRef checks a reference x to a function whose expected type is
// KFunctionN or KSuspendFunctionN want, and returns its type.
func (c *Checker) kfunRef(x *syntax.Lambda, want, fn types.Type) types.Type {
	n := len(c.errs)
	t := c.lambda(x, fn)
	if t.K == types.Invalid || len(c.errs) > n {
		return types.InvalidType // already reported
	}
	call := kfunCall(x)
	if call == nil {
		c.unsupported(x.P, "callable reference", "%s as a kotlin.reflect object: only references to functions and constructors of classes are supported", refString(x.Ref))
		return types.InvalidType
	}
	c.setKFun(x, call)
	return types.GenericType(want.Flex().NonNull().C, t.A.List)
}

// setKFun makes reference x, to the function that call calls, a
// kotlin.reflect object, of a class of its own.
func (c *Checker) setKFun(x *syntax.Lambda, call *syntax.Call) {
	x.KFun = call
	if x.KFunClass == "" && x.Class != "" {
		// A suspend conversion's, named as a suspend lambda's: its object
		// is a reference whose invoke calls the function instead.
		cls := x.Class
		x.KFunClass, x.Class = cls, ""
		c.onUndo(func() { x.KFunClass, x.Class = "", cls })
	}
	if x.KFunClass == "" {
		x.KFunClass = c.anonName()
		c.onUndo(func() { x.KFunClass = "" })
	}
}

// kfunCall returns the call of the function that reference x, checked,
// is to, if it can be a kotlin.reflect object: a function's or a class's
// constructor's, but for one of a suspend function type whose invoke
// needs a state machine (see refStateMachine).
func kfunCall(x *syntax.Lambda) *syntax.Call {
	if len(x.Body.Stmts) == 0 {
		return nil // not resolved
	}
	call, ok := x.Body.Stmts[0].(*syntax.ExprStmt).X.(*syntax.Call)
	if !ok || call.Sym == nil || call.Invoke || x.KProp != nil || call.Ctor != nil && (call.Ctor.Value || call.Ctor.Annotation || call.Ctor.Local) {
		return nil
	}
	if x.Suspend && x.SAM == nil && call.Sym.Suspend && refStateMachine(x, call.Sym) {
		return nil
	}
	return call
}

// refStateMachine reports whether the invoke of reference x, of a suspend
// function type, to suspend function f is a state machine of its own with
// kotlinc, rather than a call whose result it returns: if x ignores f's
// result, if f is inline, whose body it copies, or if f returns a value
// class's underlying reference, which it boxes.
func refStateMachine(x *syntax.Lambda, f *types.FunSym) bool {
	r := f.Ret
	return x.Ret.K == types.Unit && r.K != types.Unit || syntax.SourceInline(f) != nil || f.Inline != nil ||
		types.Unboxed(r) && !r.N && !types.Repr(r).IsPrimitive()
}

// inside reports whether the code being checked is in class cls, or in a
// class nested in it, where its private members are visible.
func (c *Checker) inside(cls *types.ClassSym) bool {
	for k := c.cls; k != nil; k = k.Outer {
		if k == cls {
			return true
		}
	}
	return false
}

// hasCallable reports whether cls has a member function or a property
// named name.
func (c *Checker) hasCallable(cls *types.ClassSym, name string) bool {
	return len(c.javaMethods(cls, name)) > 0 || c.hasProp(cls, name)
}
