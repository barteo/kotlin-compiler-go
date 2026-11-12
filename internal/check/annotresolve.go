package check

import (
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// An annotation's name must resolve to an annotation class, as kotlinc's
// UNRESOLVED_REFERENCE, NOT_A_CLASS and NOT_AN_ANNOTATION_CLASS: a
// declaration's annotations are resolved in the scope of the class it is
// in, a class's own in the scope around it, and those of local
// declarations and expressions in the current scope.

// annotationClassOf returns the annotation class of annotation a of a
// declaration in class owner, written on site, or nil, reporting why, if
// its name isn't one.
func (c *Checker) annotationClassOf(a *syntax.Annotation, owner *types.ClassSym, site annotSite, local bool) *types.ClassSym {
	if a.Implicit {
		cls := c.lookupClass(a.Name)
		if cls != nil {
			cls.Ensure()
		}
		return cls
	}
	saved := c.lookupCls
	if !local && c.cls == nil && c.scope == nil {
		c.lookupCls = annotationScope(owner, c.lookupCls, site)
	}
	if c.u.scopeRec != nil && (a.Name == ProbeName || strings.HasSuffix(a.Name, "."+ProbeName)) {
		c.probeType(&syntax.TypeRef{Name: a.Name, P: a.P})
	}
	cls := c.lookupClass(a.Name)
	c.lookupCls = saved
	if cls != nil {
		// A class path class loads its members, Retention among them,
		// only on Ensure, and files are checked in parallel.
		cls.Ensure()
	}
	if (cls == nil || cls.Retention == types.NoRetention) && c.optionalAnnotations(a.Name) {
		return nil
	}
	if cls == nil {
		switch {
		case !strings.Contains(a.Name, ".") && annotTypeParam(a.Name, owner, c.tparams):
			c.errorf(a.P, "not a class")
		case c.lookupClass("kotlin.Suppress") == nil:
			// Without the standard library on the class path, kc still
			// knows its annotations by name, so a name can't be judged.
		default:
			c.errorf(a.P, "unresolved reference: %s", a.Name)
		}
		return nil
	}
	if cls.Retention == types.NoRetention {
		c.errorf(a.P, "%s is not an annotation class", cls.DisplayName())
		return nil
	}
	return cls
}

// annotTypeParam reports whether name is a type parameter of class owner,
// the classes around it, or among tps.
func annotTypeParam(name string, owner *types.ClassSym, tps []*types.TypeParamSym) bool {
	for _, p := range tps {
		if p.Name == name {
			return true
		}
	}
	for k := owner; k != nil; k = k.Outer {
		for _, p := range k.TypeParams {
			if p.Name == name {
				return true
			}
		}
	}
	return false
}

// optionalAnnotations reports whether name may be one of the standard
// library's annotation classes for other platforms, as kotlin.js.JsName
// or kotlin.native.concurrent.ThreadLocal, which are optional
// expectations: the JVM's library has no class for them, but kotlinc
// knows them.
func (c *Checker) optionalAnnotations(name string) bool {
	optional := func(path string) bool {
		return strings.HasPrefix(path, "kotlin.js.") || strings.HasPrefix(path, "kotlin.native.") ||
			path == "kotlin.js" || path == "kotlin.native"
	}
	if optional(name) {
		return true
	}
	if c.file == nil {
		return false
	}
	for _, imp := range c.file.Imports {
		if optional(imp.Path) {
			return true
		}
	}
	return false
}
