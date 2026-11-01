package check

import (
	"fmt"
	"github.com/barteo/kotlin-compiler-go/internal/classfile"
	"maps"
	"slices"
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// This file checks the class model: supertypes, overrides, abstract
// members, and member lookup through the class hierarchy.

// resolveSupertypes resolves the supertypes of the file's classes. It runs
// for all files before anything checks subtyping: asSuper caches its
// results, and a class's supertypes must be known before the first check.
// For the same reason, the bounds of the supertypes' type arguments are
// checked later, by checkSuperBounds.
func (c *Checker) resolveSupertypes() {
	c.noBoundChecks = true
	defer func() {
		c.noBoundChecks = false
		c.tparams = nil
	}()
	object := c.u.ext.class("java/lang/Object")
	for _, cd := range c.classes {
		cls := cd.Sym
		// The supertypes don't see the class's own nested classes: a
		// data class Reply(val page: Page) : Page, with a nested Page,
		// implements the outer one. An object expression's or
		// a local class's, which has none, see the enclosing function's
		// type parameters through it.
		c.tparams, c.lookupCls = cls.TypeParams, cls.Outer
		if cls.Anonymous || cls.Local {
			c.lookupCls = cls
		}
		cls.Supers = []types.Type{}
		seen := map[*types.ClassSym]bool{}
		for _, st := range cd.Supers {
			if st.Delegate != nil && cd.Interface {
				c.errorf(st.Type.P, "delegation cannot be used in interfaces")
				st.Delegate = nil
			}
			t := c.supertype(cd, st, seen)
			if t.E || t.X > 0 {
				c.errorf(st.Type.P, "extension function type or function type with context parameters is not allowed as supertype")
			}
			if t.IsComposableFunc() {
				c.unsupported(st.Type.P, "compose", "classes that implement composable function types are not supported")
				t = types.InvalidType
			}
			st.T = t
			if t.K != types.Class {
				continue
			}
			seen[t.C] = true
			cls.Supers = append(cls.Supers, t)
			if t.C.Interface {
				cls.Interfaces = append(cls.Interfaces, t.C)
			} else {
				cls.Super = t.C
				cd.SuperCall = st.Call
			}
		}
		if cd.Annotation {
			// An annotation class implements java.lang.annotation.Annotation.
			if ann := c.u.ext.class("java/lang/annotation/Annotation"); ann != nil {
				ann.Ensure()
				cls.Interfaces = append(cls.Interfaces, ann)
				cls.Supers = append(cls.Supers, ann.This())
			}
		}
		if cd.Enum {
			// An enum class E extends java.lang.Enum<E>.
			if enum := c.u.ext.class("java/lang/Enum"); enum != nil {
				enum.Ensure()
				cls.Super = enum
				if len(enum.TypeParams) == 1 {
					cls.Supers = append([]types.Type{types.GenericType(enum, []types.Arg{{T: cls.This()}})}, cls.Supers...)
				}
			}
		}
		if cls.Super == nil {
			cls.Super = object
		}
	}
	c.lookupCls = nil
}

// supertype resolves and checks one supertype of a class.
func (c *Checker) supertype(cd *syntax.ClassDecl, st *syntax.SuperType, seen map[*types.ClassSym]bool) types.Type {
	cls := cd.Sym
	t := c.resolveType(st.Type)
	switch {
	case t.K == types.Invalid:
		return t
	case t.K == types.Any:
		if cd.Interface && st.Call != nil {
			c.errorf(st.Type.P, "an interface cannot inherit from a class")
		}
		return types.InvalidType // the default superclass
	case t.N:
		c.errorf(st.Type.P, "a supertype cannot be nullable")
		return types.InvalidType
	case t.K != types.Class:
		c.errorf(st.Type.P, "this type is final, so it cannot be inherited from")
		return types.InvalidType
	}
	sc := t.C
	sc.Ensure()
	switch {
	case seen[sc]:
		c.errorf(st.Type.P, "a supertype appears twice")
	case cd.Enum && !sc.Interface:
		c.errorf(st.Type.P, "an enum class cannot inherit from a class")
	case sc.Interface:
		if st.Call != nil {
			c.errorf(st.Call.P, "this type has no constructor, and thus must not be called")
			return types.InvalidType
		}
		return t
	case st.Delegate != nil:
		c.errorf(st.Type.P, "delegation is supported only for interfaces")
	case cd.Interface:
		c.errorf(st.Type.P, "an interface cannot inherit from a class")
	case cls.Super != nil:
		c.errorf(st.Type.P, "only one class may appear in a supertype list")
	case sc.Final:
		c.errorf(st.Type.P, "this type is final, so it cannot be inherited from")
	case !cd.HasPrimary && st.Call != nil:
		c.errorf(st.Type.P, "supertype initialization is impossible without a primary constructor")
		st.Call = nil
		return t
	case st.Call == nil && cd.HasPrimary:
		c.errorf(st.Type.P, "this type has a constructor, and thus must be initialized here")
	default:
		return t
	}
	return types.InvalidType
}

// findCycles finds the file's classes that inherit from themselves. It
// runs for all files before checkSupers breaks the cycles, so each class
// in a cycle is reported.
func (c *Checker) findCycles() {
	for _, cd := range c.classes {
		if inherits(cd.Sym, cd.Sym, map[*types.ClassSym]bool{}) {
			c.cyclic = append(c.cyclic, cd)
		}
	}
}

// checkSupers runs once every class's supertypes are known: it breaks
// cycles in the hierarchy, and checks the bounds of supertypes' type
// arguments.
func (c *Checker) checkSupers() {
	for _, cd := range c.cyclic {
		c.errorf(cd.P, "there's a cycle in the inheritance hierarchy for this type")
		cls := cd.Sym
		cls.Super, cls.Interfaces, cls.Supers = c.u.ext.class("java/lang/Object"), nil, []types.Type{}
		cd.SuperCall = nil
	}
	for _, cd := range c.classes {
		c.tparams, c.lookupCls = cd.Sym.TypeParams, cd.Sym
		c.checkConsistentSupers(cd)
		for _, st := range cd.Supers {
			if st.T.K == types.Class && st.T.A != nil {
				c.checkBounds(st.T, st.Type)
			}
			// A sealed class's subclasses are in its package, and known.
			if s := st.T.C; st.T.K == types.Class && s.Sealed {
				if packageOf(s) != packageOf(cd.Sym) {
					c.errorf(st.Type.P, "inheritors of sealed classes must be in the same package")
				}
				s.Subclasses = append(s.Subclasses, cd.Sym)
			}
		}
	}
	c.tparams, c.lookupCls = nil, nil
}

// checkConsistentSupers reports a class whose supertypes reach a generic
// class with different type arguments, as kotlinc's
// INCONSISTENT_TYPE_PARAMETER_VALUES: interface C : A<Int>, B, where
// interface B : A<String>.
func (c *Checker) checkConsistentSupers(cd *syntax.ClassDecl) {
	if len(cd.Supers) < 2 {
		return // one supertype is consistent, or reported itself
	}
	seen := map[*types.ClassSym]types.Type{}
	var queue []types.Type
	for _, st := range cd.Supers {
		queue = append(queue, st.T)
	}
	for len(queue) > 0 {
		s := queue[0]
		queue = queue[1:]
		if s.K != types.Class {
			continue
		}
		prev, ok := seen[s.C]
		if !ok {
			seen[s.C] = s
			queue = append(queue, c.directSupers(s)...)
			continue
		}
		if s.A != nil && prev.A != nil && !c.sameArgs(prev, s) {
			c.errorf(cd.Supers[0].Type.P, "type parameters of '%s' have inconsistent values: %s, %s", s.C.DisplayName(), prev, s)
			return
		}
	}
}

// sameArgs reports whether class types t and u, of the same class, have
// the same type arguments, but where one is a star projection, as a Java
// raw type's are.
func (c *Checker) sameArgs(t, u types.Type) bool {
	for i, a := range t.A.List {
		if i >= u.A.Len() {
			break
		}
		b := u.A.Get(i)
		if a.V == types.Star || b.V == types.Star {
			continue
		}
		if a.V != b.V || !c.subtype(a.T, b.T) || !c.subtype(b.T, a.T) {
			return false
		}
	}
	return true
}

// packageOf returns the JVM package of a class: its internal name up to
// the last /.
func packageOf(cls *types.ClassSym) string {
	if i := strings.LastIndexByte(cls.Name, '/'); i >= 0 {
		return cls.Name[:i]
	}
	return ""
}

// inherits reports whether one of cls's supertypes, among the program's
// classes, is target.
func inherits(cls, target *types.ClassSym, seen map[*types.ClassSym]bool) bool {
	if seen[cls] || cls.External {
		return false
	}
	seen[cls] = true
	for _, s := range cls.Supers {
		if s.C == target || inherits(s.C, target, seen) {
			return true
		}
	}
	return false
}

// platformOnly reports whether the members a function overrides are all
// members of a Kotlin collection type that only its Java interface
// declares, as Map.remove(key, value): as with kotlinc, the function
// needn't be marked override, and is the JVM method that overrides them.
func platformOnly(over []*types.FunSym) bool {
	for _, g := range over {
		if !g.Platform {
			return false
		}
	}
	return true
}

// paramKey returns the erased parameter types of a method, which tell
// apart the methods that one with the same name overrides. A name that
// a value class parameter mangles is part of it: Dp's div(Float) and
// div(Dp) both take a float, as div-u2uoSUM and div-0680j_4.
func paramKey(f *types.FunSym) string {
	if f.Ext && types.Mangles(f.Recv) || slices.ContainsFunc(f.Params, func(p *types.VarSym) bool { return types.Mangles(p.Type) }) {
		return f.JVM() + paramKey1(f)
	}
	return paramKey1(f)
}

func paramKey1(f *types.FunSym) string {
	if len(f.Contexts) == 0 {
		return paramKey2(f)
	}
	// The context parameters, apart, which a function without them but
	// with parameters of the same types doesn't override.
	k := paramKey2(f)
	if f.Desc == "" {
		d, ok := types.ParamsDesc(f.Contexts)
		if !ok {
			return k
		}
		return "c" + d + k
	}
	i := 1 // a class-path function's descriptor takes them first
	for range f.Contexts {
		for k[i] == '[' {
			i++
		}
		if k[i] == 'L' {
			i = strings.IndexByte(k[i:], ';') + i
		}
		i++
	}
	return "c" + k[:i] + ")(" + k[i:]
}

func paramKey2(f *types.FunSym) string {
	if f.Impl {
		// A static method that takes the value first: its parameters.
		d := f.Desc[:strings.IndexByte(f.Desc, ')')+1]
		i := 1
		for d[i] == '[' {
			i++
		}
		if d[i] == 'L' {
			i = strings.IndexByte(d, ';')
		}
		return "(" + d[i+1:]
	}
	if f.Desc != "" && (f.Owner == nil || !f.Owner.Func) {
		// But a suspend function type's invoke, whose descriptor takes the
		// continuation too, by its parameters, as its overrides.
		d := f.Desc[:strings.IndexByte(f.Desc, ')')+1]
		if f.Suspend {
			// A suspend function's, without the continuation, as the
			// program's overrides.
			d = strings.TrimSuffix(d, "Lkotlin/coroutines/Continuation;)") + ")"
		}
		// A composable function's, without its Composer and ints.
		return types.KotlinParamsDesc(d, f)
	}
	if d, ok := types.ParamsDesc(f.Params); ok {
		return d
	}
	return fmt.Sprintf("%p", f) // an invalid signature overrides nothing
}

// userProp finds a property of one of the program's classes, declared in
// cls or inherited, or nil. The most derived declaration comes first.
// hasProp reports whether cls has a property name of any kind: the
// program's, a Kotlin one of the class path, a Java field or a Java
// getter.
func (c *Checker) hasProp(cls *types.ClassSym, name string) bool {
	if c.userProp(cls, name) != nil {
		return true
	}
	found := false
	cls.Supertypes(func(s *types.ClassSym) bool {
		found = s.KProps[name] != nil
		return !found
	})
	return found || len(c.javaMethods(cls, types.GetterName(name))) > 0 ||
		name == "length" && c.isCharSequence(cls) || name == "size" && c.hasSize(cls)
}

func (c *Checker) userProp(cls *types.ClassSym, name string) *types.VarSym {
	var p *types.VarSym
	cls.Supertypes(func(s *types.ClassSym) bool {
		if !s.External {
			p = s.Prop(name)
		} else if f := s.JavaFields[name]; f != nil {
			p = f // a Java class's field
		}
		return p == nil
	})
	return c.propAlt(p)
}

// accessible drops the protected members that code outside a subclass of
// their class can't use.
func (c *Checker) accessible(fs []*types.FunSym) []*types.FunSym {
	for i, f := range fs {
		if f.Protected && !c.inSubclass(f.Owner) {
			out := append([]*types.FunSym(nil), fs[:i]...)
			for _, g := range fs[i+1:] {
				if !g.Protected || c.inSubclass(g.Owner) {
					out = append(out, g)
				}
			}
			return out
		}
	}
	return fs
}

// inSubclass reports whether the code is in a subclass of cls, or in a
// class nested in one, an object expression's among them: the code that
// can use cls's protected members.
func (c *Checker) inSubclass(cls *types.ClassSym) bool {
	for k := c.cls; k != nil; k = k.Outer {
		if k.IsSubclassOf(cls) {
			return true
		}
	}
	return false
}

// memberCallees returns the candidates for a call of methods of a
// receiver of type rt, without those that others override.
func (c *Checker) memberCallees(ms []*types.FunSym, rt types.Type) []callee {
	return c.dedupe(methodCallees(ms, func(f *types.FunSym) types.Subst { return c.memberSubst(rt, f.Owner) }))
}

// sameParams reports whether method g, seen from a receiver of type rt,
// takes the same parameter types as f: whether one overrides the other.
func (c *Checker) sameParams(rt types.Type, f, g *types.FunSym) bool {
	if len(f.Params) != len(g.Params) || len(f.Contexts) != len(g.Contexts) || len(f.TypeParams) != len(g.TypeParams) || f.Static != g.Static || f.Ext != g.Ext {
		return false
	}
	fs, gs := c.memberSubst(rt, f.Owner), c.overrideSubst(rt, f, g)
	// The context parameters too, in order.
	for i, p := range f.Contexts {
		if !c.sameType(fs.Apply(p.Type), gs.Apply(g.Contexts[i].Type)) {
			return false
		}
	}
	for i, p := range f.Params {
		if !c.sameType(fs.Apply(p.Type), gs.Apply(g.Params[i].Type)) {
			return false
		}
	}
	// A member extension's receiver is a parameter too.
	return !f.Ext || c.sameType(fs.Apply(f.Recv), gs.Apply(g.Recv))
}

// overrideSubst returns the substitution that maps the types in method
// g's signature, seen from a receiver of type rt, to those of f, which
// overrides it: the type parameters of generic methods correspond by
// position.
func (c *Checker) overrideSubst(rt types.Type, f, g *types.FunSym) types.Subst {
	gs := c.memberSubst(rt, g.Owner)
	if len(g.TypeParams) == 0 || len(g.TypeParams) != len(f.TypeParams) {
		return gs
	}
	gs2 := make(types.Subst, len(gs)+len(g.TypeParams))
	for p, a := range gs {
		gs2[p] = a
	}
	for i, p := range g.TypeParams {
		gs2[p] = types.Arg{T: types.ParamType(f.TypeParams[i])}
	}
	return gs2
}

// methodsNamed returns a class's methods named name that are like f: its
// member extensions for one, or else its other methods.
func methodsNamed(s *types.ClassSym, f *types.FunSym) []*types.FunSym {
	if f.Ext {
		return s.ExtMethods[f.Name]
	}
	return s.Methods[f.Name]
}

// overridden returns the methods of cls's supertypes that f, a method of
// cls, overrides.
func (c *Checker) overridden(cls *types.ClassSym, f *types.FunSym) []*types.FunSym {
	this := cls.This()
	var out []*types.FunSym
	// Kotlin sees only Any's members of Object: an interface's clone()
	// overrides nothing, unless it is Cloneable's.
	anyOnly := !anyMember[f.Name] && !c.isSubclassOf(cls, "java/lang/Cloneable")
	cls.Supertypes(func(s *types.ClassSym) bool {
		if s == cls || anyOnly && s.Name == "java/lang/Object" {
			return true
		}
		gs := methodsNamed(s, f)
		if s.Name == charSequence && f.Name == "get" && !f.Ext {
			gs = s.Methods["charAt"] // CharSequence's get, on the JVM
		}
		for _, g := range gs {
			// A private member isn't inherited: it overrides and is
			// overridden by nothing.
			if !g.Static && !g.Private && !f.Private && c.sameParams(this, f, g) {
				out = append(out, g)
			}
		}
		return true
	})
	return out
}

// superProp is a property of a supertype that a property may override: a
// property of the program, of a Kotlin class, or of a Kotlin built-in type
// that is a Java class, Throwable's message.
type superProp struct {
	owner     *types.ClassSym
	t         types.Type // as declared, in owner's type parameters
	final     bool
	mutable   bool
	abstract  bool
	getter    string // the getter's JVM name and descriptor
	setter    string // and the setter's, if it has one
	protected bool
	ctx       []*types.VarSym // its context parameters, in owner's type parameters
}

// implementingProp returns the property of cls, declared or inherited,
// that implements a property of its supertype s, of a name and context
// parameters ctx, or nil.
func (c *Checker) implementingProp(cls, s *types.ClassSym, name string, ctx []*types.VarSym) *types.VarSym {
	q := c.userProp(cls, name)
	if len(ctx) == 0 && (q == nil || q.Alt == nil) {
		return q
	}
	this := cls.This()
	want := c.memberSubst(this, s)
	q = nil
	cls.Supertypes(func(o *types.ClassSym) bool {
		if p := o.Prop(name); p != nil && !o.External {
			sub := c.memberSubst(this, o)
			for p := range altProps(p) {
				if c.sameContextsIn(ctx, want, p.Contexts, sub) {
					q = p
					return false
				}
			}
		}
		return true
	})
	return q
}

// overriddenProps returns the properties of cls's supertypes that its
// property p may override: of its name and context parameters.
func (c *Checker) overriddenProps(cls *types.ClassSym, p *types.VarSym) []superProp {
	return slices.DeleteFunc(c.superProps(cls, p.Name), func(o superProp) bool {
		return !c.sameContextsIn(o.ctx, c.memberSubst(cls.This(), o.owner), p.Contexts, nil)
	})
}

// sameContextsIn reports whether context parameters a, with substitution
// sa, and b, with sb, have the same types, in order.
func (c *Checker) sameContextsIn(a []*types.VarSym, sa types.Subst, b []*types.VarSym, sb types.Subst) bool {
	if len(a) != len(b) {
		return false
	}
	for i, p := range a {
		if !c.sameType(sa.Apply(p.Type), sb.Apply(b[i].Type)) {
			return false
		}
	}
	return true
}

// superProps returns the properties of cls's supertypes named name.
func (c *Checker) superProps(cls *types.ClassSym, name string) []superProp {
	var out []superProp
	cls.Supertypes(func(s *types.ClassSym) bool {
		if s == cls {
			return true
		}
		if !s.External {
			// A private property isn't inherited: a subclass's of the
			// same name neither hides nor overrides it.
			if p := s.Prop(name); p != nil {
				for p := range altProps(p) {
					if p.Private {
						continue
					}
					ensureVar(p)
					d, _ := types.Desc(types.GetterType(p))
					var setter string
					if p.Mutable && !p.SetterPrivate {
						fd, _ := types.Desc(types.FieldType(p))
						setter = types.SetterJVM(p) + "(" + fd + ")V"
					}
					out = append(out, superProp{s, p.Type, p.Final, p.Mutable, p.Abstract, types.GetterJVM(p) + "()" + d, setter, protectedProp(p), p.Contexts})
				}
			}
			return true
		}
		if kp := s.KProps[name]; kp != nil {
			for kp := range kpropAlts(kp) {
				var setter string
				if kp.Setter != nil {
					setter = kp.Setter.JVM() + types.FunDesc(kp.Setter)
				}
				out = append(out, superProp{s, kp.Getter.Ret, kp.Getter.Final, kp.Setter != nil, kp.Getter.Abstract, kp.Getter.JVM() + types.FunDesc(kp.Getter), setter, kp.Getter.Protected, kp.Getter.Contexts})
			}
			return true
		}
		if s.Name == charSequence && name == "length" {
			// CharSequence's length, java.lang.CharSequence.length().
			out = append(out, superProp{s, types.IntType, false, false, true, "length()I", "", false, nil})
			return true
		}
		if s.KotlinName != "" || !builtinJava[s.Name] {
			// A Java getter isn't a property that a Kotlin property hides
			// or overrides, as with kotlinc: a Thread's @JvmField var
			// state.
			return true
		}
		// A Kotlin built-in type's, as Throwable's message.
		for _, g := range s.Methods[types.GetterName(name)] {
			if len(g.Params) == 0 && !g.Static && g.Ret.K != types.Unit {
				out = append(out, superProp{s, g.Ret, g.Final, false, g.Abstract, g.JVM() + types.FunDesc(g), "", g.Protected, nil})
			}
		}
		return true
	})
	return out
}

// checkInheritance checks a class's overrides, and that a class that
// isn't abstract implements every abstract member it inherits. It records
// the bridges that overrides need.
func (c *Checker) checkInheritance(d *syntax.ClassDecl) {
	cls := d.Sym
	this := cls.This()
	c.checkExtPropOverrides(d)
	for _, m := range d.Members {
		fd, ok := m.(*syntax.FunDecl)
		if !ok || fd.SelfStatic {
			continue // write$Self is static on the JVM: it overrides nothing
		}
		f := fd.Sym
		over := c.overridden(cls, f)
		// The descriptors below need the return types, which an object
		// expression's members can see before they are inferred.
		ensureFun(f)
		for _, o := range over {
			ensureFun(o)
		}
		switch {
		case len(over) == 0 && fd.Override:
			if c.u.ext.class("java/lang/Object") != nil || !anyMember[f.Name] {
				c.errorf(fd.P, "'%s' overrides nothing", f.Name)
			}
		case len(over) > 0 && !fd.Override && !platformOnly(over):
			c.errorf(fd.P, "'%s' hides member of supertype '%s' and needs 'override' modifier", f.Name, over[0].Owner.DisplayName())
		}
		if !fd.Override {
			continue
		}
		if f.Suspend && rawSuspendRet(f.Ret) {
			for _, o := range over {
				if !rawSuspendRet(o.Ret) || o.BoxedRet {
					f.BoxedRet = true
				}
			}
		}
		if !f.Suspend && f.Ret.IsPrimitive() {
			// As boxedReturns does, for object expressions too.
			for _, o := range over {
				if d := types.FunDesc(o); d != "" && !strings.HasSuffix(d, ")"+primDesc(f.Ret)) {
					f.BoxedRet = true
				}
			}
		}
		own := memberDesc(f)
		for _, o := range over {
			if o.Infix {
				f.Infix = true // an override of an infix function is one
			}
			if o.Final {
				c.errorf(fd.P, "'%s' in '%s' is final and cannot be overridden", f.Name, o.Owner.DisplayName())
				continue
			}
			switch {
			case f.Suspend && !o.Suspend:
				c.errorf(fd.P, "suspend function '%s' cannot override non-suspend function '%s' defined in '%s'", f.Name, f.Name, o.Owner.DisplayName())
				continue
			case !f.Suspend && o.Suspend:
				c.errorf(fd.P, "non-suspend function '%s' cannot override suspend function '%s' defined in '%s'", f.Name, f.Name, o.Owner.DisplayName())
				continue
			case f.Composable != o.Composable && !c.u.compose:
				c.unsupported(fd.P, "compose", "'%s' overrides a @Composable function: compile with -compose", f.Name)
				continue
			case f.Composable != o.Composable:
				c.errorf(fd.P, "@Composable annotation mismatch with overridden function: '%s' defined in '%s'", f.Name, o.Owner.DisplayName())
				continue
			case f.Composable && (f.Changed != o.Changed || f.Defaults != o.Defaults):
				c.unsupported(fd.P, "compose", "'%s' overrides a composable function whose parameters differ", f.Name)
				continue
			}
			ret := c.overrideSubst(this, f, o).Apply(o.Ret)
			if f.Ret.K != types.Invalid && ret.K != types.Invalid && !c.subtype(f.Ret, ret) {
				c.errorf(fd.P, "return type of '%s' is not a subtype of the return type of the overridden member '%s' defined in '%s'", f.Name, f.Name, o.Owner.DisplayName())
			}
			for i, p := range f.Params {
				// Not if f's name is mangled: its bridge unboxes them.
				if types.Unboxed(p.Type) && !types.Mangles(p.Type) && i < len(o.Params) && (!types.IsValue(o.Params[i].Type) || slices.Contains(o.BoxedParams, i)) &&
					!f.Suspend && types.InstanceJVM(f) == o.JVM() && !slices.Contains(f.BoxedParams, i) {
					f.BoxedParams = append(f.BoxedParams, i)
				}
			}
			if od := o.JVM() + memberDesc(o); od != types.InstanceJVM(f)+own && !contains(f.Bridges, od) {
				f.Bridges = append(f.Bridges, od)
				if f.BridgeOf == nil {
					f.BridgeOf = map[string]*types.FunSym{}
				}
				f.BridgeOf[od] = o
				if types.Unboxed(f.Ret) && !types.Unboxed(o.Ret) && !f.Suspend {
					f.BoxingBridges = append(f.BoxingBridges, od)
				}
			}
		}
	}
	type ownProp struct {
		p        *types.VarSym
		override bool
		pos      syntax.Pos
	}
	var props []ownProp
	for _, cp := range d.CtorParams {
		if cp.PropSym != nil { // not an annotation class's parameter
			props = append(props, ownProp{cp.PropSym, cp.Override, cp.P})
		}
	}
	for _, m := range d.Members {
		if pd, ok := m.(*syntax.PropDecl); ok && pd.Recv == nil {
			props = append(props, ownProp{pd.Sym, pd.Override, pd.P})
		}
	}
	for _, op := range props {
		p := op.p
		if p.Private && !op.override {
			continue // only a field: it hides and overrides nothing
		}
		over := c.overriddenProps(cls, p)
		switch {
		case len(over) == 0 && op.override:
			c.errorf(op.pos, "'%s' overrides nothing", p.Name)
		case len(over) > 0 && !op.override:
			c.errorf(op.pos, "'%s' hides member of supertype '%s' and needs 'override' modifier", p.Name, over[0].owner.DisplayName())
		}
		if !op.override {
			continue
		}
		own := types.GetterJVM(p) + "()" + descOf(types.GetterType(p))
		var ownSetter string
		if p.Mutable && !p.SetterPrivate {
			ownSetter = types.SetterJVM(p) + "(" + descOf(types.FieldType(p)) + ")V"
		}
		for _, o := range over {
			t := c.memberSubst(this, o.owner).Apply(o.t)
			switch {
			case o.final:
				c.errorf(op.pos, "'%s' in '%s' is final and cannot be overridden", p.Name, o.owner.DisplayName())
				continue
			case o.mutable && !p.Mutable:
				c.errorf(op.pos, "val-property cannot override var-property '%s'", p.Name)
			case p.Type.K == types.Invalid || t.K == types.Invalid:
			case p.Mutable && !c.sameType(p.Type, t):
				c.errorf(op.pos, "type of '%s' doesn't match the type of the overridden var-property", p.Name)
			case !c.subtype(p.Type, t):
				c.errorf(op.pos, "type of '%s' is not a subtype of the overridden property", p.Name)
			}
			if o.getter != own && !contains(p.Bridges, o.getter) {
				p.Bridges = append(p.Bridges, o.getter)
			}
			if o.setter != "" && o.setter != ownSetter && ownSetter != "" && !contains(p.SetterBridges, o.setter) {
				p.SetterBridges = append(p.SetterBridges, o.setter)
			}
			if o.protected && !explicitVisibility(p) {
				p.InheritsProtected = true
			}
		}
	}
	c.inheritedBridges(d)
	if !cls.Abstract || d.Enum {
		c.checkImplemented(d)
	}
}

// inheritedBridges finds the members of class d's interfaces that it
// doesn't declare and that a member of a superclass implements with
// another JVM descriptor: as kotlinc does, the class gets a bridge that
// calls it, unless a superclass implements the interface and has one.
func (c *Checker) inheritedBridges(d *syntax.ClassDecl) {
	cls := d.Sym
	if cls.Interface || cls.Super == nil {
		return
	}
	this := cls.This()
	seen := map[string]bool{}
	add := func(b syntax.InheritedBridge, target string) {
		if !seen[b.Bridge] && b.Bridge != target {
			d.InheritedBridges = append(d.InheritedBridges, b)
		}
		seen[b.Bridge] = true
	}
	cls.Supertypes(func(s *types.ClassSym) bool {
		if !s.Interface || cls.Super.IsSubclassOf(s) {
			return true
		}
		for _, name := range slices.Sorted(maps.Keys(s.Methods)) {
			for _, g := range s.Methods[name] {
				if g.Static || g.Platform || g.Ext || c.ownMethod(d, this, g) {
					continue
				}
				ensureFun(g)
				od := g.JVM() + memberDesc(g)
				if seen[od] {
					continue
				}
				if h := c.superImpl(cls, this, g); h != nil {
					ensureFun(h)
					add(syntax.InheritedBridge{Bridge: od, Target: types.InstanceJVM(h), Params: append(slices.Clone(h.Contexts), h.Params...), Ret: types.JVMRet(h), Over: g, Fun: h}, types.InstanceJVM(h)+memberDesc(h))
				}
			}
		}
		type accessors struct{ getter, setter string } // JVM names and descriptors
		props := map[string]accessors{}
		if !s.External {
			for _, p := range s.Props {
				a := accessors{getter: types.GetterJVM(p) + "()" + descOf(types.GetterType(p))}
				if p.Mutable && !p.SetterPrivate {
					a.setter = types.SetterJVM(p) + "(" + descOf(types.FieldType(p)) + ")V"
				}
				props[p.Name] = a
			}
		} else {
			for name, kp := range s.KProps {
				a := accessors{getter: kp.Getter.JVM() + types.FunDesc(kp.Getter)}
				if kp.Setter != nil {
					a.setter = kp.Setter.JVM() + types.FunDesc(kp.Setter)
				}
				props[name] = a
			}
		}
		for _, name := range slices.Sorted(maps.Keys(props)) {
			od := props[name]
			if seen[od.getter] || cls.Prop(name) != nil && cls.Prop(name).Owner == cls {
				continue
			}
			for sc := cls.Super; sc != nil; sc = sc.Super {
				if !sc.External {
					if p := sc.Prop(name); p != nil && p.Owner == sc && !p.Abstract {
						add(syntax.InheritedBridge{Bridge: od.getter, Target: types.GetterJVM(p), Ret: types.GetterType(p)}, types.GetterJVM(p)+"()"+descOf(types.GetterType(p)))
						if od.setter != "" && p.Mutable && !p.SetterPrivate {
							ps := []*types.VarSym{{Name: "value", Type: types.FieldType(p), Kind: types.Param}}
							add(syntax.InheritedBridge{Bridge: od.setter, Target: types.SetterJVM(p), Params: ps, Ret: types.UnitType}, types.SetterJVM(p)+"("+descOf(types.FieldType(p))+")V")
						}
						break
					}
				} else if kp := sc.KProps[name]; kp != nil && !kp.Getter.Abstract {
					add(syntax.InheritedBridge{Bridge: od.getter, Target: kp.Getter.JVM(), Ret: types.JVMRet(kp.Getter)}, kp.Getter.JVM()+types.FunDesc(kp.Getter))
					if st := kp.Setter; od.setter != "" && st != nil {
						add(syntax.InheritedBridge{Bridge: od.setter, Target: st.JVM(), Params: st.Params, Ret: types.UnitType}, st.JVM()+types.FunDesc(st))
					}
					break
				}
			}
		}
		return true
	})
}

// superImpl returns the member of a superclass of class cls, of type
// this, that implements interface member g, or nil.
func (c *Checker) superImpl(cls *types.ClassSym, this types.Type, g *types.FunSym) *types.FunSym {
	for sc := cls.Super; sc != nil; sc = sc.Super {
		sc.Ensure()
		for _, h := range methodsNamed(sc, g) {
			if h.Owner == sc && !h.Static && !h.Abstract && c.sameParams(this, h, g) {
				return h
			}
		}
	}
	return nil
}

// protectedProp reports whether a property of the program is protected.
func protectedProp(p *types.VarSym) bool {
	switch d := p.Decl.(type) {
	case *syntax.PropDecl:
		return d.Visibility == syntax.Protected || p.InheritsProtected
	case *syntax.CtorParam:
		return d.Visibility == syntax.Protected || p.InheritsProtected
	}
	return false
}

// explicitVisibility reports whether a property of the program is declared
// with a visibility modifier.
func explicitVisibility(p *types.VarSym) bool {
	switch d := p.Decl.(type) {
	case *syntax.PropDecl:
		return d.Explicit
	case *syntax.CtorParam:
		return d.Explicit
	}
	return true
}

// charSequence is the class of kotlin.CharSequence, whose length and
// get(index) are length() and charAt(index) on the JVM: a class that
// implements it overrides them, and has those methods too, as bridges.
const charSequence = "java/lang/CharSequence"

// anyMember are the members of Any, which can be overridden even when the
// JDK, which declares them, isn't available.
var anyMember = map[string]bool{"equals": true, "hashCode": true, "toString": true}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func descOf(t types.Type) string {
	d, _ := types.Desc(t)
	return d
}

// checkImplemented reports the abstract members that a class that isn't
// abstract inherits and doesn't implement.
func (c *Checker) checkImplemented(d *syntax.ClassDecl) {
	cls := d.Sym
	this := cls.This()
	reported := map[string]bool{}
	missing := func(name string) {
		if !reported[name] {
			reported[name] = true
			if d.Enum {
				// An enum with abstract members is abstract, and its entries
				// implement them in their bodies.
				cls.Abstract = true
				for _, e := range d.Entries {
					if e.Body == nil {
						c.errorf(e.P, "enum entry '%s.%s' does not implement abstract member '%s'", cls.DisplayName(), e.Name, name)
					}
				}
				return
			}
			if cls.Anonymous {
				c.errorf(d.P, "object is not abstract and does not implement abstract member '%s'", name)
			} else {
				c.errorf(d.P, "class '%s' is not abstract and does not implement abstract member '%s'", cls.DisplayName(), name)
			}
		}
	}
	if d.Enum {
		for _, m := range d.Members {
			switch m := m.(type) {
			case *syntax.FunDecl:
				if m.Abstract {
					missing(m.Name)
				}
			case *syntax.PropDecl:
				if m.Abstract {
					missing(m.Name)
				}
			}
		}
	}
	// A class that implements Kotlin's collection types implements the
	// abstract methods of Java's collection classes it extends with their
	// Kotlin members, under other names: Java's AbstractList.size() with
	// the property size. A member of the collection types themselves is
	// implemented by an override or a method of a Java superclass, as
	// ArrayList.size() implements the size of a MutableList.
	mapped, inlineReported := false, false
	cls.Supertypes(func(s *types.ClassSym) bool {
		mapped = mapped || s.Java != nil
		return !mapped
	})
	cls.Supertypes(func(s *types.ClassSym) bool {
		if s == cls || mapped && s.External && s.KotlinName == "" && s.Java == nil {
			return true
		}
		for name, fs := range s.Methods {
			for _, g := range fs {
				if !g.Abstract || g.Static || s.Java != nil && g.Platform {
					continue
				}
				if !c.implemented(cls, this, g) && !(s.Java != nil && concreteInSuper(cls, g.JVM()+types.FunDesc(g))) {
					missing(name)
				}
				if s.Interface && hasDefaults(g) && !inlineReported {
					// The intersection override of an inline function of a
					// superclass and g, whose default values it inherits:
					// kotlinc's backend fails on its calls (KT-49722).
					if h := c.implementation(cls, this, g); h != nil && h.Owner != cls && !hasDefaults(h) && isInlineFun(h) {
						c.errorf(d.P, "functional parameters with inherited default values are not yet supported in inline functions")
						inlineReported = true
					}
				}
			}
		}
		for name, fs := range s.ExtMethods {
			for _, g := range fs {
				if g.Abstract && !c.implemented(cls, this, g) {
					missing(name)
				}
			}
		}
		if !s.External {
			for _, p := range s.Props {
				if p.Abstract {
					if q := c.implementingProp(cls, s, p.Name, p.Contexts); q == nil || q.Abstract {
						missing(p.Name)
					}
				}
			}
		}
		for name, kp := range s.KProps {
			for kp := range kpropAlts(kp) {
				if kp.Getter.Abstract {
					if q := c.implementingProp(cls, s, name, kp.Getter.Contexts); (q == nil || q.Abstract) && !concreteInSuper(cls, kp.Getter.JVM()+types.FunDesc(kp.Getter)) {
						missing(name)
					}
				}
			}
		}
		return true
	})
}

// hasDefaults reports whether a function has parameters with default
// values, declared or inherited.
func hasDefaults(f *types.FunSym) bool {
	return slices.ContainsFunc(f.Params, func(p *types.VarSym) bool { return p.Default })
}

// isInlineFun reports whether f is an inline function, of the program or
// of the class path.
func isInlineFun(f *types.FunSym) bool {
	if d, ok := f.Decl.(*syntax.FunDecl); ok {
		return d.Inline
	}
	return f.Inline != nil
}

// concreteInSuper reports whether a superclass of cls on the class path
// has a method with code with this name and descriptor.
func concreteInSuper(cls *types.ClassSym, nd string) bool {
	for s := cls.Super; s != nil; s = s.Super {
		s.Ensure()
		if acc, ok := s.JVMMethods[nd]; ok && acc&classfile.AccAbstract == 0 {
			return true
		}
	}
	return false
}

// implemented reports whether cls has a method that implements g: one
// that isn't abstract and has g's parameters, declared in a class that
// inherits g's, or in a superclass, as Object's equals implements
// Comparator's.
func (c *Checker) implemented(cls *types.ClassSym, this types.Type, g *types.FunSym) bool {
	if g.Owner != nil && g.Owner.Name == charSequence && g.Name == "length" {
		// Kotlin's CharSequence has length and get(index).
		if p := c.userProp(cls, "length"); p != nil && !p.Abstract {
			return true
		}
	}
	return c.implementation(cls, this, g) != nil
}

// implementation returns the method of cls that implements g, as
// implemented finds it, or nil.
func (c *Checker) implementation(cls *types.ClassSym, this types.Type, g *types.FunSym) *types.FunSym {
	name := g.Name
	if g.Owner != nil && g.Owner.Name == charSequence && name == "charAt" {
		name = "get"
	}
	var found *types.FunSym
	cls.Supertypes(func(s *types.ClassSym) bool {
		if s == g.Owner {
			return true
		}
		hs := methodsNamed(s, g)
		if name != g.Name {
			hs = append(hs[:len(hs):len(hs)], s.Methods[name]...)
		}
		for _, h := range hs {
			if !h.Abstract && c.sameParams(this, h, g) && (!s.Interface || s.IsSubclassOf(g.Owner)) {
				found = h
			}
		}
		return found == nil
	})
	return found
}

// superCtor checks a class's call of its superclass's constructor, B(x) in
// class A(x: Int) : B(x). Constructor parameters are in scope, but not the
// class's members: the object isn't initialized yet.
func (c *Checker) superCtor(d *syntax.ClassDecl) {
	x := d.SuperCall
	sc := d.Sym.Super
	c.state = state{scope: c.ctorScope(d.Sym), tparams: d.Sym.TypeParams, staticCls: d.Sym}
	c.inAnon(d.Sym)
	c.lookupCls = d.Sym // its nested classes
	defer func() { c.lookupCls = nil }()
	cands := superCands(d.Sym)
	if len(cands) == 0 {
		c.errorf(x.P, "%s has no constructor that a subclass can call", sc.DisplayName())
		c.args(x, nil)
		return
	}
	x.Ctor = sc
	if !c.superOuter(d.Sym, x) {
		return
	}
	c.resolve(x, cands, noWant, nil)
	if sc.Local {
		c.localConstructed(sc) // the class passes its superclass's captures
	}
}

// superTarget checks super or super<I> as a receiver of the member name,
// and returns the supertype whose members it refers to. Without <I>, as
// in Kotlin, it is the supertype that declares the member: the
// superclass, or an interface. super@Outer is an outer class's, whose
// instance the code reaches as this@Outer's.
func (c *Checker) superTarget(s *syntax.Super, name string, call bool) (types.Type, bool) {
	cls := c.cls
	switch {
	case s.Label != "" && (cls == nil || s.Label != simpleName(cls)):
		r := c.outerRecv(s.Label)
		if r == nil {
			c.errorf(s.P, "unresolved label: %s", s.Label)
			return types.InvalidType, false
		}
		s.Outer = recvThis(r, s.P)
		c.value(s.Outer, noWant)
		cls = r.sym.Type.C
	case cls == nil || cls.Decl == nil:
		c.errorf(s.P, "'super' is not an expression, it can only be used on the left-hand side of a dot ('.')")
		return types.InvalidType, false
	default:
		c.captureThis()
	}
	var t types.Type
	switch {
	case s.Iface != nil:
		// The supertype's class, written without type arguments, which
		// the supertype gives: super<List>.
		var want types.Type
		if ref := s.Iface; ref.Args == nil && ref.Ret == nil && ref.Name != "Any" {
			if wc := c.lookupClass(ref.Name); wc != nil {
				want = types.ClassType(wc)
			}
		}
		if want.K == types.Invalid {
			want = c.resolveType(s.Iface)
		}
		if want.K == types.Invalid {
			return want, false
		}
		for _, st := range cls.Supers {
			if st.C == want.C || want.K == types.Any {
				t = st
			}
		}
		if t.K == types.Invalid {
			c.errorf(s.Iface.P, "not an immediate supertype")
			return t, false
		}
	default:
		var has []types.Type
		for _, st := range cls.Supers {
			if c.superHas(st.C, name, call) {
				has = append(has, st)
			}
		}
		if len(has) > 1 {
			// Only the implementations count when some are abstract.
			var impl []types.Type
			for _, st := range has {
				if c.superHasImpl(st.C, name, call) {
					impl = append(impl, st)
				}
			}
			if len(impl) > 0 {
				has = impl
			}
		}
		if len(has) > 1 {
			c.errorf(s.P, "many supertypes available, please specify the one you mean in angle brackets, e.g. 'super<%s>'", has[0].C.DisplayName())
			return types.InvalidType, false
		}
		if len(has) == 1 {
			t = has[0]
			break
		}
		for _, st := range cls.Supers {
			if !st.C.Interface {
				t = st
			}
		}
		if t.K == types.Invalid && len(cls.Supers) == 1 {
			t = cls.Supers[0]
		}
		if t.K == types.Invalid {
			if cls.Super == nil {
				c.errorf(s.P, "'super' is not available here")
				return t, false
			}
			t = types.ClassType(cls.Super)
		}
	}
	s.Cls, s.T = t.C, t
	return t, true
}

// simpleName returns the name of class cls without its outer classes'.
func simpleName(cls *types.ClassSym) string {
	n := cls.DisplayName()
	return n[strings.LastIndexByte(n, '.')+1:]
}

// superHas reports whether the supertype cls has a function (call) or a
// property named name.
func (c *Checker) superHas(cls *types.ClassSym, name string, call bool) bool {
	if call {
		for _, f := range c.accessible(c.javaMethods(cls, name)) {
			// An interface's equals, hashCode and toString are Any's.
			if !cls.Interface || f.Owner == nil || f.Owner.Name != "java/lang/Object" {
				return true
			}
		}
		return false
	}
	if c.userProp(cls, name) != nil {
		return true
	}
	return len(c.javaMethods(cls, types.GetterName(name))) > 0
}

// superHasImpl reports whether the supertype cls has a function or a
// property named name that isn't abstract.
func (c *Checker) superHasImpl(cls *types.ClassSym, name string, call bool) bool {
	if call {
		// Any class in the hierarchy may implement it: javaMethods gives
		// the first method of each signature, which may be an interface's,
		// as Set.addAll for AbstractMutableSet, whose implementation is
		// AbstractCollection's.
		found := false
		cls.Supertypes(func(s *types.ClassSym) bool {
			for _, f := range c.accessible(s.Methods[name]) {
				if !f.Abstract {
					found = true
				}
			}
			return !found
		})
		return found
	}
	if p := c.userProp(cls, name); p != nil {
		return !p.Abstract
	}
	for _, f := range c.javaMethods(cls, types.GetterName(name)) {
		if !f.Abstract {
			return true
		}
	}
	return false
}

// superCall checks super.f(args).
func (c *Checker) superCall(x *syntax.Call, fn *syntax.Member, s *syntax.Super, want types.Type, sink *system) types.Type {
	st, ok := c.superTarget(s, fn.Name, true)
	if !ok {
		c.args(x, nil)
		return types.InvalidType
	}
	fn.T = st
	if !st.C.External && innerOf(st.C, fn.Name) != nil {
		// super.Inner() constructs the supertype's inner class with this
		// as its outer instance.
		this := &syntax.This{Label: s.Label}
		this.P = s.P
		fn.X = this
		return c.call(x, want, sink)
	}
	ms := c.accessible(c.javaMethods(st.C, fn.Name))
	if len(ms) == 0 && st.C.Interface {
		if object := c.u.ext.class("java/lang/Object"); object != nil {
			ms = c.javaMethods(object, fn.Name)
		}
	}
	if len(ms) == 0 {
		c.errorf(fn.P, "unresolved reference: %s", fn.Name)
		c.args(x, nil)
		return types.InvalidType
	}
	t := c.resolve(x, c.memberCallees(c.superImpls(st.C, ms), st), want, sink)
	if x.Sym != nil && x.Sym.Abstract {
		c.errorf(fn.P, "abstract member cannot be accessed directly")
	}
	return t
}

// superImpls replaces the abstract methods among ms, members of supertype
// cls, by the implementations of the same signature that cls inherits:
// AbstractCollection.addAll for an AbstractMutableSet's Set.addAll.
func (c *Checker) superImpls(cls *types.ClassSym, ms []*types.FunSym) []*types.FunSym {
	var out []*types.FunSym
	for _, m := range ms {
		if m.Abstract {
			cls.Supertypes(func(s *types.ClassSym) bool {
				for _, f := range s.Methods[m.Name] {
					if !f.Abstract && paramKey(f) == paramKey(m) {
						m = f
						return false
					}
				}
				return true
			})
		}
		out = append(out, m)
	}
	return out
}

// superMember checks super.p: a property of a supertype.
func (c *Checker) superMember(x *syntax.Member, s *syntax.Super) types.Type {
	st, ok := c.superTarget(s, x.Name, false)
	if !ok {
		return types.InvalidType
	}
	if p := c.userProp(st.C, x.Name); p != nil {
		if p.Abstract {
			c.errorf(x.P, "abstract member cannot be accessed directly")
		}
		ensureVar(p)
		x.Prop = p
		return c.memberSubst(st, p.Owner).Apply(p.Type)
	}
	if t, ok := c.javaProperty(x, st); ok {
		return t
	}
	c.errorf(x.P, "unresolved reference: %s", x.Name)
	return types.InvalidType
}

// inheritedGetter finds a property that the current class inherits from a
// Java or Kotlin class, used by its simple name: message in a subclass of
// Exception.
func (c *Checker) inheritedGetter(x *syntax.Ident) (types.Type, bool) {
	if c.cls == nil {
		return types.InvalidType, false
	}
	m := &syntax.Member{Name: x.Name}
	m.P = x.P
	t, ok := c.javaProperty(m, c.cls.This())
	if !ok || m.Getter == nil {
		return types.InvalidType, false
	}
	c.captureThis()
	x.Getter = m.Getter
	return t, true
}

// rawSuspendRet reports whether a suspend function with result type t
// returns it unboxed unless it overrides one that doesn't: a value class
// over a reference type, as kotlinc compiles it.
func rawSuspendRet(t types.Type) bool {
	return types.Unboxed(t) && !t.N && !types.Repr(t).IsPrimitive()
}

// boxedReturns sets BoxedRet on the overrides whose result is a primitive
// where a member they override returns a reference on the JVM, a type
// parameter or a nullable type, as kotlinc boxes them: Integer get() for
// an Int override of T get(). The members' types are all resolved then.
func (c *Checker) boxedReturns() {
	for _, d := range c.classes {
		cls := d.Sym
		for _, m := range d.Members {
			fd, ok := m.(*syntax.FunDecl)
			if !ok || !fd.Override || fd.Sym == nil || fd.Sym.Suspend || !fd.Sym.Ret.IsPrimitive() {
				continue
			}
			for _, o := range c.overridden(cls, fd.Sym) {
				if d := types.FunDesc(o); d != "" && !strings.HasSuffix(d, ")"+primDesc(fd.Sym.Ret)) {
					fd.Sym.BoxedRet = true
				}
			}
		}
		var props []*types.VarSym
		for _, cp := range d.CtorParams {
			if cp.PropSym != nil && cp.Override {
				props = append(props, cp.PropSym)
			}
		}
		for _, m := range d.Members {
			if pd, ok := m.(*syntax.PropDecl); ok && pd.Override && pd.Recv == nil && pd.Sym != nil {
				props = append(props, pd.Sym)
			}
		}
		for _, p := range props {
			if !p.Type.IsPrimitive() {
				continue
			}
			for _, o := range c.overriddenProps(cls, p) {
				if o.getter != "" && !strings.HasSuffix(o.getter, ")"+primDesc(p.Type)) {
					p.BoxedGetter = true
					if p.Getter != nil {
						p.Getter.BoxedRet = true
					}
				}
			}
		}
	}
}

// primDesc returns the descriptor of a primitive type.
func primDesc(t types.Type) string {
	d, _ := types.Desc(t)
	return d
}

// instanceField returns a public instance field of a Java class or its
// superclasses, which Kotlin sees as a property.
// visibleField reports whether f, a Java class's instance field or nil,
// is visible to the code: a protected one is to its class's subclasses,
// and else a getter with its name is called, Calendar's getTime() for
// calendar.time.
func (c *Checker) visibleField(f *types.VarSym) bool {
	return f != nil && (!f.Protected || c.inSubclass(f.Owner))
}

func instanceField(cls *types.ClassSym, name string) *types.VarSym {
	var f *types.VarSym
	for s := cls; s != nil && f == nil; s = s.Super {
		s.Ensure()
		f = s.JavaFields[name]
	}
	return f
}

// memberDesc returns the JVM descriptor of method f, with its context
// parameters and a member extension's receiver first: a class file's
// descriptors have them, and the program's functions' leave them out.
func memberDesc(f *types.FunSym) string {
	d := types.FunDesc(f)
	if f.Ext && f.Desc == "" && d != "" {
		r, ok := types.ParamDesc(f.Recv)
		if !ok {
			return d
		}
		d = "(" + r + d[1:]
	}
	if len(f.Contexts) > 0 && f.Desc == "" && d != "" {
		// Its context parameters come first.
		cd, ok := types.ParamsDesc(f.Contexts)
		if !ok {
			return d
		}
		d = cd[:len(cd)-1] + d[1:]
	}
	return d
}
