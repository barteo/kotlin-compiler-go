package check

import (
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// Imports of members.
//
// import a.b.C.x imports a member of the class or object C: an enum entry
// or a Java static, which C.x names, or a property or function of an
// object, a companion too. resolveImports records it in memberImports,
// and where the name x is used, it is rewritten to C.x, as objectMember
// rewrites the members of a companion in scope. An explicit import comes
// after the members of the classes the code is in and of implicit
// receivers, and before the program's top-level declarations.
//
// import a.b.C.* imports C's enum entries, or a Java class's statics, and
// its nested classes. They come after the declarations of the file's
// package, as a package's star import does.

type memberImport struct {
	owner *types.ClassSym
	name  string // the member's own name: the import may rename it
	p     syntax.Pos
	path  string
	star  bool // by import C.*
}

// checkMemberImports reports the member imports that name nothing. They
// are checked once the members of the program's classes are declared.
func (c *Checker) checkMemberImports() {
	for _, mi := range c.memberImports {
		if !c.usable(mi.owner, mi.p) {
			continue
		}
		switch {
		case c.importedStatic(mi) || mi.owner.Object && c.objectHas(mi.owner, mi.name):
		case c.objectHas(mi.owner, mi.name):
			c.errorf(mi.p, "cannot import '%s'. Functions and properties can only be imported from packages or objects", mi.name)
		default:
			c.errorf(mi.p, "unresolved import: %s", mi.path)
		}
	}
}

// importedStatic reports whether an imported member is a static field,
// such as an enum entry, or static methods of its class.
func (c *Checker) importedStatic(mi memberImport) bool {
	return c.staticFieldOf(mi.owner, mi.name) != nil || len(javaStatics(mi.owner, mi.name)) > 0
}

// objectHas reports whether a class has a member named name, which its
// instances have: a property, a method, or an extension declared in it.
func (c *Checker) objectHas(cls *types.ClassSym, name string) bool {
	if c.userProp(cls, name) != nil || len(c.javaMethods(cls, name)) > 0 ||
		len(memberExtProps(cls, name)) > 0 || len(memberExtMethods(cls, name)) > 0 {
		return true
	}
	found := false
	cls.Supertypes(func(s *types.ClassSym) bool {
		found = s.KProps[name] != nil
		return !found
	})
	if !found && cls.External {
		m := &syntax.Member{Name: name}
		_, found = c.javaProperty(m, cls.This())
	}
	return found
}

// importedCallable reports whether an imported member can be called by
// its name: a static method, or a method of an object.
func (c *Checker) importedCallable(mi memberImport) bool {
	if len(javaStatics(mi.owner, mi.name)) > 0 {
		return true
	}
	return mi.owner.Object && len(c.javaMethods(mi.owner, mi.name)) > 0
}

// callableImport returns the member import of a name that can be called,
// if there is one.
func (c *Checker) callableImport(name string) (memberImport, bool) {
	mi, ok := c.memberImports[name]
	if !ok || !c.importedCallable(mi) ||
		mi.star && (len(c.userFuns[name]) > 0 || len(c.funImports[name]) > 0 || len(nonExt(c.pkg.Funs[name])) > 0) {
		return memberImport{}, false
	}
	return mi, true
}

// valueImport reports whether a name is imported as a property, an enum
// entry or a static field, rather than as a function.
func (c *Checker) valueImport(name string) bool {
	mi, ok := c.memberImports[name]
	if !ok || c.importedCallable(mi) {
		return false
	}
	if mi.star {
		// The file's package's declarations come first: another file's
		// private property isn't one of them.
		_, user := c.userVars[name]
		_, file := c.fileVars[name]
		_, pkg := c.pkg.Vars[name]
		return !user && !file && !pkg
	}
	return true
}

// importedRef returns the expression C.x that an imported member's name
// stands for, unchecked.
func (c *Checker) importedRef(mi memberImport, p syntax.Pos) *syntax.Member {
	var x syntax.Expr
	if mi.owner.Object && c.staticFieldOf(mi.owner, mi.name) == nil {
		x = objectIdent(mi.owner, p)
	} else {
		id := &syntax.Ident{Name: c.fieldOwner(mi.owner, mi.name).DisplayName()}
		id.P = p
		x = id
	}
	m := &syntax.Member{X: x, Name: mi.name}
	m.P = p
	return m
}

// fieldOwner returns the class whose static field a member of cls named
// name is: a library companion's constants are fields of its outer class,
// as import Context.Companion.TERMINAL_KEY's.
func (c *Checker) fieldOwner(cls *types.ClassSym, name string) *types.ClassSym {
	if cls.External && cls.Object && javaField(cls, name) == nil {
		if i := strings.LastIndexByte(cls.Name, '$'); i >= 0 {
			if outer := c.u.ext.class(cls.Name[:i]); outer != nil {
				if outer.Ensure(); outer.Companion == cls && javaField(outer, name) != nil {
					return outer
				}
			}
		}
	}
	return cls
}

// importedMember checks a name as an imported property, enum entry or
// static field. It reports false if the name isn't one.
func (c *Checker) importedMember(x *syntax.Ident) (types.Type, bool) {
	if !c.valueImport(x.Name) {
		return types.InvalidType, false
	}
	mi := c.memberImports[x.Name]
	m := c.importedRef(mi, x.P)
	x.Member = m
	return c.checkImportedRef(m, mi), true
}

// checkImportedRef checks the expression importedRef returns. A static is
// found in its class directly: the class's name may not be in scope.
func (c *Checker) checkImportedRef(m *syntax.Member, mi memberImport) types.Type {
	var t types.Type
	if id, ok := m.X.(*syntax.Ident); ok && id.Object == nil {
		t = c.staticField(m, c.fieldOwner(mi.owner, mi.name))
	} else {
		t = c.member(m)
	}
	m.SetType(t)
	return t
}

// importTarget returns the target of an assignment: a name that is
// nothing else but an imported property is that property.
func (c *Checker) importTarget(x syntax.Expr) syntax.Expr {
	id, ok := x.(*syntax.Ident)
	if !ok || !c.valueImport(id.Name) {
		return x
	}
	if _, _, _, ok := c.lookupVar(id.Name); ok {
		return x
	}
	return c.importedRef(c.memberImports[id.Name], id.P)
}
