package check

import (
	"slices"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// classRules checks the rules about a class's declarations that need its
// members' signatures, and nothing else of the class: those kotlinc's
// declaration checkers report.
func (c *Checker) classRules(d *syntax.ClassDecl) {
	if d.Object && d.Data {
		c.dataObjectEquality(d)
	}
	if d.Sealed && !d.Interface {
		c.sealedConstructors(d)
	}
	if d.Sealed && d.Fun {
		c.errorf(modPos(d.Written, "sealed", d.P), "'sealed' modifier is not allowed on fun interfaces")
	}
	if d.Fun {
		c.functionDelegateClash(d)
	}
	if st := recordSuper(d); st != nil {
		c.errorf(st.Type.P, "classes cannot have explicit 'java.lang.Record' supertype")
	} else if a := c.findAnnotation(d.Annotations, "", "JvmRecord", "kotlin/jvm/JvmRecord"); a != nil {
		c.jvmRecord(d, a)
	}
	if thr := c.u.ext.class("java/lang/Throwable"); thr != nil && d.Sym != nil && d.Sym.IsSubclassOf(thr) {
		// catch couldn't tell the instances apart by their type
		// arguments, which the JVM erases.
		switch {
		case len(d.TypeParams) > 0:
			c.errorf(d.TypeParams[0].P, "subclass of 'Throwable' cannot have generic parameters")
		case d.Sym.Inner && d.Sym.OuterParams > 0:
			c.errorf(d.P, "inner class of generic class extending 'Throwable' is prohibited")
		}
	}
}

// recordSuper returns class d's first supertype if it is java.lang.Record,
// which kotlinc's FirJvmRecordChecker reports, as a typealias of it too,
// whether d is annotated @JvmRecord or not: kotlinc gives a record that
// supertype.
func recordSuper(d *syntax.ClassDecl) *syntax.SuperType {
	if len(d.Supers) == 0 {
		return nil
	}
	if st := d.Supers[0]; st.Type != nil && st.T.K == types.Class && st.T.C != nil && st.T.C.Name == "java/lang/Record" {
		return st
	}
	return nil
}

// jvmRecordClass reports whether class d, whose annotations are resolved,
// is compiled as a record: a data class annotated @JvmRecord, for a JVM
// target that has records. Those jvmRecord rejects are compiled as
// records as well, for the errors about their members.
func (c *Checker) jvmRecordClass(d *syntax.ClassDecl) bool {
	return d.Data && !d.Object && c.u.jvmTarget >= 60 &&
		slices.ContainsFunc(d.Sym.Annotations, func(a *types.ClassSym) bool { return a.Name == "kotlin/jvm/JvmRecord" })
}

// isRecord reports whether class cls is a record: one that extends
// java.lang.Record.
func isRecord(cls *types.ClassSym) bool {
	return cls != nil && cls.Super != nil && cls.Super.Name == "java/lang/Record"
}

// jvmRecord checks class d, annotated @JvmRecord with a, as kotlinc's
// FirJvmRecordChecker does: a record is a final data class, neither local
// nor inner, whose constructor's parameters, at least one, all vals and
// only the last a vararg, are its only fields, for a JVM target of 16 or
// later. As kotlinc, it reports the first of these errors, and then the
// others about the class's members.
func (c *Checker) jvmRecord(d *syntax.ClassDecl, a *syntax.Annotation) {
	switch {
	case c.u.jvmTarget < 60:
		// At the start of the declaration, its annotations', as
		// kotlinc.
		p := a.P
		earlier := func(q syntax.Pos) {
			if q.Before(p) {
				p = q
			}
		}
		earlier(d.P)
		for _, a := range d.Annotations {
			if !a.Implicit {
				earlier(a.P)
			}
		}
		for _, m := range d.Written {
			earlier(m.P)
		}
		c.errorf(p, "using @JvmRecord is only allowed with -jvm-target 16 or later (or -jvm-target 15 with the -Xjvm-enable-preview flag enabled)")
		return
	case d.Local:
		c.errorf(a.P, "local '@JvmRecord' classes are prohibited")
		return
	case d.Inner:
		c.errorf(modPos(d.Written, "inner", d.P), "'@JvmRecord' class cannot be inner")
		return
	case d.Open || d.Abstract || d.Sealed || d.Interface:
		p := d.P
		for _, m := range d.Written {
			if m.Name == "open" || m.Name == "abstract" || m.Name == "sealed" {
				p = m.P
				break
			}
		}
		c.errorf(p, "'@JvmRecord' class must be final")
		return
	case d.Enum:
		c.errorf(modPos(d.Written, "enum", d.P), "'@JvmRecord' class cannot be an enum")
		return
	case !d.Data:
		c.errorf(a.P, "only data classes are allowed to be marked as '@JvmRecord'")
		return
	}
	if d.HasPrimary || d.Object {
		if len(d.CtorParams) == 0 {
			c.errorf(a.P, "primary constructor with parameters is required for '@JvmRecord' class")
			return
		}
		for _, cp := range d.CtorParams[:len(d.CtorParams)-1] {
			if cp.Param.Vararg {
				c.errorf(cp.P, "only the last constructor parameter of '@JvmRecord' can be a vararg")
			}
		}
	}
	for _, cp := range d.CtorParams {
		if cp.IsProp && cp.Mutable {
			c.errorf(cp.P, "constructor parameter of '@JvmRecord' class must be a 'val'")
		}
	}
	for _, m := range d.Members {
		if pd, ok := m.(*syntax.PropDecl); ok && pd.DelegateOf == nil && !pd.Generated && pd.Sym != nil &&
			(pd.Field != nil || pd.Sym.HasField()) {
			c.errorf(pd.P, "non-constructor properties with backing field in '@JvmRecord' class are prohibited")
		}
	}
	for _, st := range d.Supers {
		if st.Delegate != nil {
			c.errorf(st.Type.P, "delegation is prohibited for '@JvmRecord' classes")
		}
	}
	if len(d.Supers) > 0 {
		// Any, too, written as a supertype: not the one a class has
		// without.
		st := d.Supers[0]
		if st.Type != nil && (st.Call != nil || st.T.K == types.Any || st.T.K == types.Class && st.T.C != nil && !st.T.C.Interface && !st.T.C.Object) {
			c.errorf(d.P, "record cannot extend a class")
		}
	}
}

// dataObjectEquality reports a data object's overrides of equals and
// hashCode, which it generates, as kotlinc's
// DATA_OBJECT_CUSTOM_EQUALS_OR_HASH_CODE.
func (c *Checker) dataObjectEquality(d *syntax.ClassDecl) {
	for _, m := range d.Members {
		fd, ok := m.(*syntax.FunDecl)
		if !ok || !fd.Override || fd.Recv != nil || fd.Sym == nil {
			continue
		}
		ps := fd.Sym.Params
		equals := fd.Name == "equals" && len(ps) == 1 && ps[0].Type.K == types.Any && ps[0].Type.N
		hashCode := fd.Name == "hashCode" && len(ps) == 0
		if equals || hashCode {
			c.errorf(modPos(fd.Written, "override", fd.P), "data objects cannot have custom implementations of 'equals' or 'hashCode'")
		}
	}
}

// sealedConstructors reports a sealed class's constructors declared
// public or internal, as kotlinc's NON_PRIVATE_OR_PROTECTED_CONSTRUCTOR_IN_SEALED:
// only its subclasses call them.
func (c *Checker) sealedConstructors(d *syntax.ClassDecl) {
	open := func(m syntax.Modifiers) bool {
		return m.Explicit && (m.Visibility == syntax.Public || m.Visibility == syntax.Internal)
	}
	const msg = "constructors of sealed classes can only be protected or private"
	if open(d.CtorMods) {
		c.errorf(modPos(d.CtorMods.Written, "", d.P), msg)
	}
	for _, sc := range d.Secondaries() {
		if open(sc.Modifiers) {
			c.errorf(modPos(sc.Written, "", sc.P), msg)
		}
	}
}

// modPos returns the position of the modifier name among written, of the
// first visibility modifier if name is "", or def if there is none.
func modPos(written []syntax.Mod, name string, def syntax.Pos) syntax.Pos {
	for _, m := range written {
		switch {
		case m.Name == name, name == "" && (m.Name == "public" || m.Name == "internal" || m.Name == "protected" || m.Name == "private"):
			return m.P
		}
	}
	return def
}

// superSafeCall reports super?.m, as kotlinc's UNEXPECTED_SAFE_CALL: super
// is never null.
func (c *Checker) superSafeCall(x *syntax.Member) {
	if x.Safe {
		c.errorf(x.P, "the safe call operator is not allowed on super")
	}
}

// publishedApi reports @PublishedApi on a declaration that isn't
// internal, as kotlinc's NON_INTERNAL_PUBLISHED_API: it makes an internal
// declaration usable from public inline functions, and others already
// are, or can't be. An override without a visibility modifier has the
// visibility of the members it overrides, often internal, and isn't
// reported.
func (c *Checker) publishedApi(m *syntax.Modifiers) {
	if m.Override && !m.Explicit {
		return
	}
	if a := m.Find("", "PublishedApi"); a != nil && m.Visibility != syntax.Internal {
		c.errorf(a.P, "@PublishedApi annotation is only applicable for internal declaration")
	}
}

// functionDelegateClash reports a fun interface's property functionDelegate
// or function getFunctionDelegate(), as kotlinc's
// FUNCTION_DELEGATE_MEMBER_NAME_CLASH: the JVM classes of its SAM
// conversions declare getFunctionDelegate().
func (c *Checker) functionDelegateClash(d *syntax.ClassDecl) {
	const msg = "fun interface member '%s' clashes with the function delegate of its SAM conversions"
	for _, m := range d.Members {
		switch m := m.(type) {
		case *syntax.PropDecl:
			if m.Name == "functionDelegate" && m.Recv == nil {
				c.errorf(m.P, msg, m.Name)
			}
		case *syntax.FunDecl:
			if m.Name == "getFunctionDelegate" && m.Recv == nil && len(m.Params) == 0 {
				c.errorf(m.P, msg, m.Name)
			}
		}
	}
}
