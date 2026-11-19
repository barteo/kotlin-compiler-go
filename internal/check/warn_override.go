package check

import (
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// This file reports kotlinc's OVERRIDE_DEPRECATION: an override that
// isn't deprecated itself of members that all are, at any level, which
// its callers then use without a warning. checkInheritance judges each
// override by the members it overrides, of every supertype: an explicit
// override that isn't deprecated is among them, so that overriding it
// needs no annotation, as with kotlinc.

// overrideDeprecation reports the override declared at p.
func (c *Checker) overrideDeprecation(p syntax.Pos) {
	c.warnf(p, "OVERRIDE_DEPRECATION", "This declaration overrides a deprecated member but is not marked as deprecated itself. Add the '@Deprecated' annotation or suppress the diagnostic.")
}

func deprecated(d depr) bool { return d.level != "" }

// funDepr returns the deprecation of function f, the program's or of the
// class path.
func funDepr(f *types.FunSym) depr {
	d := depr{}
	switch decl := f.Decl.(type) {
	case *syntax.FunDecl:
		d = deprecation(decl.Annotations)
	case nil:
		d = deprecationLevel(f.Deprecated)
	}
	if f.Hidden {
		d = d.or(depr{level: "HIDDEN"})
	}
	return d
}

// propDeclDepr returns the deprecation of property p of the program, or
// of its getter or setter.
func propDeclDepr(p *types.VarSym) depr {
	switch d := p.Decl.(type) {
	case *syntax.PropDecl:
		return deprecation(d.Annotations, "property", "get", "set")
	case *syntax.CtorParam:
		return deprecation(d.Modifiers.Annotations, "property", "get", "set")
	case nil:
		return deprecationLevel(p.Deprecated)
	}
	return depr{}
}

// kpropDepr returns the deprecation of property kp of the class path, its
// getter's or its setter's.
func kpropDepr(kp *types.KProp) depr {
	d := funDepr(kp.Getter)
	if kp.Setter != nil {
		d = d.or(funDepr(kp.Setter))
	}
	return d
}
