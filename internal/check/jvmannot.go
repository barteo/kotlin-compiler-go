package check

import (
	"slices"
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

// multifileSynthetic reports the parts of a multi-file class that aren't
// annotated @file:JvmSynthetic when another part is, as kotlinc's
// NOT_ALL_MULTIFILE_CLASS_PARTS_ARE_JVM_SYNTHETIC: the facade is either
// synthetic or not.
func multifileSynthetic(checkers []*Checker) {
	synthetic := map[string]bool{}
	for _, c := range checkers {
		if f := c.file; f.Part != "" && fileAnnotation(f, "JvmSynthetic") != nil {
			synthetic[f.Facade] = true
		}
	}
	for _, c := range checkers {
		f := c.file
		if f.Part == "" || !synthetic[f.Facade] || fileAnnotation(f, "JvmSynthetic") != nil {
			continue
		}
		if a := fileAnnotation(f, "JvmMultifileClass"); a != nil {
			c.errorf(a.P, "not all parts of the multi-file class are annotated @JvmSynthetic")
		}
	}
}

// fileAnnotation returns file f's annotation of simple name name, or nil.
func fileAnnotation(f *syntax.File, name string) *syntax.Annotation {
	for i := range f.Annotations {
		if a := &f.Annotations[i]; a.Is(name) {
			return a
		}
	}
	return nil
}

// exposeBoxedPrivate reports @JvmExposeBoxed on the declarations among ds
// that are private, or in a private class if outer, as kotlinc's
// JVM_EXPOSE_BOXED_CANNOT_EXPOSE_PRIVATE: Java can't call them.
func (c *Checker) exposeBoxedPrivate(ds []syntax.Decl, outer bool) {
	report := func(as []syntax.Annotation, private bool, target string) {
		if !private {
			return
		}
		for _, a := range as {
			if a.Is("JvmExposeBoxed") && a.Target == target {
				c.errorf(a.P, "'@JvmExposeBoxed' cannot be applied to a private declaration")
			}
		}
	}
	for _, d := range ds {
		switch d := d.(type) {
		case *syntax.FunDecl:
			report(d.Annotations, outer || d.Visibility == syntax.Private, "")
		case *syntax.PropDecl:
			private := outer || d.Visibility == syntax.Private
			getter := false
			for i, ms := range d.AccessorMods {
				if !d.AccessorSets[i] && slices.ContainsFunc(ms, func(m syntax.Mod) bool { return m.Name == "private" }) {
					getter = true
				}
			}
			report(d.Annotations, private || getter, "get")
			report(d.Annotations, private || d.SetterVis == syntax.Private, "set")
		case *syntax.ClassDecl:
			private := outer || d.Visibility == syntax.Private
			report(d.Annotations, private, "")
			report(d.CtorMods.Annotations, private || d.CtorMods.Visibility == syntax.Private, "")
			for _, sc := range d.Secondaries() {
				report(sc.Annotations, private || sc.Visibility == syntax.Private, "")
			}
			c.exposeBoxedPrivate(d.Members, private)
		}
	}
}
