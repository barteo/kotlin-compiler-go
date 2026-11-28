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
	cls := c.lookupClass(a.Name)
	if r, ok := c.u.rec.(AnnotationAliasRecorder); ok && cls != nil {
		if al := c.lookupAlias(a.Name); al != nil && c.aliasClass(al) == cls {
			r.AnnotationAlias(c.file, a, al)
		}
	}
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
			c.errorf(a.P, "NOT_A_CLASS", "not a class")
		case c.lookupClass("kotlin.Suppress") == nil:
			// Without the standard library on the class path, kc still
			// knows its annotations by name, so a name can't be judged.
		default:
			c.errorf(a.P, "UNRESOLVED_REFERENCE", "unresolved reference: %s", a.Name)
		}
		return nil
	}
	if cls.Retention == types.NoRetention {
		c.errorf(a.P, "NOT_AN_ANNOTATION_CLASS", "%s is not an annotation class", cls.DisplayName())
		return nil
	}
	if k := c.requiredTargetAmbiguity(a, cls); k != nil {
		return k
	}
	return cls
}

// requiredTargetAmbiguity reports @Target, written without a qualifier,
// that names a nested class, its own or inherited, not imported, and
// returns kotlin.annotation.Target, which kotlinc reads it as before it
// resolves types: as K2 has it, an ambiguity, with the arguments checked
// as Target's (COMPILER_REQUIRED_ANNOTATION_AMBIGUITY).
func (c *Checker) requiredTargetAmbiguity(a *syntax.Annotation, cls *types.ClassSym) *types.ClassSym {
	if a.Name != "Target" || cls.Outer == nil || cls.External || cls.Name == "kotlin/annotation/Target" {
		return nil
	}
	for _, im := range c.file.Imports {
		if !im.Star && (im.Alias == "Target" || im.Alias == "" && strings.HasSuffix("."+im.Path, ".Target")) {
			return nil
		}
	}
	k := c.lookupClass("kotlin.annotation.Target")
	if k == nil {
		return nil
	}
	k.Ensure()
	c.errorf(a.P, "COMPILER_REQUIRED_ANNOTATION_AMBIGUITY", "resolution of the annotation type is ambiguous between '%s' and the compiler-required annotation 'Target': specify a fully qualified annotation name", cls.DisplayName())
	return k
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

// findAnnotation returns the annotation of as with use-site target
// target that is of class fq (as kotlin/jvm/JvmStatic), written by its
// name or through a typealias of it, or nil.
func (c *Checker) findAnnotation(as []syntax.Annotation, target, name, fq string) *syntax.Annotation {
	for i := range as {
		a := &as[i]
		if a.Target != target {
			continue
		}
		if a.Is(name) {
			return a
		}
		if strings.Contains(a.Name, ".") || c.lookupAlias(a.Name) == nil {
			continue
		}
		if cls := c.lookupClass(a.Name); cls != nil && cls.Name == fq {
			return a
		}
	}
	return nil
}
