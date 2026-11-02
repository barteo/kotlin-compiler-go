package analysis

import (
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/codegen"
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// annotationClass returns the class of annotation a, or nil if the
// checker didn't resolve it.
func (b *builder) annotationClass(a *syntax.Annotation) *types.ClassSym {
	switch {
	case a.Class != nil:
		return a.Class
	case a.Source != nil:
		return a.Source
	}
	return b.rec.annotClass[a]
}

// annotationNames returns the fully qualified names of the classes of
// annotations as.
func (b *builder) annotationNames(as []syntax.Annotation) []string {
	var out []string
	for i := range as {
		if as[i].Implicit {
			continue
		}
		if cls := b.annotationClass(&as[i]); cls != nil {
			out = append(out, classFQ(cls))
		}
	}
	return out
}

// declAnnotations returns the annotations of the declaration of a
// function of the program: a primary constructor's are its class's
// constructor modifiers'.
func declAnnotations(d any) ([]syntax.Annotation, bool) {
	switch d := d.(type) {
	case *syntax.FunDecl:
		return d.Annotations, true
	case *syntax.SecondaryCtor:
		return d.Annotations, true
	case *syntax.ClassDecl:
		return d.CtorMods.Annotations, true
	case *syntax.PropDecl:
		return d.Annotations, true
	}
	return nil, false
}

// descriptorNames returns the fully qualified names of the annotation
// classes of descriptors, La/b/C;, or internal names, a/b/C.
func descriptorNames(ds []string) []string {
	var out []string
	for _, d := range ds {
		if strings.HasPrefix(d, "L") && strings.HasSuffix(d, ";") {
			d = d[1 : len(d)-1]
		}
		out = append(out, strings.NewReplacer("/", ".", "$", ".").Replace(d))
	}
	return out
}

// classAnnotationNames returns the fully qualified names of the
// annotations of class c.
func (b *builder) classAnnotationNames(c *types.ClassSym) []string {
	if d, ok := c.Decl.(*syntax.ClassDecl); ok {
		return b.annotationNames(d.Annotations)
	}
	var out []string
	for _, a := range c.Annotations {
		out = append(out, classFQ(a))
	}
	return out
}

// callAnnotations sets the annotations of the callee of c: f, or the
// constructor of class ctor; its class's, not those of the classes that
// enclose it, as detekt reads the containing declaration's; and, of the
// class path, its Java package's.
func (b *builder) callAnnotations(c *Call, f *types.FunSym, ctor *types.ClassSym) {
	owner := ctor
	if f != nil {
		if as, ok := declAnnotations(f.Decl); ok {
			c.Annotations = b.annotationNames(as)
		} else {
			c.Annotations = descriptorNames(f.Annotations)
		}
		if f.Owner != nil {
			owner = f.Owner
		}
	}
	if owner != nil {
		c.OwnerAnnotations = b.classAnnotationNames(owner)
	}
	if owner != nil && owner.External {
		c.PackageAnnotations = b.packageAnnotations(owner.Name)
	} else if f != nil && f.Decl == nil && f.Facade != "" {
		c.PackageAnnotations = b.packageAnnotations(f.Facade)
	}
}

// packageAnnotations returns the fully qualified names of the annotations
// of the package of class name, a/b/C, that its package-info class keeps.
func (b *builder) packageAnnotations(name string) []string {
	i := strings.LastIndexByte(name, '/')
	if i < 0 || b.cp == nil {
		return nil
	}
	pkg := name[:i]
	if names, ok := b.pkgAnnots[pkg]; ok {
		return names
	}
	var names []string
	if b.cp.HasPackage(pkg) {
		if info, err := b.cp.Load(pkg + "/package-info"); err == nil && info != nil {
			names = descriptorNames(append(append([]string{}, info.Invisible...), info.Visible...))
		}
	}
	b.pkgAnnots[pkg] = names
	return names
}

// deprecated returns the message of deprecation d if it is a warning, as
// kotlinc reports a use of what it deprecates; nil if it is an error,
// hidden, or nil.
func deprecated(d *types.Deprecation) *string {
	if d == nil || d.Level != "" && d.Level != "WARNING" {
		return nil
	}
	m := d.Message
	return &m
}

// declDeprecation returns the deprecation of the program's declaration
// with annotations as: its @kotlin.Deprecated's message and level.
func declDeprecation(as []syntax.Annotation) *types.Deprecation {
	for i := range as {
		a := &as[i]
		if a.Class == nil || a.Class.Name != "kotlin/Deprecated" {
			continue
		}
		d := &types.Deprecation{Level: "WARNING"}
		for _, v := range a.Values {
			if len(v.Elems) == 0 || v.Elems[0].X == nil {
				continue
			}
			x := v.Elems[0].X
			switch v.Name {
			case "message":
				if s, ok := codegen.ConstValue(x); ok {
					d.Message, _ = s.(string)
				}
			case "level":
				switch x := x.(type) {
				case *syntax.Member:
					d.Level = x.Name
				case *syntax.Ident:
					d.Level = x.Name
					if x.Member != nil {
						d.Level = x.Member.Name
					}
				}
			}
		}
		return d
	}
	return nil
}

// classDeprecation returns the deprecation of class c.
func classDeprecation(c *types.ClassSym) *types.Deprecation {
	if c == nil {
		return nil
	}
	if d, ok := c.Decl.(*syntax.ClassDecl); ok {
		return declDeprecation(d.Annotations)
	}
	return c.Deprecated
}

// symDeprecation returns the deprecation of what a reference names: a
// function, a constructor (or its class), a property or a class.
func symDeprecation(s any) *types.Deprecation {
	switch s := s.(type) {
	case *types.FunSym:
		if s == nil {
			return nil
		}
		var d *types.Deprecation
		if as, ok := declAnnotations(s.Decl); ok {
			d = declDeprecation(as)
		} else {
			d = s.Deprecated
		}
		if d == nil && s.Name == "<init>" {
			d = classDeprecation(s.Owner)
		}
		return d
	case *types.VarSym:
		if s == nil {
			return nil
		}
		if pd, ok := s.Decl.(*syntax.PropDecl); ok {
			return declDeprecation(pd.Annotations)
		}
	case *types.ClassSym:
		return classDeprecation(s)
	}
	return nil
}
