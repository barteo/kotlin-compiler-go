package check

import (
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// Where an annotation can be written is checked as kotlinc does, for the
// annotation classes whose targets kc knows (types.ClassSym.TargetsKnown):
// the program's, and the class path's Kotlin ones and Java ones with a
// @Target. An annotation without a use-site target must have one of the
// targets of the element it is on; one with a use-site target, @get:A,
// that target's, and the element must have it.

// siteKind is the kind of element annotations are written on.
type siteKind uint8

const (
	siteUnchecked siteKind = iota
	siteClass
	siteEnumEntry
	siteCtor
	siteFun
	siteProp     // a property
	siteCtorProp // a constructor's val or var parameter
	siteParam    // a value parameter
	siteTypealias
	siteFile
	siteLocal // a local variable, or a catch block's parameter
	siteExpr  // an expression or another statement
	siteInit  // an init block
)

// annotSite is an element annotations are written on: its kind, the
// targets an annotation on it may have, and kotlinc's name for it.
type annotSite struct {
	kind    siteKind
	targets types.Targets
	name    string
}

var (
	ctorSite      = annotSite{siteCtor, types.TargetConstructor, "constructor"}
	paramSite     = annotSite{siteParam, types.TargetValueParameter, "value parameter"}
	typealiasSite = annotSite{siteTypealias, types.TargetTypealias, "typealias"}
	fileSite      = annotSite{siteFile, types.TargetFile, "file"}
	enumEntrySite = annotSite{siteEnumEntry, types.TargetProperty | types.TargetField, "enum entry"}
	localVarSite  = annotSite{siteLocal, types.TargetLocalVariable, "local variable"}
	catchSite     = annotSite{siteLocal, types.TargetValueParameter | types.TargetLocalVariable, "value parameter"}
	localFunSite  = annotSite{siteFun, types.TargetFunction, "local function"}
	exprSite      = annotSite{siteExpr, types.TargetExpression, "expression"}
	// A lambda and an anonymous function are functions too, and an object
	// expression a class.
	lambdaSite     = annotSite{siteExpr, types.TargetFunction | types.TargetExpression, "expression"}
	objectExprSite = annotSite{siteExpr, types.TargetClass | types.TargetExpression, "expression"}
	initSite       = annotSite{siteInit, 0, "initializer"}
)

func classSite(d *syntax.ClassDecl) annotSite {
	s := annotSite{siteClass, types.TargetClass, "class"}
	switch {
	case d.Annotation:
		s.targets |= types.TargetAnnotationClass
		s.name = "annotation class"
	case d.Enum:
		s.name = "enum class"
	case d.Interface:
		s.name = "interface"
	case d.Companion:
		s.name = "companion object"
	case d.Object:
		s.name = "object"
	}
	return s
}

func funSite(owner *types.ClassSym) annotSite {
	if owner == nil {
		return annotSite{siteFun, types.TargetFunction, "top level function"}
	}
	return annotSite{siteFun, types.TargetFunction, "member function"}
}

// propSite is the site of a property, whose annotations without a use-site
// target go to the property or else its field. kc doesn't check which
// properties have fields.
func propSite(owner *types.ClassSym) annotSite {
	if owner == nil {
		return annotSite{siteProp, types.TargetProperty | types.TargetField, "top level property"}
	}
	return annotSite{siteProp, types.TargetProperty | types.TargetField, "member property"}
}

// ctorParamSite is the site of a parameter of class cls's primary
// constructor: a property's also has the property's targets, but an
// annotation class's properties have no fields.
func ctorParamSite(cls *types.ClassSym, cp *syntax.CtorParam) annotSite {
	if !cp.IsProp {
		return paramSite
	}
	s := annotSite{siteCtorProp, types.TargetValueParameter | types.TargetProperty | types.TargetField, "value parameter"}
	if cls != nil && cls.Annotation {
		s.targets &^= types.TargetField
	}
	return s
}

// useSiteTargets are the targets that use-site targets require.
var useSiteTargets = map[string]types.Targets{
	"field": types.TargetField, "delegate": types.TargetField, "property": types.TargetProperty,
	"get": types.TargetGetter, "set": types.TargetSetter, "setparam": types.TargetValueParameter,
	"param": types.TargetValueParameter, "file": types.TargetFile,
}

// annotationTarget checks where annotation a, of annotation class cls, is
// written: on site.
func (c *Checker) annotationTarget(a *syntax.Annotation, cls *types.ClassSym, site annotSite) {
	if site.kind == siteUnchecked || !cls.TargetsKnown {
		return
	}
	switch {
	case a.Accessor && site.kind == siteProp:
		// @A get(): the getter's.
		t, name := types.TargetGetter, "getter"
		if a.Target == "set" {
			t, name = types.TargetSetter, "setter"
		}
		if cls.Targets&t == 0 {
			c.errorf(a.P, "this annotation is not applicable to target '%s'", name)
		}
	case a.Target == "":
		if cls.Targets&site.targets == 0 {
			c.errorf(a.P, "this annotation is not applicable to target '%s'", site.name)
		}
	default:
		want, ok := useSiteTargets[a.Target]
		if !ok || !c.useSiteApplies(a.Target, site) {
			return
		}
		if cls.Targets&want == 0 || !useSiteOn(a.Target, site) {
			c.errorf(a.P, "this annotation is not applicable to target '%s' and use-site target '@%s'", site.name, a.Target)
		}
	}
}

// useSiteApplies reports whether kc checks use-site target u on site: on
// a property or a constructor's property, a file, or the elements that
// kotlinc reports it on, not those it reports another error for, as
// @param: on a function.
func (c *Checker) useSiteApplies(u string, site annotSite) bool {
	switch site.kind {
	case siteProp:
		return u != "param" && u != "file"
	case siteCtorProp:
		return u != "file"
	case siteFile:
		return u == "file"
	case siteClass, siteCtor:
		return u != "file" && u != "set"
	case siteFun, siteParam:
		switch u {
		case "field", "delegate", "property", "get":
			return true
		}
	}
	return false
}

// useSiteOn reports whether site has use-site target u.
func useSiteOn(u string, site annotSite) bool {
	switch site.kind {
	case siteProp:
		return u != "param"
	case siteCtorProp:
		return true
	case siteFile:
		return u == "file"
	}
	return false
}

// repeatKey identifies the annotations of one class on one target of an
// element, which only a repeatable class can have several of.
type repeatKey struct {
	cls    *types.ClassSym
	target string
}

// repeated reports annotation a, of annotation class cls, on site if
// another annotation among seen has the same class and target, and
// records it.
func (c *Checker) repeated(a *syntax.Annotation, cls *types.ClassSym, site annotSite, seen map[repeatKey]bool) {
	if site.kind == siteUnchecked || !cls.TargetsKnown || cls.Repeatable || a.Target == "all" {
		return
	}
	for _, m := range cls.Annotations {
		if m.Name == "kotlin/annotation/Repeatable" || m.Name == "java/lang/annotation/Repeatable" {
			return
		}
	}
	target := a.Target
	if target == "" {
		switch site.kind {
		case siteProp:
			// A property's go to the property, or else to its field.
			switch {
			case cls.Targets&types.TargetProperty != 0:
				target = "property"
			case cls.Targets&types.TargetField != 0:
				target = "field"
			}
		case siteCtorProp:
			target = "param"
		}
	}
	k := repeatKey{cls, target}
	if seen[k] {
		c.errorf(a.P, "this annotation is not repeatable")
	}
	seen[k] = true
}
