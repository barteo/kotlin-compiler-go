package check

import (
	"fmt"
	"github.com/barteo/kotlin-compiler-go/internal/classfile"
	"maps"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

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
		c.supersOf = nil
	}()
	object := c.u.ext.class("java/lang/Object")
	for _, cd := range c.classes {
		cls := cd.Sym
		// The supertypes don't see the class's own nested classes: a
		// data class Reply(val page: Page) : Page, with a nested Page,
		// implements the outer one. An object expression's or
		// a local class's, which has none, see the enclosing function's
		// type parameters through it.
		c.tparams, c.lookupCls, c.supersOf = cls.TypeParams, cls.Outer, cls
		if cls.Anonymous || cls.Local {
			c.lookupCls = cls
		}
		cls.Annotations = c.classAnnotations(cd.Annotations)
		cls.RestrictsSuspension = slices.ContainsFunc(cls.Annotations, func(a *types.ClassSym) bool { return a.Name == "kotlin/coroutines/RestrictsSuspension" })
		cls.Supers = []types.Type{}
		seen := map[*types.ClassSym]bool{}
		for _, st := range cd.Supers {
			if st.Delegate != nil && cd.Interface {
				c.errorf(st.Type.P, "DELEGATION_IN_INTERFACE", "delegation cannot be used in interfaces")
				st.Delegate = nil
			}
			if st.Delegate != nil && !cd.HasPrimary {
				// The delegate is a field the primary constructor sets.
				c.errorf(st.Type.P, "UNSUPPORTED", "delegation without primary constructor is unsupported")
				st.Delegate = nil
			}
			t := c.supertype(cd, st, seen)
			if t.E || t.X > 0 {
				c.errorf(st.Type.P, "SUPERTYPE_IS_EXTENSION_OR_CONTEXT_FUNCTION_TYPE", "extension function type or function type with context parameters is not allowed as supertype")
			}
			if t.IsComposableFunc() {
				c.unsupported(st.Type.P, "compose", "classes that implement composable function types are not supported")
				t = types.InvalidType
			}
			st.T = t
			if t.K != types.Class {
				continue
			}
			if st.Call == nil {
				c.superProjections(st, t)
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
		if cls.Super == nil && c.jvmRecordClass(cd) {
			// A record extends java.lang.Record, as kotlinc has it.
			if rec := c.u.ext.class("java/lang/Record"); rec != nil {
				rec.Ensure()
				cls.Super = rec
				cls.Supers = append([]types.Type{rec.This()}, cls.Supers...)
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
		if c.errorSupers == nil {
			c.errorSupers = map[*syntax.ClassDecl]bool{}
		}
		c.errorSupers[cd] = true
		return t
	case t.K == types.Any:
		if cd.Interface {
			c.errorf(st.Type.P, "INTERFACE_WITH_SUPERCLASS", "an interface cannot inherit from a class")
			c.interfaceSuper(cd, c.u.ext.class("java/lang/Object"))
		}
		return types.InvalidType // the default superclass
	case t.N:
		c.errorf(st.Type.P, "NULLABLE_SUPERTYPE", "a supertype cannot be nullable")
		return types.InvalidType
	case t.K != types.Class:
		c.errorf(st.Type.P, "SUPERTYPE_NOT_A_CLASS_OR_INTERFACE", "this type is final, so it cannot be inherited from")
		return types.InvalidType
	}
	sc := t.C
	sc.Ensure()
	if sc.JavaSealed {
		// javac checks that the subclasses it permits extend it, which
		// it doesn't see a Kotlin class do.
		c.errorf(st.Type.P, "CLASS_INHERITS_JAVA_SEALED_CLASS", "a Kotlin class can't inherit from sealed Java class or interface '%s'", sc.DisplayName())
	}
	switch {
	case seen[sc]:
		c.errorf(st.Type.P, "SUPERTYPE_APPEARS_TWICE", "a supertype appears twice")
	case cd.Enum && !sc.Interface:
		c.errorf(st.Type.P, "CLASS_IN_SUPERTYPE_FOR_ENUM", "an enum class cannot inherit from a class")
	case sc.Name == "java/lang/Enum" && !cd.Interface:
		// Only an enum class extends kotlin.Enum. As in FIR, the class
		// still inherits from it.
		c.errorf(st.Type.P, "CLASS_CANNOT_BE_EXTENDED_DIRECTLY", "class 'Enum' cannot be extended directly")
		return t
	case sc.Interface:
		if st.Call != nil {
			c.errorf(st.Call.P, "NO_CONSTRUCTOR", "this type has no constructor, and thus must not be called")
			return types.InvalidType
		}
		return t
	case st.Delegate != nil:
		c.errorf(st.Type.P, "DELEGATION_NOT_TO_INTERFACE", "delegation is supported only for interfaces")
	case cd.Interface:
		c.errorf(st.Type.P, "INTERFACE_WITH_SUPERCLASS", "an interface cannot inherit from a class")
		c.interfaceSuper(cd, sc)
	case cls.Super != nil:
		c.errorf(st.Type.P, "MANY_CLASSES_IN_SUPERTYPE_LIST", "only one class may appear in a supertype list")
	case sc.Final:
		// As in FIR, the class still inherits from it.
		name := "FINAL_SUPERTYPE"
		if sc.Object {
			name = "SINGLETON_IN_SUPERTYPE"
		}
		c.errorf(st.Type.P, name, "this type is final, so it cannot be inherited from")
		return t
	case !cd.HasPrimary && st.Call != nil:
		c.errorf(st.Type.P, "SUPERTYPE_INITIALIZED_WITHOUT_PRIMARY_CONSTRUCTOR", "supertype initialization is impossible without a primary constructor")
		if c.superCallNoPrimary == nil {
			c.superCallNoPrimary = map[*syntax.ClassDecl]bool{}
		}
		c.superCallNoPrimary[cd] = true
		for _, sc := range cd.Secondaries() {
			if len(sc.Params) == 0 {
				// kotlinc's class still has a primary constructor, which
				// a secondary one without parameters conflicts with.
				c.errorf(cd.P, "CONFLICTING_OVERLOADS", "conflicting overloads: constructor %s()", cd.Name)
				c.errorf(sc.P, "CONFLICTING_OVERLOADS", "conflicting overloads: constructor %s()", cd.Name)
				break
			}
		}
		st.Call = nil
		return t
	case st.Call == nil && cd.HasPrimary:
		c.errorf(st.Type.P, "SUPERTYPE_NOT_INITIALIZED", "this type has a constructor, and thus must be initialized here")
		return t
	default:
		return t
	}
	return types.InvalidType
}

// interfaceSuper records sc, a class interface cd inherits from, which is
// reported (Checker.interfaceSupers).
func (c *Checker) interfaceSuper(cd *syntax.ClassDecl, sc *types.ClassSym) {
	if sc == nil {
		return
	}
	if c.interfaceSupers == nil {
		c.interfaceSupers = map[*syntax.ClassDecl][]*types.ClassSym{}
	}
	c.interfaceSupers[cd] = append(c.interfaceSupers[cd], sc)
}

// interfaceSuperFun returns a function of name and arity n of a class
// interface d inherits from (Checker.interfaceSupers), or nil.
func (c *Checker) interfaceSuperFun(d *syntax.ClassDecl, name string, n int) *types.FunSym {
	for _, sc := range c.interfaceSupers[d] {
		for _, m := range c.javaMethods(sc, name) {
			if !m.Static && len(m.Params) == n {
				return m
			}
		}
	}
	return nil
}

// findCycles finds the file's classes that inherit from themselves. It
// runs for all files before checkSupers breaks the cycles, so each class
// in a cycle is reported.
func (c *Checker) findCycles() {
	walk := c.u.ext.javaSource
	if c.u.shadowing {
		// The class path's classes may inherit from the program's that
		// take the place of theirs (see classTable.shadow):
		//
		//	// MODULE: dependency
		//	interface ToSubstitute
		//	interface Intermediate : ToSubstitute
		//	// MODULE: main(dependency)
		//	interface ToSubstitute : Intermediate // a cycle
		walk = func(*types.ClassSym) bool { return true }
	}
	for _, cd := range c.classes {
		if !inherits(cd.Sym, cd.Sym, walk, map[*types.ClassSym]bool{}) {
			continue
		}
		c.cyclic = append(c.cyclic, cd)
		c.cyclicAlias(cd, walk)
		var keep []types.Type
		for _, t := range cd.Sym.Supers {
			if t.K != types.Class || t.C != cd.Sym && !inherits(t.C, cd.Sym, walk, map[*types.ClassSym]bool{}) {
				keep = append(keep, t)
			}
		}
		if c.cyclicKeep == nil {
			c.cyclicKeep = map[*syntax.ClassDecl][]types.Type{}
		}
		c.cyclicKeep[cd] = keep
		if !c.u.shadowing {
			continue
		}
		// A cycle through the class path is reported on the supertypes
		// that lead back, as kotlinc does, before checkSupers breaks it.
		if c.cyclicAt == nil {
			c.cyclicAt = map[*syntax.ClassDecl][]syntax.Pos{}
		}
		for _, st := range cd.Supers {
			if st.Type != nil && st.T.K == types.Class && st.T.C != nil &&
				(st.T.C == cd.Sym || inherits(st.T.C, cd.Sym, walk, map[*types.ClassSym]bool{})) {
				c.cyclicAt[cd] = append(c.cyclicAt[cd], st.Type.P)
			}
		}
	}
}

// cyclicAlias reports the typealiases among the supertypes of class cd,
// in a cycle, that expand to a class nested in one of the cycle: kotlinc
// can't expand them (RECURSIVE_TYPEALIAS_EXPANSION).
//
//	class A : B() { open class Nested<T> }
//	typealias ANested<T> = A.Nested<T>
//	open class B : ANested<Int>()
func (c *Checker) cyclicAlias(cd *syntax.ClassDecl, walk func(*types.ClassSym) bool) {
	saved := c.lookupCls
	c.lookupCls = cd.Sym.Outer // as the supertypes are resolved
	defer func() { c.lookupCls = saved }()
	for _, st := range cd.Supers {
		if st.Type == nil || st.T.K != types.Class {
			continue
		}
		a := c.lookupAlias(st.Type.Name)
		if a == nil || a.Decl == nil || c.u.aliasFile[a] == nil {
			continue
		}
		d := a.Decl.(*syntax.TypeAliasDecl)
		if st.T.C == cd.Sym {
			// class A : S of typealias S = A.
			c.u.aliasFile[a].errorf(d.Type.P, "RECURSIVE_TYPEALIAS_EXPANSION", "recursive type alias in expansion: %s", a.Name)
			continue
		}
		for o := st.T.C.Outer; o != nil; o = o.Outer {
			if o == cd.Sym || inherits(o, cd.Sym, walk, map[*types.ClassSym]bool{}) {
				c.u.aliasFile[a].errorf(d.Type.P, "RECURSIVE_TYPEALIAS_EXPANSION", "recursive type alias in expansion: %s", a.Name)
				break
			}
		}
	}
}

// checkSupers runs once every class's supertypes are known: it breaks
// cycles in the hierarchy, and checks the bounds of supertypes' type
// arguments.
func (c *Checker) checkSupers() {
	for _, cd := range c.cyclic {
		at := c.cyclicAt[cd]
		if len(at) == 0 {
			at = []syntax.Pos{cd.P}
		}
		for _, p := range at {
			c.errorf(p, "CYCLIC_INHERITANCE_HIERARCHY", "there's a cycle in the inheritance hierarchy for this type")
		}
		cls := cd.Sym
		super := cls.Super
		cls.Super, cls.Interfaces, cls.Supers = c.u.ext.class("java/lang/Object"), nil, []types.Type{}
		for _, t := range c.cyclicKeep[cd] {
			cls.Supers = append(cls.Supers, t)
			if t.K != types.Class {
				continue
			}
			if t.C.Interface {
				cls.Interfaces = append(cls.Interfaces, t.C)
			} else {
				cls.Super = t.C
			}
		}
		if cls.Super != super {
			cd.SuperCall = nil
		}
	}
	for _, cd := range c.classes {
		c.tparams, c.lookupCls = cd.Sym.TypeParams, cd.Sym
		c.checkConsistentSupers(cd)
		for _, st := range cd.Supers {
			c.javaCycle(st)
			if st.Type != nil && st.Type.Ret != nil {
				for _, p := range st.Type.Named {
					c.errorf(p, "UNSUPPORTED", "named parameter in function type as supertype is unsupported")
				}
			}
			if st.T.K == types.Class && st.T.A != nil {
				c.lookupCls = cd.Sym.Outer // as the supertypes are resolved
				if st.Type != nil && c.lookupAlias(st.Type.Name) != nil {
					// The arguments of its expansion's arguments too:
					// List<TK<Any, Any>> of OneList<Any>.
					c.refBounds(st.T, st.Type, nil, nil)
				} else {
					c.checkBounds(st.T, st.Type)
				}
				c.lookupCls = cd.Sym
			}
			// A sealed class's subclasses are in its package, and known.
			if s := st.T.C; st.T.K == types.Class && s.Sealed && !s.JavaSealed {
				switch {
				case s.External && !s.Source && !s.JavaSealed:
					c.errorf(st.Type.P, "SEALED_INHERITOR_IN_DIFFERENT_MODULE", "inheritor of sealed class or interface declared in a different module")
				case packageOf(s) != packageOf(cd.Sym):
					c.errorf(st.Type.P, "SEALED_INHERITOR_IN_DIFFERENT_PACKAGE", "inheritors of sealed classes must be in the same package")
				}
				s.Subclasses = append(s.Subclasses, cd.Sym)
			}
		}
		c.annotationSupers(cd)
	}
	c.tparams, c.lookupCls = nil, nil
	c.checkAliasBounds()
}

// annotationSupers reports a class that inherits from an annotation
// class, as kotlinc's EXTENDING_AN_ANNOTATION_CLASS_ERROR: on each
// supertype that is one, or, if one is further up, on the class.
func (c *Checker) annotationSupers(cd *syntax.ClassDecl) {
	direct := false
	for _, st := range cd.Supers {
		if st.T.K == types.Class && isAnnotationClass(st.T.C) {
			c.errorf(st.Type.P, "FINAL_SUPERTYPE", "inheriting an annotation class is prohibited")
			direct = true
		}
	}
	if direct {
		return
	}
	found := false
	cd.Sym.Supertypes(func(k *types.ClassSym) bool {
		found = k != cd.Sym && isAnnotationClass(k)
		return !found
	})
	if found {
		c.errorf(cd.P, "EXTENDING_AN_ANNOTATION_CLASS_ERROR", "inheriting an annotation class is prohibited")
	}
}

// isAnnotationClass reports whether k is an annotation class, Kotlin's or
// Java's.
func isAnnotationClass(k *types.ClassSym) bool {
	if k.KotlinAnnotation() {
		return true
	}
	return k.External && k.Retention != types.NoRetention
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
			c.errorf(cd.Supers[0].Type.P, "INCONSISTENT_TYPE_PARAMETER_VALUES", "type parameters of '%s' have inconsistent values: %s, %s", s.C.DisplayName(), prev, s)
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
// classes and those of its Java sources (javaSource), is target: class
// K : J(), where the Java source has class J extends K, is a cycle too.
func inherits(cls, target *types.ClassSym, javaSource func(*types.ClassSym) bool, seen map[*types.ClassSym]bool) bool {
	if seen[cls] || cls.External && !javaSource(cls) {
		return false
	}
	seen[cls] = true
	if cls.External {
		// A Java class has its superclass and interfaces, and Supers only
		// if it is generic.
		cls.Ensure()
		for _, s := range append([]*types.ClassSym{cls.Super}, cls.Interfaces...) {
			if s != nil && (s == target || inherits(s, target, javaSource, seen)) {
				return true
			}
		}
		return false
	}
	for _, s := range cls.Supers {
		if s.C == target || inherits(s.C, target, javaSource, seen) {
			return true
		}
	}
	// Resolving a supertype written as a nested class of another, A.N or a
	// typealias of it, looks N up among A's nested classes and those A
	// inherits: kotlinc's supertypes of cls depend on A's, so
	// class E : E.EE() is a cycle.
	if cd, ok := cls.Decl.(*syntax.ClassDecl); ok {
		for _, st := range cd.Supers {
			if st.Type == nil || st.T.K != types.Class || st.T.C == nil || !viaOuter(st.Type.Name, st.T.C, javaSource) {
				continue
			}
			for o := outerClass(st.T.C); o != nil; o = outerClass(o) {
				if o == target || inherits(o, target, javaSource, seen) {
					return true
				}
			}
		}
	}
	return false
}

// outerClass returns the class cls is nested in: its Outer, or a nested
// class of the program's Java sources that isn't inner's.
func outerClass(cls *types.ClassSym) *types.ClassSym {
	if cls.External {
		cls.Ensure()
		if cls.Outer == nil {
			return cls.SourceOuter
		}
	}
	return cls.Outer
}

// viaOuter reports whether a supertype written as name, which is the class
// cls, names it through its outer class: A.N, or a typealias. A class of
// the class path's outer class can't depend on the program's, but one of
// its Java sources' can: interface T : J.N, where J extends a subtype of
// T, is a cycle.
func viaOuter(name string, cls *types.ClassSym, javaSource func(*types.ClassSym) bool) bool {
	if cls.External && !javaSource(cls) || outerClass(cls) == nil {
		return false
	}
	simple := cls.Name[strings.LastIndexAny(cls.Name, "/$")+1:]
	return strings.Contains(name, ".") || name != simple
}

// platformOnly reports whether the members a function overrides are all
// members of a Kotlin collection type that only its Java interface
// declares, as Map.remove(key, value): as with kotlinc, the function
// needn't be marked override, and is the JVM method that overrides them.
func platformOnly(over []*types.FunSym) bool {
	for _, g := range over {
		if !g.Platform && !declaredPlatformDependent(g) {
			return false
		}
	}
	return true
}

// declaredPlatformDependent reports whether the program declares f with
// @kotlin.internal.PlatformDependent, which the standard library's
// declarations of JDK methods have and a program can use where it
// suppresses INVISIBLE_REFERENCE.
func declaredPlatformDependent(f *types.FunSym) bool {
	d, ok := f.Decl.(*syntax.FunDecl)
	if !ok {
		return false
	}
	for i := range d.Annotations {
		if n := d.Annotations[i].Name; n == "PlatformDependent" || n == "kotlin.internal.PlatformDependent" {
			return true
		}
	}
	return false
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
	// A capitalized name, as TimeZone, names a class, not the property
	// that Kotlin sees for getTimeZone(): timeZone.
	r, _ := utf8.DecodeRuneInString(name)
	return found || !unicode.IsUpper(r) && len(c.javaMethods(cls, types.GetterName(name))) > 0 ||
		name == "length" && c.isCharSequence(cls) || name == "size" && c.hasSize(cls)
}

func (c *Checker) userProp(cls *types.ClassSym, name string) *types.VarSym {
	return c.userProp1(cls, name, false)
}

// memberProp is userProp for a property of an instance of cls: its
// supertypes' private properties aren't cls's.
func (c *Checker) memberProp(cls *types.ClassSym, name string) *types.VarSym {
	return c.userProp1(cls, name, true)
}

func (c *Checker) userProp1(cls *types.ClassSym, name string, inherited bool) *types.VarSym {
	// The first the code can see, or else the first: a superclass's
	// private property gives way to an interface's.
	var p, hidden *types.VarSym
	cls.Supertypes(func(s *types.ClassSym) bool {
		var q *types.VarSym
		if !s.External {
			q = s.Prop(name)
		} else if f := s.JavaFields[name]; f != nil {
			q = f // a Java class's field
		}
		switch {
		case q == nil:
		case inherited && propNotInherited(q, cls):
		case p != nil:
			// A val and a var of unrelated supertypes: the intersection
			// override is a var, as C's x of A's abstract val x and
			// B's var x.
			if q.Mutable && q.Owner != nil && p.Owner != nil && !p.Owner.IsSubclassOf(q.Owner) && !q.Owner.IsSubclassOf(p.Owner) && c.propVisible(q) {
				p = q
				return false
			}
		case c.propVisible(q):
			p = q
			if p.Mutable || p.Owner == cls || p.Owner == nil || p.Owner.External {
				return false
			}
		case hidden == nil:
			hidden = q
		}
		return true
	})
	if p == nil {
		p = hidden
	}
	return c.propAlt(p)
}

// accessible drops the members of class on (nil for a super call) that
// the code can't see: private ones outside their class, protected ones
// outside its subclasses (see visibility.go), and the private members of
// on's supertypes, which on doesn't inherit, unless they are members of
// also, the class of a smart-cast variable's declared type.
func (c *Checker) accessible(on *types.ClassSym, fs []*types.FunSym, also ...*types.ClassSym) []*types.FunSym {
	keep := func(f *types.FunSym) bool {
		if !c.funVisible(f) {
			return false
		}
		if !notInherited(f, on) {
			return true
		}
		for _, k := range also {
			if k != nil && !notInherited(f, k) {
				return true
			}
		}
		return false
	}
	for i, f := range fs {
		if !keep(f) {
			out := append([]*types.FunSym(nil), fs[:i]...)
			for _, g := range fs[i+1:] {
				if keep(g) {
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
	cls := c.dedupe(methodCallees(ms, func(f *types.FunSym) types.Subst { return c.memberSubst(rt, f.Owner) }))
	for i, cl := range cls {
		if g, ok := c.unerased(rt, cl); ok {
			cls[i] = callee{f: g, tparams: g.TypeParams, recv: c.memberSubst(rt, g.Owner)}
		}
		if cl := &cls[i]; len(cl.tparams) > 0 && cl.f.Owner != nil && javaClass(cl.f.Owner) && rawSubst(cl.recv) && !rt.IsParam() {
			// A raw type's generic method is erased too, as in Kotlin:
			// a raw StubElement's <E> E bar(E) is bar(Any!): Any!.
			s := maps.Clone(cl.recv)
			for _, p := range cl.tparams {
				s[p] = types.Arg{T: erasure(p), Raw: true}
			}
			cl.recv, cl.tparams = s, nil
		}
	}
	return c.intersectionOverride(rt, cls)
}

// intersectionOverride replaces each of cls with the member of an
// unrelated supertype of rt with the same parameters and the most
// specific return type, if it is more specific: as Kotlin's intersection
// override, which returns that type, a.baz() of an A : J, K with Java's
// CharSequence baz() and K's String baz() is K's, a String, and not
// deprecated if only J's is. The lookup of the methods by name keeps only
// the first of a signature that it meets.
func (c *Checker) intersectionOverride(rt types.Type, cls []callee) []callee {
	if rt.K != types.Class {
		return cls
	}
	for i, cl := range cls {
		f := cl.f
		if f.Owner == nil || f.Static || f.Ext || len(f.TypeParams) > 0 {
			continue
		}
		best, bestRet := cl, cl.recv.Apply(f.Ret)
		rt.C.Supertypes(func(s *types.ClassSym) bool {
			if s == f.Owner || f.Owner.IsSubclassOf(s) || s.IsSubclassOf(f.Owner) {
				return true
			}
			for _, g := range methodsNamed(s, f) {
				if g.Static || g.Private || g.Ext || len(g.TypeParams) > 0 || !c.funVisible(g) || !c.sameParams(rt, f, g) {
					continue
				}
				gs := c.memberSubst(rt, g.Owner)
				if rawSubst(gs) {
					continue
				}
				if gt := gs.Apply(g.Ret); c.subtype(gt, bestRet) && !c.subtype(bestRet, gt) {
					best, bestRet = callee{f: g, recv: gs}, gt
				}
			}
			return true
		})
		cls[i] = best
	}
	return cls
}

// unerased returns the method that cl's, a Java method of a raw
// supertype of rt, overrides in a supertype rt has with arguments, and
// that has a more specific type: as Kotlin's intersection override, a
// class that extends raw Base and implements Params<String> has Params's
// fetch(): Map<String, String>, not Base's erased fetch(): Map.
func (c *Checker) unerased(rt types.Type, cl callee) (*types.FunSym, bool) {
	f := cl.f
	if rt.K != types.Class || f.Owner == nil || !javaClass(f.Owner) || f.Static || !rawSubst(cl.recv) {
		return nil, false
	}
	var out *types.FunSym
	rt.C.Supertypes(func(s *types.ClassSym) bool {
		if s == f.Owner || !f.Owner.IsSubclassOf(s) {
			return true
		}
		for _, g := range methodsNamed(s, f) {
			gs := c.memberSubst(rt, g.Owner)
			if g.Static || g.Private || rawSubst(gs) || !c.sameParams(rt, f, g) {
				continue
			}
			if c.subtype(gs.Apply(g.Ret), cl.recv.Apply(f.Ret)) && !c.subtype(cl.recv.Apply(f.Ret), gs.Apply(g.Ret)) {
				out = g
				return false
			}
		}
		return true
	})
	return out, out != nil
}

// rawSubst reports whether s maps a raw type's parameters to their
// erasures.
func rawSubst(s types.Subst) bool {
	for _, a := range s {
		if a.Raw {
			return true
		}
	}
	return false
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
		ft, gt := fs.Apply(p.Type), gs.Apply(g.Params[i].Type)
		if hasRaw(ft) != hasRaw(gt) {
			raw, other, rf, of := gt, ft, g, f
			if hasRaw(ft) {
				raw, other, rf, of = ft, gt, f, g
			}
			if rf.Owner != of.Owner && rf.Owner.IsSubclassOf(of.Owner) {
				// A Java method overriding by erasure: foo(List) overrides
				// foo(MutableList<Int>).
				if !c.subtype(other, rawUpper(raw)) {
					return false
				}
				continue
			}
			// A raw type's erased member, as Kotlin sees it: from its
			// arguments' erasures to star projections. A raw A's
			// foo(Map<String, List<T>>) takes a Map<Any?, Any?>, not a
			// Map<Any?, Any>.
			if !c.subtype(rawLower(raw), other) || !c.subtype(other, rawUpper(raw)) {
				return false
			}
			continue
		}
		if !c.sameType(ft, gt) && !c.sameType(ft, invArray(gt)) {
			return false
		}
	}
	// A member extension's receiver is a parameter too.
	if f.Ext && !c.sameType(fs.Apply(f.Recv), gs.Apply(g.Recv)) {
		return false
	}
	return c.sameBounds(fs, gs, f, g)
}

// isRaw reports whether t is a raw type: a generic class without type
// arguments, as Java code may use one.
func isRaw(t types.Type) bool {
	return t.K == types.Class && t.A == nil && t.C.Raw()
}

// hasRaw reports whether t is or has a raw type.
func hasRaw(t types.Type) bool { return t.Has(isRaw) }

// rawLower returns t with its raw types replaced by their lower bounds,
// as Kotlin's: the class with its parameters' erasures, a raw Map a
// MutableMap<Any?, Any?>, and a raw HashMap a HashMap<Any!, Any!>.
func rawLower(t types.Type) types.Type {
	if isRaw(t) {
		args := make([]types.Arg, len(t.C.TypeParams))
		for i, p := range t.C.TypeParams {
			b := erasure(p)
			if o := p.Bound(); !o.P && !javaClass(t.C) {
				b.P, b.N = false, o.N // a Kotlin class's, as List's
			}
			args[i] = types.Arg{T: b}
		}
		t.A = types.MakeArgs(args)
		return t
	}
	return mapTypeArgs(t, rawLower)
}

// rawUpper returns t with its raw types replaced by their upper bounds,
// as Kotlin's: the read-only class with star projections, nullable: a raw
// Map a Map<*, *>?.
func rawUpper(t types.Type) types.Type {
	if isRaw(t) {
		args := make([]types.Arg, len(t.C.TypeParams))
		for i := range args {
			args[i] = types.Arg{V: types.Star}
		}
		if t.C.ReadOnly != nil {
			t.C = t.C.ReadOnly
		}
		t.A = types.MakeArgs(args)
		return t.Nullable()
	}
	return mapTypeArgs(t, rawUpper)
}

// javaClass reports whether cls is a Java class, not one Kotlin sees as
// its own, as List.
func javaClass(cls *types.ClassSym) bool {
	return cls.External && cls.KotlinName == "" && cls.Java == nil
}

// mapTypeArgs returns t with f applied to its type arguments.
func mapTypeArgs(t types.Type, f func(types.Type) types.Type) types.Type {
	if t.K != types.Class || t.A == nil {
		return t
	}
	args := slices.Clone(t.A.List)
	for i, a := range args {
		if a.V != types.Star {
			args[i].T = f(a.T)
		}
	}
	t.A = types.MakeArgs(args)
	return t
}

// invArray returns the type of a Java method's array parameter, T[], which
// Kotlin sees as Array<(out) T>!, as Array<T>!: an override may declare
// either.
func invArray(t types.Type) types.Type {
	if !t.P || t.K != types.Class || t.C != types.ArrayClass || t.A.Get(0).V != types.Out {
		return t
	}
	return types.GenericType(t.C, []types.Arg{{T: t.A.Get(0).T}}).Platform()
}

// sameBounds reports whether the type parameters of methods f and g,
// whose types sameParams maps by fs and gs, have the same upper bounds,
// in any order: functions that differ only in them are overloads, as
// with kotlinc, not overrides. Only the program's declarations and Kotlin
// classes' members of the class path are compared, whose bounds are as
// written; a Java method's are not what Kotlin sees.
func (c *Checker) sameBounds(fs, gs types.Subst, f, g *types.FunSym) bool {
	asWritten := func(f *types.FunSym) bool { return f.Decl != nil || f.Owner != nil && f.Owner.FromKotlin }
	if !asWritten(f) || !asWritten(g) {
		return true
	}
	for i, p := range f.TypeParams {
		q := g.TypeParams[i]
		if len(p.Bounds) == 0 && len(q.Bounds) == 0 {
			continue
		}
		pb, qb := boundsOf(p), boundsOf(q)
		if len(pb) != len(qb) {
			return false
		}
		for _, b := range pb {
			if !slices.ContainsFunc(qb, func(d types.Type) bool { return c.sameType(fs.Apply(b), gs.Apply(d)) }) {
				return false
			}
		}
	}
	return true
}

// boundsOf returns a type parameter's upper bounds, Any? if it has none.
func boundsOf(p *types.TypeParamSym) []types.Type {
	if len(p.Bounds) == 0 {
		return []types.Type{types.AnyType.Nullable()}
	}
	return p.Bounds
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
		if s.Name == "java/lang/Throwable" && f.Name == "fillInStackTrace" {
			// Not a member, as Kotlin sees Throwable: a function of
			// that name needs no 'override'. But K2 accepts one, as
			// kotlinx.coroutines' exceptions have, and it then
			// overrides Throwable's.
			if d, ok := f.Decl.(*syntax.FunDecl); !ok || !d.Override {
				return true
			}
		}
		gs := methodsNamed(s, f)
		if s.Name == charSequence && f.Name == "get" && !f.Ext {
			gs = s.Methods["charAt"] // CharSequence's get, on the JVM
		}
		for _, g := range gs {
			// A private member isn't inherited: it overrides and is
			// overridden by nothing.
			// Nor is a Java member that is package-private in another
			// package.
			if !g.Static && !g.Private && !f.Private && !(g.PackagePrivate && packageOf(g.Owner) != packageOf(cls)) && c.sameParams(this, f, g) {
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
	// vis and setterVis are the visibilities of its getter and setter.
	// inherits is set if the property is the program's override without
	// a visibility modifier, which has the visibility of a property it
	// overrides, and inheritsSetter if its setter has none either.
	vis, setterVis           syntax.Visibility
	inherits, inheritsSetter bool
	depr                     depr // its deprecation, or its getter's or setter's
}

// implementingProp returns the property of cls, declared or inherited,
// that implements a property of its supertype s, of a name and context
// parameters ctx, or nil.
func (c *Checker) implementingProp(cls, s *types.ClassSym, name string, ctx []*types.VarSym) *types.VarSym {
	q := c.memberProp(cls, name) // a superclass's private property is not cls's
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

// markInheritedProtected marks the properties of the file's classes that
// override a protected property without a visibility of their own, which
// are protected too, before any code that reads them is checked.
func (c *Checker) markInheritedProtected() {
	for _, cd := range c.classes {
		cls := cd.Sym
		if cls == nil {
			continue
		}
		mark := func(p *types.VarSym, override bool) {
			if p == nil || !override || explicitVisibility(p) {
				return
			}
			for _, o := range c.superProps(cls, p.Name) {
				if o.protected {
					p.InheritsProtected = true
					return
				}
			}
		}
		for _, cp := range cd.CtorParams {
			mark(cp.PropSym, cp.Override)
		}
		for _, m := range cd.Members {
			if pd, ok := m.(*syntax.PropDecl); ok && pd.Recv == nil {
				mark(pd.Sym, pd.Override)
			}
		}
	}
}

// javaFinalAccessor returns the Java superclass of cls below o's owner
// that declares a final method with the JVM signature of o's getter or
// setter, which Kotlin sees as o, final: or nil.
func javaFinalAccessor(cls *types.ClassSym, o superProp) *types.ClassSym {
	for s := cls.Super; s != nil; s = s.Super {
		if !javaClass(s) || s.FromKotlin || s == o.owner || !s.IsSubclassOf(o.owner) {
			continue
		}
		s.Ensure()
		for _, sig := range []string{o.getter, o.setter} {
			if acc, ok := s.JVMMethods[sig]; ok && sig != "" && acc&classfile.AccFinal != 0 && acc&classfile.AccStatic == 0 {
				return s
			}
		}
	}
	return nil
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
					vis, _ := propVisibility(p)
					setterVis := vis
					if pd, ok := p.Decl.(*syntax.PropDecl); ok && pd.SetterVis != syntax.Public {
						setterVis = pd.SetterVis
					}
					inherits := inheritsVisibility(p.Decl)
					out = append(out, superProp{s, p.Type, p.Final, p.Mutable, p.Abstract, types.GetterJVM(p) + "()" + d, setter, protectedProp(p), p.Contexts, vis, setterVis,
						inherits, inherits && setterVis == vis, propDeclDepr(p)})
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
				vis, setterVis := syntax.Public, syntax.Public
				if kp.Getter.Protected {
					vis, setterVis = syntax.Protected, syntax.Protected
				}
				if kp.Setter != nil && kp.Setter.Protected {
					setterVis = syntax.Protected
				}
				out = append(out, superProp{s, kp.Getter.Ret, kp.Getter.Final, kp.Setter != nil, kp.Getter.Abstract, kp.Getter.JVM() + types.FunDesc(kp.Getter), setter, kp.Getter.Protected, kp.Getter.Contexts, vis, setterVis, false, false, kpropDepr(kp)})
			}
			return true
		}
		if s.Name == charSequence && name == "length" {
			// CharSequence's length, java.lang.CharSequence.length().
			out = append(out, superProp{s, types.IntType, false, false, true, "length()I", "", false, nil, syntax.Public, syntax.Public, false, false, depr{}})
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
				vis := syntax.Public
				if g.Protected {
					vis = syntax.Protected
				}
				out = append(out, superProp{s, g.Ret, g.Final, false, g.Abstract, g.JVM() + types.FunDesc(g), "", g.Protected, nil, vis, vis, false, false, deprecationLevel(g.Deprecated)})
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
	// A class that mixes suspend and non-suspend function types is
	// reported once, at its supertypes (checkMixedSuspendSupertypes), not
	// again for its invoke overriding one of each kind.
	mixed := mixesSuspendSupertypes(d)
	if d.Value && !d.Interface && !d.Object && !d.Local {
		c.reservedValueMembers(d)
	}
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
			if o := c.interfaceSuperFun(d, f.Name, len(f.Params)); o != nil {
				if o.Final {
					c.errorf(fd.P, "OVERRIDING_FINAL_MEMBER", "'%s' in '%s' is final and cannot be overridden", f.Name, o.Owner.DisplayName())
				}
			} else if !c.errorSupers[d] && (c.u.ext.class("java/lang/Object") != nil || !anyMember[f.Name]) {
				c.errorf(fd.P, "NOTHING_TO_OVERRIDE", "'%s' overrides nothing", f.Name)
			}
		case fd.Override && javaHidden(cls, over):
			c.errorf(fd.P, "NOTHING_TO_OVERRIDE", "'%s' overrides nothing", f.Name)
		case len(over) > 0 && !fd.Override && !platformOnly(over):
			c.errorf(fd.P, "VIRTUAL_MEMBER_HIDDEN", "'%s' hides member of supertype '%s' and needs 'override' modifier", f.Name, over[0].Owner.DisplayName())
		}
		if !fd.Override {
			continue
		}
		c.paramNameWarnings(fd, over)
		if !deprecated(deprecation(fd.Annotations)) && len(over) > 0 && (!slices.ContainsFunc(over, func(o *types.FunSym) bool { return !deprecated(funDepr(o)) }) ||
			c.deprecatedOverrideOfHidden(f) != "") {
			c.overrideDeprecation(fd.P)
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
		if !fd.Explicit {
			c.inferredVisibility(fd, over)
		}
		for _, o := range over {
			if o.Infix {
				f.Infix = true // an override of an infix function is one
			}
			if o.Final {
				c.errorf(fd.P, "OVERRIDING_FINAL_MEMBER", "'%s' in '%s' is final and cannot be overridden", f.Name, o.Owner.DisplayName())
				continue
			}
			if fd.Explicit && !o.PackagePrivate && !inheritsVisibility(o.Decl) {
				c.overrideVisibility(fd.P, f.Name, memberVisibility(o), fd.Visibility, o.Owner)
			}
			switch {
			case mixed && f.Suspend != o.Suspend && o.Owner.Func:
				continue
			case f.Suspend && !o.Suspend:
				c.errorf(fd.P, "NON_SUSPEND_OVERRIDDEN_BY_SUSPEND", "suspend function '%s' cannot override non-suspend function '%s' defined in '%s'", f.Name, f.Name, o.Owner.DisplayName())
				continue
			case !f.Suspend && o.Suspend:
				c.errorf(fd.P, "SUSPEND_OVERRIDDEN_BY_NON_SUSPEND", "non-suspend function '%s' cannot override suspend function '%s' defined in '%s'", f.Name, f.Name, o.Owner.DisplayName())
				continue
			case f.Composable != o.Composable && !c.u.compose:
				c.unsupported(fd.P, "compose", "'%s' overrides a @Composable function: compile with -compose", f.Name)
				continue
			case f.Composable != o.Composable:
				c.errorf(fd.P, "CONFLICTING_OVERLOADS", "@Composable annotation mismatch with overridden function: '%s' defined in '%s'", f.Name, o.Owner.DisplayName())
				continue
			case f.Composable && (f.Changed != o.Changed || f.Defaults != o.Defaults):
				c.unsupported(fd.P, "compose", "'%s' overrides a composable function whose parameters differ", f.Name)
				continue
			}
			ret := c.overrideSubst(this, f, o).Apply(o.Ret)
			if f.Ret.K != types.Invalid && ret.K != types.Invalid && !c.overrideRetFits(f.Ret, ret) {
				c.errorf(fd.P, "RETURN_TYPE_MISMATCH_ON_OVERRIDE", "return type of '%s' is not a subtype of the return type of the overridden member '%s' defined in '%s'", f.Name, f.Name, o.Owner.DisplayName())
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
		c.javaFieldWarnings(cls, p, op.pos)
		if p.Private && !op.override {
			continue // only a field: it hides and overrides nothing
		}
		over := c.overriddenProps(cls, p)
		switch {
		case len(over) == 0 && op.override && !c.errorSupers[d]:
			c.errorf(op.pos, "NOTHING_TO_OVERRIDE", "'%s' overrides nothing", p.Name)
		case len(over) > 0 && !op.override:
			c.errorf(op.pos, "VIRTUAL_MEMBER_HIDDEN", "'%s' hides member of supertype '%s' and needs 'override' modifier", p.Name, over[0].owner.DisplayName())
		}
		if !op.override {
			continue
		}
		if !deprecated(propDeclDepr(p)) && len(over) > 0 && !slices.ContainsFunc(over, func(o superProp) bool { return !deprecated(o.depr) }) {
			c.overrideDeprecation(op.pos)
		}
		own := types.GetterJVM(p) + "()" + descOf(types.GetterType(p))
		var ownSetter string
		if p.Mutable && !p.SetterPrivate {
			ownSetter = types.SetterJVM(p) + "(" + descOf(types.FieldType(p)) + ")V"
		}
		finalReported := false
		for _, o := range over {
			t := c.memberSubst(this, o.owner).Apply(o.t)
			switch {
			case o.final:
				c.errorf(op.pos, "OVERRIDING_FINAL_MEMBER", "'%s' in '%s' is final and cannot be overridden", p.Name, o.owner.DisplayName())
				continue
			case javaFinalAccessor(cls, o) != nil:
				if !finalReported {
					c.errorf(op.pos, "OVERRIDING_FINAL_MEMBER", "'%s' in '%s' is final and cannot be overridden", p.Name, javaFinalAccessor(cls, o).DisplayName())
					finalReported = true
				}
				continue
			case o.mutable && !p.Mutable:
				c.errorf(op.pos, "VAR_OVERRIDDEN_BY_VAL", "val-property cannot override var-property '%s'", p.Name)
			case p.Type.K == types.Invalid || t.K == types.Invalid:
			case p.Mutable && o.mutable && !c.sameType(p.Type, t):
				c.errorf(op.pos, "VAR_TYPE_MISMATCH_ON_OVERRIDE", "type of '%s' doesn't match the type of the overridden var-property", p.Name)
			case !c.subtype(p.Type, t):
				c.errorf(op.pos, "PROPERTY_TYPE_MISMATCH_ON_OVERRIDE", "type of '%s' is not a subtype of the overridden property", p.Name)
			}
			vis, explicit := declVisibility(p.Decl)
			if o.inherits {
				// Its visibility is that of a member it overrides, which
				// is among the others.
			} else if explicit && explicitVisibility(p) {
				c.overrideVisibility(op.pos, p.Name, o.vis, vis, o.owner)
			}
			if pd, ok := p.Decl.(*syntax.PropDecl); ok && o.mutable && p.Mutable && pd.SetterVis != syntax.Public && !o.inheritsSetter {
				// A setter's visibility, if it has a modifier; a val's
				// overrider may have any.
				at := op.pos
				switch {
				case pd.Setter != nil:
					at = pd.Setter.P
				case pd.Getter == nil && pd.AccessorP.Line > 0:
					at = pd.AccessorP
				}
				c.overrideVisibility(at, p.Name, o.setterVis, pd.SetterVis, o.owner)
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
	reported := map[string]bool{}
	c.checkMixedSuspendSupertypes(d)
	if !d.Annotation {
		reported = c.checkInheritedFuns(d, mixed)
		c.checkInheritedProps(d, reported)
	}
	if d.Data && !d.Object {
		c.checkDataOverrides(d)
	}
	if len(d.Delegated) > 0 || slices.ContainsFunc(d.Supers, func(st *syntax.SuperType) bool { return st.Delegate != nil }) {
		c.checkDelegatedMembers(d)
	}
	if !cls.Abstract || d.Enum {
		c.checkImplemented(d, reported)
	}
	if !cls.Interface && c.inheritsJavaFunctionN(cls) {
		c.errorf(d.P, "UNSUPPORTED_INHERITANCE_FROM_JAVA_MEMBER_REFERENCING_KOTLIN_FUNCTION", "a Kotlin class can't inherit a Java member that references 'kotlin.jvm.functions.FunctionN'")
	}
}

// inheritsJavaFunctionN reports whether class cls inherits a method of
// the program's Java sources whose descriptor has FunctionN, the class of
// function types of more than 22 parameters, which kotlinc doesn't
// support.
func (c *Checker) inheritsJavaFunctionN(cls *types.ClassSym) bool {
	found := false
	cls.Supertypes(func(s *types.ClassSym) bool {
		if s == cls || !c.u.ext.javaSource(s) {
			return true
		}
		for _, fs := range s.Methods {
			for _, f := range fs {
				if !f.Private && !f.Static && f.Owner == s && strings.Contains(jvmDesc(f), "Lkotlin/jvm/functions/FunctionN;") {
					found = true
				}
			}
		}
		return !found
	})
	return found
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
				} else if nd, t := c.javaProp(sc, name); nd != "" {
					add(syntax.InheritedBridge{Bridge: od.getter, Target: nd[:strings.IndexByte(nd, '(')], Ret: t}, nd)
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
// abstract inherits and doesn't implement, but those named in reported,
// which checkInheritedFuns reported.
func (c *Checker) checkImplemented(d *syntax.ClassDecl, reported map[string]bool) {
	cls := d.Sym
	this := cls.This()
	missing := func(name string) {
		if !reported[name] {
			reported[name] = true
			if d.Enum {
				// An enum with abstract members is abstract, and its entries
				// implement them in their bodies.
				cls.Abstract = true
				for _, e := range d.Entries {
					if e.Body == nil {
						c.errorf(e.P, "ABSTRACT_MEMBER_NOT_IMPLEMENTED_BY_ENUM_ENTRY", "enum entry '%s.%s' does not implement abstract member '%s'", cls.DisplayName(), e.Name, name)
					}
				}
				return
			}
			if cls.Anonymous {
				c.errorf(d.P, "ABSTRACT_MEMBER_NOT_IMPLEMENTED", "object is not abstract and does not implement abstract member '%s'", name)
			} else {
				c.errorf(d.P, "ABSTRACT_MEMBER_NOT_IMPLEMENTED", "class '%s' is not abstract and does not implement abstract member '%s'", cls.DisplayName(), name)
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
	// An internal abstract member of another module's superclass can't be
	// implemented here, as kotlinc's INVISIBLE_ABSTRACT_MEMBER_FROM_SUPER:
	// unless a subclass in that module implements it, the class can't be
	// instantiated.
	seen := map[string]bool{}
invisible:
	for sc := cls.Super; sc != nil; sc = sc.Super {
		sc.Ensure()
		for _, key := range slices.Sorted(maps.Keys(sc.InvisibleMembers)) {
			if seen[key] {
				continue
			}
			seen[key] = true
			name, _, _ := strings.Cut(strings.TrimPrefix(key, "prop "), "/")
			// A member of the class of that name is an error of its own,
			// CANNOT_OVERRIDE_INVISIBLE_MEMBER, instead.
			declares := slices.ContainsFunc(cls.Methods[name], func(f *types.FunSym) bool { return f.Owner == cls }) ||
				cls.Prop(name) != nil && cls.Prop(name).Owner == cls
			if sc.InvisibleMembers[key] && !d.Enum && !declares {
				what := "class '" + cls.DisplayName() + "'"
				if cls.Object {
					what = "object '" + cls.DisplayName() + "'"
				}
				c.errorf(d.P, "INVISIBLE_ABSTRACT_MEMBER_FROM_SUPER_ERROR", "%s inherits invisible abstract member '%s' of %s, which it can't implement", what, name, sc.DisplayName())
				break invisible
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
		// By name: the errors' order doesn't depend on the maps'.
		for _, name := range slices.Sorted(maps.Keys(s.Methods)) {
			fs := s.Methods[name]
			if d.Data && anyMember[name] {
				continue // a data class generates them
			}
			for _, g := range fs {
				if !g.Abstract || g.Static || s.Java != nil && g.Platform || numberJavaName(s, g) {
					continue
				}
				if !c.implemented(cls, this, g) && !(s.Java != nil && concreteInSuper(cls, g.Owner, g.JVM()+types.FunDesc(g))) && !numberImplemented(cls, s, g) && !c.javaImplements(cls, g) &&
					!(s.External && s.KotlinName == "" && s.Java == nil && kotlinAccessorImplements(cls, s, g)) {
					if s.Name == charSequence && name == "charAt" {
						name = "get"
					}
					missing(name)
				}
				if s.Interface && hasDefaults(g) && !inlineReported {
					// The intersection override of an inline function of a
					// superclass and g, whose default values it inherits:
					// kotlinc's backend fails on its calls (KT-49722).
					if h := c.implementation(cls, this, g); h != nil && h.Owner != cls && !hasDefaults(h) && isInlineFun(h) {
						c.errorf(d.P, "", "functional parameters with inherited default values are not yet supported in inline functions")
						inlineReported = true
					}
				}
			}
		}
		for _, name := range slices.Sorted(maps.Keys(s.ExtMethods)) {
			for _, g := range s.ExtMethods[name] {
				if g.Abstract && !c.implemented(cls, this, g) && !c.javaImplements(cls, g) {
					missing(name)
				}
			}
		}
		if !s.External {
			for _, p := range s.Props {
				if p.Abstract {
					if q := c.implementingProp(cls, s, p.Name, p.Contexts); (q == nil || q.Abstract) && !c.javaPropImplements(cls, this, s, p.Name, p.Type, p.Contexts, p.Mutable) {
						missing(p.Name)
					} else {
						c.inheritedPropImpl(d, cls, s, p, q)
					}
				}
			}
		}
		for _, name := range slices.Sorted(maps.Keys(s.KProps)) {
			for kp := range kpropAlts(s.KProps[name]) {
				if kp.Getter.Abstract {
					if q := c.implementingProp(cls, s, name, kp.Getter.Contexts); (q == nil || q.Abstract) && !concreteInSuper(cls, nil, kp.Getter.JVM()+types.FunDesc(kp.Getter)) &&
						!c.javaPropImplements(cls, this, s, name, kp.Getter.Ret, kp.Getter.Contexts, kp.Setter != nil) {
						missing(name)
					}
				}
			}
		}
		return true
	})
}

// kotlinAccessorImplements reports whether abstract method g of Java
// class s is implemented by the accessor of a Kotlin property that isn't
// abstract, of cls or a supertype other than s, with g's JVM name and
// number of parameters: A.y's getY() implements Java interface C's
// getY() for a class D extends A implements C, and an override of a
// Kotlin interface's val B.prop1 Java interface Foo's getProp1(T t), Foo
// extending the Kotlin one.
func kotlinAccessorImplements(cls, s *types.ClassSym, g *types.FunSym) bool {
	name, arity := g.JVM(), len(g.Params)
	if !strings.HasPrefix(name, "get") && !strings.HasPrefix(name, "is") && !strings.HasPrefix(name, "set") {
		return false
	}
	fits := func(f *types.FunSym) bool {
		return f != nil && !f.Abstract && !f.Static && f.JVM() == name && jvmArity(f) == arity
	}
	// One of cls overrides a property of a supertype: a val y of its own
	// is an accidental override of a Java interface's getY().
	inherited := func(prop string) bool {
		has := false
		cls.Supertypes(func(t *types.ClassSym) bool {
			has = t != cls && (t.Prop(prop) != nil || len(t.ExtProps[prop]) > 0 || t.KProps[prop] != nil)
			return !has
		})
		return has
	}
	found := false
	cls.Supertypes(func(t *types.ClassSym) bool {
		if t == s {
			return true
		}
		switch {
		case !t.External:
			for _, p := range t.Props {
				if p.Abstract || t == cls && !inherited(p.Name) {
					continue
				}
				if types.GetterJVM(p) == name && len(p.Contexts) == arity || p.Mutable && !p.SetterPrivate && types.SetterJVM(p) == name && len(p.Contexts)+1 == arity {
					found = true
				}
			}
			for prop, gs := range t.ExtProps {
				if t != cls || inherited(prop) {
					found = found || slices.ContainsFunc(gs, fits) || slices.ContainsFunc(gs, func(f *types.FunSym) bool { return fits(f.ExtSetter) })
				}
			}
		case t.KotlinName != "":
			for _, kp := range t.KProps {
				found = found || fits(kp.Getter) || fits(kp.Setter)
			}
		}
		return !found
	})
	return found
}

// jvmArity returns the number of parameters of f's JVM method: its
// context parameters, extension receiver and parameters.
func jvmArity(f *types.FunSym) int {
	if f.Desc != "" {
		n := 0
		for d := f.Desc[1:]; len(d) > 0 && d[0] != ')'; n++ {
			for d[0] == '[' {
				d = d[1:]
			}
			if d[0] == 'L' {
				d = d[strings.IndexByte(d, ';')+1:]
			} else {
				d = d[1:]
			}
		}
		return n
	}
	n := len(f.Contexts) + len(f.Params)
	if f.Ext {
		n++
	}
	return n
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
// has a method with code with this name and descriptor. A Java source has
// no bridge methods, so in a subclass of owner, if not nil, one of its
// methods with a narrower return type counts too: String get(Object) for
// Map's Object get(Object).
func concreteInSuper(cls, owner *types.ClassSym, nd string) bool {
	params, ret, _ := strings.Cut(nd, ")")
	params += ")"
	covariant := strings.HasPrefix(ret, "L") // a reference type
	for s := cls.Super; s != nil; s = s.Super {
		s.Ensure()
		if acc, ok := s.JVMMethods[nd]; ok && acc&classfile.AccAbstract == 0 {
			return true
		}
		if covariant && owner != nil && s.Java == nil && !s.FromKotlin && s.IsSubclassOf(owner) {
			for k, acc := range s.JVMMethods {
				if r, ok := strings.CutPrefix(k, params); ok && acc&(classfile.AccAbstract|classfile.AccStatic) == 0 && (r[0] == 'L' || r[0] == '[') {
					return true
				}
			}
		}
	}
	return false
}

// javaImplements reports whether a superclass of cls among the program's
// Java sources, which inherits the program's abstract method g, has a
// method that isn't abstract with g's JVM signature: Java's foo(String a,
// int b) implements a Kotlin interface's context(a: String) foo(b: Int).
func (c *Checker) javaImplements(cls *types.ClassSym, g *types.FunSym) bool {
	if g.Owner == nil || g.Owner.External {
		return false
	}
	sig := g.JVM() + memberDesc(g)
	for s := cls.Super; s != nil; s = s.Super {
		if !c.u.ext.javaSource(s) || !s.IsSubclassOf(g.Owner) {
			continue
		}
		for _, f := range s.Methods[g.JVM()] {
			if !f.Abstract && !f.Static && f.JVM()+jvmDesc(f) == sig {
				return true
			}
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
		if !g.Owner.Interface && g.Owner.IsSubclassOf(s) {
			return true // g, abstract, overrides its members: A's abstract toString() Any's
		}
		for _, h := range hs {
			if _, ok := h.Decl.(*syntax.FunDecl); ok && g.Owner.Name == charSequence && (g.Name == "charAt" || g.Name == "length") && h.Name == g.Name {
				continue // a Kotlin charAt() or length() doesn't implement CharSequence's get or length
			}
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
	if len(x.TypeArgs) > 0 && sc.OuterParams > 0 && !sc.External && len(x.TypeArgs) != len(sc.TypeParams)-sc.OuterParams {
		// Type arguments of a typealias are for all of an inner class's
		// type parameters, its outer class's too: inner class Test1 :
		// GI<T1>() for typealias GI<T> = Generic<T>.Inner. Written ones
		// are its own: Inner<Int>().
		for i := range cands {
			cands[i].tparams, cands[i].recv = sc.TypeParams, nil
		}
	}
	if len(cands) == 0 {
		c.errorf(x.P, "SEALED_INHERITOR_IN_DIFFERENT_MODULE", "%s has no constructor that a subclass can call", sc.DisplayName())
		c.args(x, nil)
		return
	}
	x.Ctor = sc
	if !c.superOuter(d.Sym, x) || !c.superOuterArgs(d.Sym, x) {
		return
	}
	want := noWant
	if len(x.TypeArgs) > 0 && sc.OuterParams == 0 && c.superViaAlias(d, x) {
		// A typealias's type arguments are its own, Alias<String> for
		// typealias Alias<X> = Inv<X, Inv<X, X>>: its expansion, the
		// supertype, gives the class's.
		x.TypeArgs = nil
	}
	if len(x.TypeArgs) == 0 {
		// Through a typealias, the supertype gives the type arguments:
		// class D : FooStr() for typealias FooStr = Foo<String>.
		for _, t := range d.Sym.Supers {
			if t.K == types.Class && t.C == sc && t.A.Len() > 0 {
				want = t
			}
		}
		if want.K != types.Invalid && slices.ContainsFunc(want.A.List, func(a types.Arg) bool { return a.V != types.Inv }) {
			c.errorf(x.P, "PROJECTION_IN_IMMEDIATE_ARGUMENT_TO_SUPERTYPE", "type alias with projections in its expansion can't be used as a constructor or a supertype: %s", want)
			c.args(x, nil)
			return
		}
	}
	c.resolve(x, cands, want, nil)
	if d.Sym.Object {
		c.selfCalls(d.Sym, x)
	}
	if sc.Local {
		c.localConstructed(sc) // the class passes its superclass's captures
	}
}

// superViaAlias reports whether class d's superclass call x names a
// typealias.
func (c *Checker) superViaAlias(d *syntax.ClassDecl, x *syntax.Call) bool {
	for _, st := range d.Supers {
		if st.Call == x {
			return st.Type != nil && st.Type.Name != "" && c.lookupAlias(st.Type.Name) != nil
		}
	}
	return false
}

// superOuterArgs reports whether the outer class type arguments of inner
// class cls's superclass, an inner class too, are those of the outer
// instance cls passes it: not in inner class Test2 : GIInt() of a
// Generic<T1>, for typealias GIInt = Generic<Int>.Inner.
func (c *Checker) superOuterArgs(cls *types.ClassSym, x *syntax.Call) bool {
	sc := x.Ctor
	if !sc.Inner || sc.OuterParams == 0 || !cls.Inner || cls.Outer == nil {
		return true
	}
	os, ok := c.asSuper(cls.Outer.This(), sc.Outer)
	if !ok || os.A.Len() != sc.OuterParams {
		return true
	}
	own := len(sc.TypeParams) - sc.OuterParams
	for _, t := range cls.Supers {
		if t.K != types.Class || t.C != sc || t.A.Len() != len(sc.TypeParams) {
			continue
		}
		for i := range sc.OuterParams {
			if t.A.Get(own+i) != os.A.Get(i) {
				c.errorf(x.P, "INNER_CLASS_CONSTRUCTOR_NO_RECEIVER", "constructor of inner class %s can be called only with receiver of containing class", sc.DisplayName())
				c.args(x, nil)
				return false
			}
		}
	}
	return true
}

// selfCalls reports the uses of object obj in its own superclass's
// constructor call x, which runs before the object exists: prop, this.prop
// and Companion.prop in companion object : Foo(...).
func (c *Checker) selfCalls(obj *types.ClassSym, x *syntax.Call) {
	self := func(e syntax.Expr) bool {
		switch e := e.(type) {
		case *syntax.Ident:
			return e.Object == obj
		case *syntax.Member:
			return e.Object == obj
		case *syntax.This:
			return e.Obj == obj
		}
		return false
	}
	for _, a := range x.Args {
		syntax.Inspect(a, func(n any) bool {
			e, ok := n.(syntax.Expr)
			if !ok {
				return true
			}
			switch e.(type) {
			case *syntax.Lambda, *syntax.ObjectExpr:
				return false // runs later, when the object exists
			}
			at := e
			if id, ok := e.(*syntax.Ident); ok && id.Member != nil && self(id.Member.X) {
				at = id // a member of the object in scope
			} else if !self(e) {
				return true
			}
			c.errorf(at.Pos(), "SELF_CALL_IN_NESTED_OBJECT_CONSTRUCTOR_ERROR", "self call in nested object constructor is prohibited")
			return false
		})
	}
}

// superTarget checks super or super<I> as a receiver of the member name,
// and returns the supertype whose members it refers to. Without <I>, as
// in Kotlin, it is the supertype that declares the member: the
// superclass, or an interface. super@Outer is an outer class's, whose
// instance the code reaches as this@Outer's.
func (c *Checker) superTarget(s *syntax.Super, name string, call bool) (types.Type, bool) {
	cls := c.cls
	if s.Label != "" && c.ambiguousLabel(s.Label) {
		c.errorf(s.P, "AMBIGUOUS_LABEL", "ambiguous label: %s", s.Label)
	}
	switch {
	case s.Label != "" && (cls == nil || s.Label != simpleName(cls)):
		r := c.outerRecv(s.Label)
		if r == nil {
			c.errorf(s.P, "UNRESOLVED_LABEL", "unresolved label: %s", s.Label)
			return types.InvalidType, false
		}
		s.Outer = recvThis(r, s.P)
		c.value(s.Outer, noWant)
		cls = r.sym.Type.C
	case s.Label == "" && cls == nil && c.headerClassRecv() != nil:
		// In an object expression's supertypes, the code around it's
		// super, as object : A by super.f() in a class's method.
		r := c.headerClassRecv()
		s.Outer = recvThis(r, s.P)
		c.value(s.Outer, noWant)
		cls = r.sym.Type.C
	case cls == nil || cls.Decl == nil:
		c.errorf(s.P, "INSTANCE_ACCESS_BEFORE_SUPER_CALL", "'super' is not an expression, it can only be used on the left-hand side of a dot ('.')")
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
		args := false // a typealias that gives the type arguments
		if ref := s.Iface; ref.Args == nil && ref.Ret == nil && ref.Name != "Any" {
			if a := c.lookupAlias(ref.Name); a != nil && len(a.TypeParams) == 0 {
				c.resolveAlias(a)
				if at := a.Type; at.K == types.Class && at.A.Len() > 0 {
					want, args = at, true
				}
			}
			if wc := c.lookupClass(ref.Name); wc != nil && !args {
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
			// Type arguments, written or a typealias's, must be the
			// supertype's: super<G<String>> of G<Int> is NOT_A_SUPERTYPE.
			if st.C == want.C && (!args && s.Iface.Args == nil || c.sameArgs(st, want)) || want.K == types.Any {
				t = st
			}
		}
		if t.K == types.Invalid {
			c.errorf(s.Iface.P, "NOT_A_SUPERTYPE", "not an immediate supertype")
			return t, false
		}
		if s.Outer != nil && s.Iface.Args != nil && c.sameArgs(t, want) {
			// kotlinc reports it of an outer class's super only.
			c.warnf(s.Iface.P, "TYPE_ARGUMENTS_REDUNDANT_IN_SUPER_QUALIFIER", "type arguments do not need to be specified in a 'super' qualifier")
		}
	default:
		var has []types.Type
		for _, st := range cls.Supers {
			if c.superHas(st.C, name, call) {
				has = append(has, st)
			}
		}
		// A supertype that another one extends: the other's members
		// override its own.
		has = slices.DeleteFunc(slices.Clone(has), func(st types.Type) bool {
			return slices.ContainsFunc(has, func(o types.Type) bool { return o.C != st.C && o.C.IsSubclassOf(st.C) })
		})
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
		if i := slices.IndexFunc(cls.Supers, func(st types.Type) bool { return st.C != nil && st.C.Name == "java/lang/Cloneable" }); i >= 0 && len(has) == 1 && name == "clone" && !has[0].C.IsSubclassOf(cls.Supers[i].C) {
			// kotlin.Cloneable declares clone(), which java.lang.Cloneable
			// doesn't: a superclass's is one of two.
			has = append(has, cls.Supers[i])
		}
		if len(has) > 1 {
			c.errorf(s.P, "AMBIGUOUS_SUPER", "many supertypes available, please specify the one you mean in angle brackets, e.g. 'super<%s>'", has[0].C.DisplayName())
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
				c.errorf(s.P, "SUPER_NOT_AVAILABLE", "'super' is not available here")
				return t, false
			}
			t = types.ClassType(cls.Super)
		}
	}
	s.Cls, s.T = t.C, t
	return t, true
}

// headerClassRecv returns, in the supertypes of an object expression or
// a local class, the receiver that is the enclosing class's this, or nil.
func (c *Checker) headerClassRecv() *implicitRecv {
	if c.cls != nil || c.staticCls == nil || c.anon[c.staticCls] == nil {
		return nil
	}
	for i := len(c.recvs) - 1; i >= 0; i-- {
		if r := &c.recvs[i]; r.class && r.outer && r.sym.Type.K == types.Class {
			return r
		}
	}
	return nil
}

// simpleName returns the name of class cls without its outer classes'.
func simpleName(cls *types.ClassSym) string {
	n := cls.DisplayName()
	return n[strings.LastIndexByte(n, '.')+1:]
}

// superHas reports whether the supertype cls has a property or, for a
// call, a function named name.
func (c *Checker) superHas(cls *types.ClassSym, name string, call bool) bool {
	if call {
		for _, f := range c.accessible(nil, c.javaMethods(cls, name)) {
			// An interface's equals, hashCode and toString are Any's.
			if !cls.Interface || f.Owner == nil || f.Owner.Name != "java/lang/Object" {
				return true
			}
		}
		// A Kotlin property of the name counts too, even one that can't
		// be invoked: super.fn() is ambiguous between a fun fn() and a
		// val fn of another supertype. A Java getter doesn't.
		return c.userProp(cls, name) != nil
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
		// An implementation doesn't count where a subtype of its class
		// redeclares it abstract, as IWithToString does Any's toString,
		// nor an interface's where a class declares it abstract: the
		// intersection of them is abstract.
		var impls, abstracts []*types.FunSym
		cls.Supertypes(func(s *types.ClassSym) bool {
			for _, f := range c.accessible(nil, s.Methods[name]) {
				if f.Abstract {
					abstracts = append(abstracts, f)
				} else {
					impls = append(impls, f)
				}
			}
			return true
		})
		for _, f := range impls {
			if !slices.ContainsFunc(abstracts, func(a *types.FunSym) bool {
				return a.Owner != nil && f.Owner != nil && a.Owner != f.Owner && paramKey(a) == paramKey(f) &&
					(f.Owner.Name == "java/lang/Object" || a.Owner.IsSubclassOf(f.Owner) || f.Owner.Interface && !a.Owner.Interface)
			}) {
				return true
			}
		}
		p := c.userProp(cls, name)
		return p != nil && !p.Abstract
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
	if n, _ := c.innerNamed(st.C, fn.Name); n != nil && n.Inner {
		// super.Inner() constructs the supertype's inner class with this
		// as its outer instance.
		this := &syntax.This{Label: s.Label}
		this.P = s.P
		fn.X = this
		return c.call(x, want, sink)
	}
	ms := c.accessible(nil, c.javaMethods(st.C, fn.Name))
	if len(ms) == 0 && st.C.Interface {
		if object := c.u.ext.class("java/lang/Object"); object != nil {
			ms = c.javaMethods(object, fn.Name)
		}
	}
	if len(ms) == 0 && c.hasFunProp(st, fn.Name, x) {
		// A property of a function type, invoked: super.x(y) for a
		// supertype's val x: (String) -> Boolean is super.x.invoke(y),
		// which reads the supertype's property.
		inv := &syntax.Member{X: fn, Name: "invoke"}
		inv.P = fn.P
		x.Fun = inv
		return c.call(x, want, sink)
	}
	if len(ms) == 0 {
		c.errorf(fn.P, "UNRESOLVED_REFERENCE", "unresolved reference: %s", fn.Name)
		c.args(x, nil)
		return types.InvalidType
	}
	t := c.resolve(x, c.memberCallees(c.superImpls(st.C, ms), st), want, sink)
	if x.Sym != nil && x.Sym.Abstract {
		c.errorf(fn.P, "ABSTRACT_SUPER_CALL", "abstract member cannot be accessed directly")
	} else if x.Sym != nil && usesDefaults(x) {
		// The super call would need the supertype's foo$default, which
		// dispatches virtually.
		c.errorf(fn.P, "SUPER_CALL_WITH_DEFAULT_PARAMETERS", "super calls with default arguments are not allowed, specify all arguments of 'super.%s' explicitly", fn.Name)
	}
	return t
}

// usesDefaults reports whether a resolved call leaves a parameter of its
// callee, other than a vararg, to its default value.
func usesDefaults(x *syntax.Call) bool {
	ps := x.Sym.Params
	passed := make([]bool, len(ps))
	for i := range x.Args {
		j := i
		if x.ArgMap != nil {
			j = x.ArgMap[i]
		}
		if j >= 0 && j < len(ps) {
			passed[j] = true
		}
	}
	for j, p := range ps {
		if !passed[j] && !p.Vararg {
			return true
		}
	}
	return false
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
					// An interface's doesn't implement a class's.
					if !f.Abstract && paramKey(f) == paramKey(m) && (m.Owner == nil || f.Owner == nil || m.Owner.Interface || !f.Owner.Interface) {
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
			c.errorf(x.P, "ABSTRACT_SUPER_CALL", "abstract member cannot be accessed directly")
		}
		ensureVar(p)
		x.Prop = p
		return c.memberSubst(st, p.Owner).Apply(p.Type)
	}
	if t, ok := c.javaProperty(x, st); ok {
		return t
	}
	c.errorf(x.P, "INVISIBLE_REFERENCE", "unresolved reference: %s", x.Name)
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
	return f != nil && (!f.Protected || c.inSubclass(f.Owner) || c.javaPackage(f.Owner)) && (!f.PackagePrivate || c.samePackage(f.Owner))
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

// numberJavaName reports whether g is one of java.lang.Number's methods
// under its Java name, intValue(): Kotlin sees it as Number's toInt(),
// which a subclass implements instead, and which numberConversions adds.
func numberJavaName(s *types.ClassSym, g *types.FunSym) bool {
	if s.Name != "java/lang/Number" || len(g.Params) != 0 {
		return false
	}
	for _, conv := range numberConversions {
		if g.Name == conv[1] {
			return true
		}
	}
	return false
}

// javaProp returns the method of class s of the class path, with code,
// that Kotlin sees as the property name of its Kotlin type: size() of a
// collection or a map, name() and ordinal() of an Enum. It returns its JVM
// name and descriptor and the property's type, or "".
func (c *Checker) javaProp(s *types.ClassSym, name string) (string, types.Type) {
	if !s.External || s.Interface {
		return "", types.InvalidType
	}
	var nd string
	var t types.Type
	switch {
	case name == "size" && c.hasSize(s):
		nd, t = "size()I", types.IntType
	case name == "ordinal" && c.isSubclassOf(s, "java/lang/Enum"):
		nd, t = "ordinal()I", types.IntType
	case name == "name" && c.isSubclassOf(s, "java/lang/Enum"):
		nd, t = "name()Ljava/lang/String;", types.StringType
	default:
		return "", types.InvalidType
	}
	s.Ensure()
	if acc, ok := s.JVMMethods[nd]; !ok || acc&classfile.AccAbstract != 0 {
		return "", types.InvalidType
	}
	return nd, t
}

// javaPropImplements reports whether a superclass of cls on the class
// path implements property name of supertype s, of type t as s declares
// it, a var if mutable, with methods Kotlin sees as the property: an
// enum's name implements an interface's val name: String (see
// javaProp), and a Java class's getX() and setX(x) implement a var x.
func (c *Checker) javaPropImplements(cls *types.ClassSym, this types.Type, s *types.ClassSym, name string, t types.Type, ctx []*types.VarSym, mutable bool) bool {
	if len(ctx) > 0 {
		return c.javaContextAccessors(cls, s, name, ctx, mutable)
	}
	want := c.memberSubst(this, s).Apply(t)
	for sc := cls.Super; sc != nil; sc = sc.Super {
		if !sc.External {
			continue
		}
		if s.Interface && !mutable {
			if _, pt := c.javaProp(sc, name); pt.K != types.Invalid {
				return c.subtype(pt, want)
			}
		}
		if sc.KotlinName == "" && sc.Java == nil && c.javaAccessorsImplement(this, sc, name, want, mutable) {
			return true
		}
	}
	return false
}

// javaContextAccessors reports whether a superclass of cls among the
// program's Java sources, which inherits s, declares methods with code
// that implement s's property name with context parameters ctx, a var if
// mutable: getX and setX that take the context parameters first, as
// their JVM descriptors have them.
func (c *Checker) javaContextAccessors(cls, s *types.ClassSym, name string, ctx []*types.VarSym, mutable bool) bool {
	cd, ok := types.ParamsDesc(ctx)
	if !ok {
		return false
	}
	cd = cd[:len(cd)-1]
	for sc := cls.Super; sc != nil; sc = sc.Super {
		if !c.u.ext.javaSource(sc) || !sc.IsSubclassOf(s) {
			continue
		}
		has := func(acc string, params int) bool {
			return slices.ContainsFunc(sc.Methods[acc], func(h *types.FunSym) bool {
				return h.Owner == sc && !h.Static && !h.Abstract && len(h.Params) == params && strings.HasPrefix(jvmDesc(h), cd)
			})
		}
		if has(types.GetterName(name), len(ctx)) && (!mutable || has(types.SetterName(name), len(ctx)+1)) {
			return true
		}
	}
	return false
}

// javaAccessorsImplement reports whether Java class sc, a superclass of
// this, declares methods with code that implement property name of type
// want, a var if mutable: a getter getX() or isX() whose result is a
// subtype of want, and a setter setX(x) that takes one.
func (c *Checker) javaAccessorsImplement(this types.Type, sc *types.ClassSym, name string, want types.Type, mutable bool) bool {
	sc.Ensure()
	sub := c.memberSubst(this, sc)
	has := func(acc string, params int, fits func(*types.FunSym) bool) bool {
		return slices.ContainsFunc(sc.Methods[acc], func(h *types.FunSym) bool {
			return h.Owner == sc && !h.Static && !h.Abstract && !h.Ext && len(h.TypeParams) == 0 && len(h.Params) == params && fits(h)
		})
	}
	if !has(types.GetterName(name), 0, func(h *types.FunSym) bool { return c.subtype(sub.Apply(h.Ret), want) }) {
		return false
	}
	return !mutable || has(types.SetterName(name), 1, func(h *types.FunSym) bool { return c.subtype(want, sub.Apply(h.Params[0].Type)) })
}

// numberImplemented reports whether g, Kotlin's Number.toInt() or one of
// the others, is implemented by its Java method, intValue(), for a class
// cls that extends a subclass of Number on the class path, such as
// AtomicInteger, which declares it.
func numberImplemented(cls, s *types.ClassSym, g *types.FunSym) bool {
	if s.Name != "java/lang/Number" || g.JVMName == "" {
		return false
	}
	// As with kotlinc, Number's own byteValue() doesn't implement toByte():
	// AtomicInteger's subclass must.
	nd := g.JVMName + types.FunDesc(g)
	for sc := cls.Super; sc != nil && sc != s; sc = sc.Super {
		if !sc.External {
			continue
		}
		sc.Ensure()
		if acc, ok := sc.JVMMethods[nd]; ok {
			return acc&classfile.AccAbstract == 0
		}
	}
	return false
}

// overrideRetFits reports whether an override's return type t is a
// subtype of the overridden member's u. As in K2, a builder's variable
// in t isn't constrained by it: a stub returned by an anonymous object's
// member fits, and one in an argument doesn't (see stubInference).
func (c *Checker) overrideRetFits(t, u types.Type) bool {
	if !c.hasStub(t) {
		return c.subtype(t, u)
	}
	if c.stub(t.Flex()) != nil {
		return true
	}
	saved := c.stubs
	c.stubs = nil
	defer func() { c.stubs = saved }()
	return c.subtype(t, u)
}

// methodsOfAny reports the functions of interface d with a body that are
// equals, hashCode or toString of Any: a class always inherits Any's,
// which wins over an interface's.
func (c *Checker) methodsOfAny(d *syntax.ClassDecl) {
	for _, m := range d.Members {
		f, ok := m.(*syntax.FunDecl)
		if !ok || f.Sym == nil || f.Body == nil && f.ExprBody == nil || f.Recv != nil || len(f.Contexts) > 0 {
			continue
		}
		any := false
		switch f.Name {
		case "hashCode", "toString":
			any = len(f.Sym.Params) == 0
		case "equals":
			any = len(f.Sym.Params) == 1 && f.Sym.Params[0].Type.K == types.Any && f.Sym.Params[0].Type.N
		}
		if any && !suppresses(f.Annotations, "METHOD_OF_ANY_IMPLEMENTED_IN_INTERFACE") {
			c.errorf(f.P, "METHOD_OF_ANY_IMPLEMENTED_IN_INTERFACE", "an interface may not implement a method of 'Any'")
		}
	}
}

// overrideVisibility reports an override, of a name, whose visibility v
// is weaker than, or not comparable with, the visibility ov of a member of
// owner that it overrides.
func (c *Checker) overrideVisibility(p syntax.Pos, name string, ov, v syntax.Visibility, owner *types.ClassSym) {
	switch cmp, ok := compareVisibility(ov, v); {
	case !ok:
		c.errorf(p, "CANNOT_CHANGE_ACCESS_PRIVILEGE", "cannot change access privilege '%s' for '%s' in '%s'", visibilityName(ov), name, owner.DisplayName())
	case cmp > 0:
		c.errorf(p, "CANNOT_WEAKEN_ACCESS_PRIVILEGE", "cannot weaken access privilege '%s' for '%s' in '%s'", visibilityName(ov), name, owner.DisplayName())
	}
}

// inferredVisibility reports override fd, without a visibility, of
// members over none of which is the wider of the others, as an internal
// and a protected one: kotlinc then gives it the protected one, which
// the internal one's can't change to.
func (c *Checker) inferredVisibility(fd *syntax.FunDecl, over []*types.FunSym) {
	var internal *types.FunSym
	protected := false
	for _, o := range over {
		if o.PackagePrivate || inheritsVisibility(o.Decl) {
			continue
		}
		switch memberVisibility(o) {
		case syntax.Public:
			return
		case syntax.Internal:
			internal = o
		case syntax.Protected:
			protected = true
		}
	}
	if internal != nil && protected {
		c.overrideVisibility(fd.P, fd.Name, syntax.Protected, syntax.Internal, internal.Owner)
	}
}

// compareVisibility compares visibilities a and b: positive if a is the
// wider, and false if neither is, as protected and internal.
func compareVisibility(a, b syntax.Visibility) (int, bool) {
	rank := func(v syntax.Visibility) int {
		switch v {
		case syntax.Private:
			return 0
		case syntax.Public:
			return 2
		}
		return 1
	}
	if a == b {
		return 0, true
	}
	ra, rb := rank(a), rank(b)
	if ra == rb {
		return 0, false // protected and internal
	}
	return ra - rb, true
}

// visibilityName returns the keyword of visibility v.
func visibilityName(v syntax.Visibility) string {
	switch v {
	case syntax.Private:
		return "private"
	case syntax.Protected:
		return "protected"
	case syntax.Internal:
		return "internal"
	}
	return "public"
}

// inheritsVisibility reports whether decl is an override of the program
// without a visibility modifier, which has the visibility of the members
// it overrides.
func inheritsVisibility(decl any) bool {
	switch d := decl.(type) {
	case *syntax.FunDecl:
		if d.AccessorOf != nil {
			return inheritsVisibility(d.AccessorOf.Decl)
		}
		return d.Override && !d.Explicit
	case *syntax.PropDecl:
		return d.Override && !d.Explicit
	case *syntax.CtorParam:
		return d.Override && !d.Explicit
	}
	return false
}

// superProjections reports projected type arguments of supertype t,
// written st without a constructor call (superCall checks those with
// one): a class can't implement Inv<*>, nor through a typealias whose
// expansion projects them.
func (c *Checker) superProjections(st *syntax.SuperType, t types.Type) {
	if t.A == nil || !slices.ContainsFunc(t.A.List, func(a types.Arg) bool { return a.V != types.Inv }) {
		return
	}
	for _, a := range st.Type.Args {
		if a.Variance != types.Inv {
			c.errorf(a.P, "PROJECTION_IN_IMMEDIATE_ARGUMENT_TO_SUPERTYPE", "projections are not allowed for immediate arguments of a supertype")
			return
		}
	}
	if c.lookupAlias(st.Type.Name) != nil {
		c.errorf(st.Type.P, "CONSTRUCTOR_OR_SUPERTYPE_ON_TYPEALIAS_WITH_TYPE_PROJECTION_ERROR", "type alias with projections in its expansion can't be used as a constructor or a supertype: %s", t)
	}
}

// checkMixedSuspendSupertypes reports a class that has both a suspend and
// a non-suspend function type among its supertypes, directly or through
// other supertypes (MIXING_SUSPEND_AND_NON_SUSPEND_SUPERTYPES), at its
// supertype list, as kotlinc does. It reports whether it did.
func (c *Checker) checkMixedSuspendSupertypes(d *syntax.ClassDecl) bool {
	if mixesSuspendSupertypes(d) {
		c.errorf(d.Supers[0].Type.P, "MIXING_SUSPEND_AND_NON_SUSPEND_SUPERTYPES", "mixing suspend and non-suspend supertypes is not allowed")
		return true
	}
	return false
}

// mixesSuspendSupertypes reports whether class d has both a suspend and
// a non-suspend function type among its supertypes.
func mixesSuspendSupertypes(d *syntax.ClassDecl) bool {
	if len(d.Supers) == 0 || d.Sym == nil {
		return false
	}
	var suspend, plain bool
	d.Sym.Supertypes(func(s *types.ClassSym) bool {
		if s != d.Sym && s.Func {
			if s.Suspend {
				suspend = true
			} else {
				plain = true
			}
		}
		return !(suspend && plain)
	})
	return suspend && plain
}
