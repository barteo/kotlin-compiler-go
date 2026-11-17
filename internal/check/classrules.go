package check

import (
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
// are, or can't be.
func (c *Checker) publishedApi(m *syntax.Modifiers) {
	if a := m.Find("", "PublishedApi"); a != nil && m.Visibility != syntax.Internal {
		c.errorf(a.P, "@PublishedApi annotation is only applicable for internal declaration")
	}
}
