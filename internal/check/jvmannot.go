package check

import (
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// The JVM annotations of kotlin.jvm have rules of their own, which
// kotlinc's JVM checkers enforce: where @JvmOverloads, @JvmStatic,
// @Synchronized, @JvmSynthetic and @Strictfp can be written, which names
// @JvmName gives, and which packages @JvmPackageName does. jvmAnnotation
// checks those of one annotation by where it is written, and
// jvmMemberAnnotations those that depend on the declaration's class.

// jvmAnnotation checks annotation a, of class cls, written on site.
func (c *Checker) jvmAnnotation(a *syntax.Annotation, cls *types.ClassSym, site annotSite) {
	if !strings.HasPrefix(cls.Name, "kotlin/jvm/") {
		return
	}
	switch strings.TrimPrefix(cls.Name, "kotlin/jvm/") {
	case "JvmName":
		if s, ok := a.Str("name"); ok && !validJvmName(s) {
			c.errorf(a.P, "illegal JVM name")
		}
	case "JvmPackageName":
		if s, ok := a.Str("name"); ok && site == fileSite {
			c.jvmPackageName(a, s)
		}
	case "Strictfp":
		if site.kind == siteClass || site == objectExprSite {
			c.errorf(a.P, "'@Strictfp' annotation on classes is unsupported yet")
		}
	case "JvmOverloads":
		if site == localFunSite {
			c.errorf(a.P, "'@JvmOverloads' annotation cannot be used on local declarations")
		}
	case "JvmStatic":
		if site == localFunSite {
			c.errorf(a.P, "only members in named objects and companion objects can be annotated with '@JvmStatic'")
		}
	}
}

// validJvmName reports whether s can name a method or a class on the
// JVM: it isn't empty, and has none of the characters the class file
// format reserves.
func validJvmName(s string) bool {
	return s != "" && !strings.ContainsAny(s, ".;[/<>")
}

// jvmPackageName checks name, the value of @file:JvmPackageName a.
func (c *Checker) jvmPackageName(a *syntax.Annotation, name string) {
	switch {
	case name == "":
		c.errorf(a.P, "'@JvmPackageName' annotation value cannot be empty")
		return
	case !validPackageName(name):
		c.errorf(a.P, "'@JvmPackageName' annotation value must be a valid dot-qualified name of a package")
		return
	}
	if c.file != nil {
		for _, d := range c.file.Decls {
			if _, ok := d.(*syntax.ClassDecl); ok {
				// kotlinc's "is not supported for files with class
				// declarations", reworded: kc's "not supported" are
				// features it lacks.
				c.errorf(a.P, "'@JvmPackageName' annotation cannot be used in files with class declarations")
				return
			}
		}
	}
}

// validPackageName reports whether s is a dot-qualified name of Java
// identifiers.
func validPackageName(s string) bool {
	for _, part := range strings.Split(s, ".") {
		if part == "" {
			return false
		}
		for i, r := range part {
			letter := r == '_' || r == '$' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r > 0x7f
			if !letter && (i == 0 || r < '0' || r > '9') {
				return false
			}
		}
	}
	return true
}

// jvmMemberAnnotations checks the JVM annotations of declaration d, in
// class owner (nil at top level), that depend on what owner is.
func (c *Checker) jvmMemberAnnotations(d syntax.Decl, owner *types.ClassSym) {
	switch d := d.(type) {
	case *syntax.FunDecl:
		c.overloads(&d.Modifiers, owner)
		c.synchronized(&d.Modifiers, owner, "")
	case *syntax.PropDecl:
		for _, acc := range []*syntax.FunDecl{d.Getter, d.Setter} {
			if acc != nil {
				c.synchronized(&acc.Modifiers, owner, "")
			}
		}
		c.synchronized(&d.Modifiers, owner, "get")
		c.synchronized(&d.Modifiers, owner, "set")
		if d.Field != nil {
			if a := d.Find("delegate", "JvmSynthetic"); a != nil {
				c.errorf(a.P, "'@JvmSynthetic' annotation cannot be used on delegated properties")
			}
		}
	case *syntax.ClassDecl:
		if a := d.CtorMods.Find("", "JvmOverloads"); a != nil && d.Annotation {
			c.errorf(a.P, "'@JvmOverloads' annotation cannot be used on constructors of annotation classes")
		}
		for _, cp := range d.CtorParams {
			if a := cp.Modifiers.Find("", "JvmStatic"); a != nil && cp.IsProp {
				c.errorf(a.P, "only members in named objects and companion objects can be annotated with '@JvmStatic'")
			}
		}
	}
}

// overloads checks the @JvmOverloads of a member function of owner with
// modifiers m.
func (c *Checker) overloads(m *syntax.Modifiers, owner *types.ClassSym) {
	a := m.Find("", "JvmOverloads")
	if a == nil || owner == nil {
		return
	}
	switch {
	case owner.Interface:
		c.errorf(a.P, "'@JvmOverloads' annotation cannot be used on interface methods")
	case m.Abstract:
		c.errorf(a.P, "'@JvmOverloads' annotation cannot be used on abstract methods")
	case localClass(owner):
		c.errorf(a.P, "'@JvmOverloads' annotation cannot be used on local declarations")
	}
}

// synchronized checks the @Synchronized, with use-site target target, of
// a member of owner with modifiers m.
func (c *Checker) synchronized(m *syntax.Modifiers, owner *types.ClassSym, target string) {
	a := m.Find(target, "Synchronized")
	if a == nil || owner == nil {
		return
	}
	switch {
	case owner.Interface:
		c.errorf(a.P, "'@Synchronized' annotation cannot be used on interface members")
	case m.Abstract:
		c.errorf(a.P, "'@Synchronized' annotation cannot be used on abstract functions")
	}
}

// projectedAnnotationParams reports the parameters of annotation class d
// whose array types have projected type arguments, as Array<out A>.
func (c *Checker) projectedAnnotationParams(d *syntax.ClassDecl) {
	for _, cp := range d.CtorParams {
		t := cp.Type
		if t == nil || t.Name != "Array" && t.Name != "kotlin.Array" {
			continue
		}
		for _, arg := range t.Args {
			if arg.Variance == types.Out || arg.Variance == types.In {
				c.errorf(t.P, "projections are not allowed in types of annotation members")
				break
			}
		}
	}
}
