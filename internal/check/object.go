package check

import (
	"slices"
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// This file resolves nested classes, and objects and companion objects
// used as values.

// scopeClass returns the class whose nested classes' names are in scope.
// In the signatures of an object expression's class or a local class,
// which are resolved in the code around it, it is that class, then the
// classes it is in: object { inner class C : B(); open inner class B }.
func (c *Checker) scopeClass() *types.ClassSym {
	if k := c.lookupCls; k != nil && c.cls != nil && (k.Anonymous || k.Local || k.Inner) && k != c.supersOf {
		return k // not in its own supertypes, which don't see its nested classes
	}
	if c.cls != nil {
		return c.cls
	}
	return c.lookupCls
}

// nestedClass finds a nested class by its simple name in the classes the
// code is in, innermost first.
func (c *Checker) nestedClass(name string) *types.ClassSym {
	k := c.nestedClassifier(name)
	if k.alias != nil {
		return c.aliasClass(k.alias)
	}
	return k.cls
}

// nestedClassifier finds a class or a typealias nested in the enclosing
// classes by its simple name.
func (c *Checker) nestedClassifier(name string) classifier {
	for k := c.scopeClass(); k != nil; k = c.nextEnclosing(k) {
		if k == c.headerBounds {
			continue
		}
		var found classifier
		// A class sees its superclasses' nested classes too, but not its
		// interfaces': a Widget.Mode doesn't hide an imported Mode in the
		// classes that implement Widget.
		for s := k; s != nil && found == (classifier{}); s = s.Super {
			s.Ensure()
			if s.External {
				// A class path class's, as java.util.AbstractMap's
				// SimpleEntry, or clikt's AbstractHelpFormatter's
				// RenderedSection.
				if n := c.nestedOf(s, name); n != nil && (!privateClass(n) || n.PackagePrivate && c.samePackage(n)) {
					// A private one isn't inherited: AbstractList's
					// SubList in a subclass's. A package-private one is,
					// in its package: B_S of class X : D() for a Java
					// class B { static class B_S {} } that D extends.
					found = classifier{cls: n}
				}
			} else if n := s.Nested[name]; n != nil {
				found = classifier{cls: n}
			} else if a := s.Aliases[name]; a != nil {
				found = classifier{alias: a}
			}
		}
		// And its companion's: class X { companion object { class Y };
		// val y: Y }, but not in the companion's supertypes.
		if found == (classifier{}) && k.Companion != nil && !k.Companion.External && k.Companion != c.supersOf {
			if n := k.Companion.Nested[name]; n != nil {
				found = classifier{cls: n}
			}
		}
		if found != (classifier{}) {
			return found
		}
	}
	return classifier{}
}

// privateClass loads a class of the class path and reports whether it is
// private.
func privateClass(cls *types.ClassSym) bool {
	cls.Ensure()
	return cls.Private
}

// dottedClass resolves a qualified name of one of the program's classes:
// a.b.C, Outer.Nested or a.b.Outer.Nested. It returns nil for other names.
func (c *Checker) dottedClass(parts []string) *types.ClassSym {
	for i := len(parts) - 1; i >= 1; i-- {
		if cls := c.userClass(strings.Join(parts[:i], "."), parts[i]); cls != nil {
			return nestedPath(cls, parts[i+1:])
		}
	}
	if aliasQualifier(c.findClassifier(parts[0]), parts) {
		return nil
	}
	if cls := c.findClass(parts[0]); cls != nil && !cls.External {
		return nestedPath(cls, parts[1:])
	}
	if c.findClassifier(parts[0]) == (classifier{}) && !c.u.isPackage(parts[0]) {
		if cls := c.rootQualifier(parts[0]); cls != nil {
			// Klass.Nested of the root package's Klass (see rootQualifier).
			for _, n := range parts[1:] {
				if cls = c.nestedOf(cls, n); cls == nil {
					break
				}
			}
			return cls
		}
	}
	return nil
}

// aliasQualifier reports whether parts, a dotted name whose first part
// is k, names a class nested in one through a typealias, KT.Nested for
// typealias KT = KTest, which kotlinc doesn't resolve: only the
// companion's members are found through one.
func aliasQualifier(k classifier, parts []string) bool {
	return k.alias != nil && len(parts) > 1 && parts[1] != "Companion"
}

// userClassPath resolves the fully qualified name of one of the program's
// classes, a.b.C or a.b.Outer.Nested, or C or Outer.Nested in the
// default package, or returns nil.
func (c *Checker) userClassPath(parts []string) *types.ClassSym {
	for i := len(parts) - 1; i >= 1; i-- {
		if cls := c.userClass(strings.Join(parts[:i], "."), parts[i]); cls != nil {
			return nestedPath(cls, parts[i+1:])
		}
	}
	if p := c.u.Pkgs[""]; p != nil {
		if cls := p.Classes[parts[0]]; cls != nil {
			return nestedPath(cls, parts[1:])
		}
	}
	return nil
}

func nestedPath(cls *types.ClassSym, names []string) *types.ClassSym {
	for _, n := range names {
		if cls = cls.Nested[n]; cls == nil {
			return nil
		}
	}
	return cls
}

// builtinCompanion returns the companion object of a built-in type, such
// as Int's, when the standard library is on the class path.
func (c *Checker) builtinCompanion(name string) *types.ClassSym {
	jc, ok := companionJVM["kotlin/"+name+".Companion"]
	if !ok || !c.u.ext.kotlin {
		return nil
	}
	return c.u.ext.class(jc)
}

// objectOf returns the object that a class name denotes as a value: the
// object itself, or the class's companion object, or nil.
func objectOf(cls *types.ClassSym) *types.ClassSym {
	cls.Ensure()
	if cls.Object {
		return cls
	}
	return cls.Companion
}

// objectValue marks x, which names an object or a class with a companion
// object, as that object used as a value, and returns its type.
func objectValue(x syntax.Expr, obj *types.ClassSym) types.Type {
	t := types.ClassType(obj)
	switch x := x.(type) {
	case *syntax.Ident:
		x.Object = obj
	case *syntax.Member:
		x.Object = obj
	}
	x.SetType(t)
	return t
}

// objectIdent returns an expression for an object used as a value.
func objectIdent(obj *types.ClassSym, p syntax.Pos) *syntax.Ident {
	id := &syntax.Ident{Name: obj.DisplayName()}
	id.P = p
	objectValue(id, obj)
	return id
}

// scopeObjects lists the objects whose members are in scope without a
// receiver: the companions of the classes the code is in, and the
// objects it is nested in, innermost first.
func (c *Checker) scopeObjects() []*types.ClassSym {
	var out []*types.ClassSym
	for _, k := range c.enclosingClasses() {
		if k != c.cls && k.Object {
			out = append(out, k)
		}
		if k.Companion != nil && k.Companion != c.cls {
			out = append(out, k.Companion)
		}
		// The companions of its superclasses, as Kotlin's static scope
		// has them.
		for s := k.Super; s != nil; s = s.Super {
			if s.Ensure(); s.Companion != nil && s.Companion != c.cls {
				out = append(out, s.Companion)
			}
		}
	}
	return out
}

// objectHasProp reports whether an object in scope may have a property
// named name: a property of the program's, or a getter or field of a
// class path's, or an extension property that fits it.
func (c *Checker) objectHasProp(name string) bool {
	for _, obj := range c.scopeObjects() {
		if !obj.External {
			if c.userProp(obj, name) != nil {
				return true
			}
		} else {
			obj.Ensure()
			if obj.KProps[name] != nil || obj.Fields[name] != nil {
				return true
			}
		}
		if len(c.javaMethods(obj, types.GetterName(name))) > 0 {
			return true
		}
		if _, ok := c.extProp(&syntax.Member{Name: name}, obj.This()); ok {
			return true
		}
	}
	return false
}

// objectMember checks a name as a property of an object in scope, such as
// a companion's: in class A { companion object { val n = 1 } }, n inside
// A is Companion.n. It reports false if no such object has one.
func (c *Checker) objectMember(x *syntax.Ident) (types.Type, bool) {
	for _, obj := range c.scopeObjects() {
		m := &syntax.Member{X: objectIdent(obj, x.P), Name: x.Name}
		m.P = x.P
		n := len(c.errs)
		t := c.expr(m, noWant)
		if len(c.errs) > n {
			c.errs = c.errs[:n]
			continue
		}
		x.Member = m
		return t, true
	}
	return types.InvalidType, false
}

// objectTaking returns the object in scope with a method named name that
// may take x's arguments, or nil.
func (c *Checker) objectTaking(name string, x *syntax.Call) *types.ClassSym {
	for _, obj := range c.scopeObjects() {
		if c.anyMayTake(c.javaMethods(obj, name), x) {
			return obj
		}
	}
	return nil
}

// anyMayTake reports whether one of fs may take x's arguments.
func (c *Checker) anyMayTake(fs []*types.FunSym, x *syntax.Call) bool {
	for _, f := range fs {
		if c.mayTake(f, x) {
			return true
		}
	}
	return false
}

// objectMethods returns the object in scope with methods named name that
// may take x's arguments, or else the first with methods named name, or
// nil.
func (c *Checker) objectMethods(name string, x *syntax.Call) *types.ClassSym {
	var first *types.ClassSym
	for _, obj := range c.scopeObjects() {
		ms := c.javaMethods(obj, name)
		if len(ms) == 0 {
			continue
		}
		for _, f := range ms {
			if c.mayTake(f, x) {
				return obj
			}
		}
		if first == nil {
			first = obj
		}
	}
	return first
}

// objectExtension returns the innermost object in scope with an
// extension function named name that may take x's arguments, or nil.
// An object whose own methods may take them comes first.
func (c *Checker) objectExtension(name string, x *syntax.Call) *types.ClassSym {
	for _, obj := range c.scopeObjects() {
		if c.anyMayTake(c.javaMethods(obj, name), x) {
			return nil
		}
		var fs []*types.FunSym
		for _, cl := range c.extCalleesFor(name, obj.This(), x) {
			if c.mayTake(cl.f, x) {
				fs = append(fs, cl.f)
			}
		}
		// As for this's extensions, a top-level function of their
		// package comes first: run { } is kotlin's run(block), not
		// Companion.run { }.
		if len(fs) > 0 && !c.sameScopeTopFun(name, x, fs) {
			return obj
		}
	}
	return nil
}

// enclosingClasses returns the class the code is in, or whose static
// scope it is in, and the classes around it, the innermost first: past an
// object expression or local class declared in a static scope, such as
// an enum entry's arguments, that scope's class.
func (c *Checker) enclosingClasses() []*types.ClassSym {
	var out []*types.ClassSym
	for k := c.firstEnclosing(); k != nil; k = c.nextEnclosing(k) {
		out = append(out, k)
	}
	return out
}

// firstEnclosing and nextEnclosing walk the classes the code is in, as
// enclosingClasses returns them, without allocating.
func (c *Checker) firstEnclosing() *types.ClassSym {
	if c.cls == nil {
		return c.staticCls
	}
	return c.cls
}

func (c *Checker) nextEnclosing(k *types.ClassSym) *types.ClassSym {
	if k.Outer == nil && c.anon[k] != nil {
		return c.anon[k].staticCls
	}
	return k.Outer
}

// enumEntryInScope finds an entry of an enum class the code is in, which
// its members can use by its simple name, as can its classes' calls of
// their superclasses' constructors.
func (c *Checker) enumEntryInScope(name string) *types.VarSym {
	for _, k := range c.enclosingClasses() {
		if k.Entries != nil && !k.External {
			if e := k.Fields[name]; e != nil {
				return e
			}
		}
	}
	return nil
}

// enumInScope returns the innermost enum class the code is in, whose
// entries its members and its companion's can use unqualified.
func (c *Checker) enumInScope() *types.ClassSym {
	for _, k := range c.enclosingClasses() {
		if k.Entries != nil && !k.External {
			return k
		}
	}
	return nil
}

// entriesEnum returns the enum class whose entries the code's entries is:
// that of the innermost enum the code is in, unless it or a class in
// between has a member entries.
func (c *Checker) entriesEnum() *types.ClassSym {
	for k := c.firstEnclosing(); k != nil; k = c.nextEnclosing(k) {
		if c.hasCallable(k, "entries") {
			return nil
		}
		if k.Entries != nil && !k.External {
			return k
		}
	}
	return nil
}

// enumEntries checks E.entries, the list of an enum's entries, which needs
// the standard library: a static getter returning EnumEntries<E>.
func (c *Checker) enumEntries(x *syntax.Member, cls *types.ClassSym) types.Type {
	ee := c.u.ext.pkgClass("kotlin.enums", "EnumEntries")
	if ee == nil {
		c.errorf(x.P, "unresolved reference: entries")
		return types.InvalidType
	}
	t := types.GenericType(ee, []types.Arg{{T: types.ClassType(cls)}})
	x.Static = cls
	x.Getter = &types.FunSym{Name: "getEntries", Owner: cls, Static: true, Desc: "()Lkotlin/enums/EnumEntries;", Ret: t, Resolved: true}
	if cls.Ensure(); cls.External && len(cls.Statics["getEntries"]) == 0 {
		// A Java enum's: enumEntries(E.values()).
		arr := types.GenericType(types.ArrayClass, []types.Arg{{T: types.ClassType(cls)}})
		x.Getter = &types.FunSym{Name: "values", Owner: cls, Static: true, Desc: "()[L" + cls.Name + ";", Ret: arr, Resolved: true}
		x.Entries = true
	}
	return t
}

// hasStatic reports whether a class has a static field, such as an enum
// entry, or a static property named name, which wins over its companion
// object's members.
func (c *Checker) hasStatic(cls *types.ClassSym, name string) bool {
	if cls == nil {
		return false
	}
	cls.Ensure()
	return c.staticFieldOf(cls, name) != nil || name == "entries" && cls.Entries != nil
}

// staticFieldOf returns a class's static field named name, or nil. A
// companion object's const and @JvmField properties of the class path
// are static fields of the class it is the companion of, where
// Outer.Companion.X finds them.
func (c *Checker) staticFieldOf(cls *types.ClassSym, name string) *types.VarSym {
	if f := c.visibleJavaField(cls, name); f != nil || !cls.External || !cls.Object {
		return f
	}
	i := strings.LastIndexByte(cls.Name, '$')
	if i < 0 {
		return nil
	}
	if outer := c.u.ext.class(cls.Name[:i]); outer != nil {
		if outer.Ensure(); outer.Companion == cls {
			return c.visibleJavaField(outer, name)
		}
	}
	return nil
}

// visibleJavaField returns the static field of a class named name, or
// nil if there is none the code sees: a package-private one is its
// package's. Of two that the class inherits, it returns the first, and
// staticField reports the ambiguity.
func (c *Checker) visibleJavaField(cls *types.ClassSym, name string) *types.VarSym {
	if fs := c.visibleStaticFields(cls, name); len(fs) > 0 {
		return fs[0]
	}
	return nil
}

// visibleStaticFields returns the static fields named name in the static
// scope of cls that the code sees.
func (c *Checker) visibleStaticFields(cls *types.ClassSym, name string) []*types.VarSym {
	var out []*types.VarSym
	for _, f := range staticFields(cls, name) {
		if c.visibleField(f) {
			out = append(out, f)
		}
	}
	return out
}

// staticFields returns the distinct static fields named name in the
// static scope of cls, as K2's JavaClassStaticUseSiteScope has it. A
// Kotlin class has its own only: KotlinSub.JAVA_FIELD is unresolved. A
// Java class has its own, or, if it declares none of the name, those of
// its supertypes, through Kotlin classes, of which two distinct ones are
// ambiguous: E.field, for a class E implementing two interfaces with a
// field. One field reached along two paths is one.
func staticFields(cls *types.ClassSym, name string) []*types.VarSym {
	cls.Ensure()
	if f := cls.Fields[name]; f != nil {
		return []*types.VarSym{f}
	}
	if !isJavaClass(cls) {
		return nil
	}
	var out []*types.VarSym
	seen := map[*types.ClassSym]bool{}
	var walk func(s *types.ClassSym)
	walk = func(s *types.ClassSym) {
		if seen[s] {
			return
		}
		seen[s] = true
		if s.Ensure(); isJavaClass(s) {
			for _, f := range staticFields(s, name) {
				if !slices.Contains(out, f) {
					out = append(out, f)
				}
			}
			return
		}
		// A Kotlin class has no statics of its supertypes, but a Java
		// class below it sees those of the Java classes above it.
		if s.Super != nil {
			walk(s.Super)
		}
		for _, t := range s.Interfaces {
			walk(t)
		}
	}
	if cls.Super != nil {
		walk(cls.Super)
	}
	for _, t := range cls.Interfaces {
		walk(t)
	}
	return out
}

// isJavaClass reports whether cls is a Java class: one of the class path
// that Kotlin didn't compile, or of the program's Java sources.
func isJavaClass(cls *types.ClassSym) bool {
	return cls.External && !cls.FromKotlin && cls.KotlinName == "" && cls.Java == nil
}

// inheritedStatic checks a name as a static field in the scope of the
// class the code is in, or an outer one: RESULT_OK in an Activity's code.
// As in K2, that scope is the static scope of the class and of its
// superclasses, so a field of a Java interface the class implements
// directly isn't in it, but one a Java superclass inherits is. The first
// Java superclass's static scope has all those above it.
func (c *Checker) inheritedStatic(x *syntax.Ident) (types.Type, bool) {
	for k := c.cls; k != nil; k = k.Outer {
		for s := k; s != nil; s = s.Super {
			java := isJavaClass(s)
			if fs := c.visibleStaticFields(s, x.Name); len(fs) > 0 {
				id := &syntax.Ident{Name: s.DisplayName()}
				id.P = x.P
				m := &syntax.Member{X: id, Name: x.Name}
				m.P = x.P
				x.Member = m
				t := c.staticField(m, s)
				m.SetType(t)
				return t, true
			}
			if java {
				break
			}
		}
	}
	return types.InvalidType, false
}

// staticInScope returns the class the code is in, or an outer one, with
// static methods named name, or the Java superclass of one that has them:
// Thread's currentThread() in a subclass's code. As in Kotlin, a class's
// companion comes before its superclasses' statics: it returns nil if one
// on the way has methods named name, which objectMethods finds.
func (c *Checker) staticInScope(name string) *types.ClassSym {
	companionHas := func(k *types.ClassSym) bool {
		return !k.External && k.Companion != nil && len(c.javaMethods(k.Companion, name)) > 0
	}
	for k := c.cls; k != nil; k = k.Outer {
		if !k.External && len(k.Statics[name]) > 0 {
			return k
		}
		if companionHas(k) {
			return nil
		}
		// The first Java superclass, through which a public static of a
		// package-private class above it is accessible.
		var java *types.ClassSym
		for s := k.Super; s != nil; s = s.Super {
			if s.Ensure(); java == nil && isJavaClass(s) {
				java = s
			}
			if s.External && s.KotlinName == "" && len(s.Statics[name]) > 0 {
				if java != nil {
					return java
				}
				return s
			}
			if companionHas(s) {
				return nil
			}
		}
	}
	return nil
}

// ownStaticTakes reports whether this class's Java superclasses have a
// static method name that may take call x.
func (c *Checker) ownStaticTakes(name string, x *syntax.Call) bool {
	for s := c.cls; s != nil; s = s.Super {
		if s.Ensure(); s.External && s.KotlinName == "" && c.anyMayTake(s.Statics[name], x) {
			return true
		}
	}
	return false
}

// outerObjectFuns returns the functions named name, not hidden, of the
// first object in scope but this class that has any.
func (c *Checker) outerObjectFuns(name string) []*types.FunSym {
	for _, obj := range c.scopeObjects() {
		if obj == c.cls {
			continue
		}
		var fs []*types.FunSym
		for _, f := range c.accessible(obj, c.javaMethods(obj, name)) {
			if !f.Hidden {
				fs = append(fs, f)
			}
		}
		if len(fs) > 0 {
			return fs
		}
	}
	return nil
}

// localGivesWay reports whether local functions fs, named name, give way
// to the top-level functions of the name in call x: as in Kotlin, where
// each level of scope is tried in turn, fun foobar(x: String) in a block
// doesn't take foobar(MyEnum3.X), which the top level's foobar(x: MyEnum3)
// does.
func (c *Checker) localGivesWay(fs []*types.FunSym, name string, x *syntax.Call) bool {
	tops := c.topFuns(name)
	if len(tops) == 0 || !c.anyMayTake(tops, x) || hasLambda(x.Args) {
		return false
	}
	ts := c.argTypes(x)
	return !c.takeTypes(fs, x, ts) && c.takeTypes(tops, x, ts)
}
