package javasrc

import (
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/classfile"
)

// Details returns what a class file's classfile.Details would say of
// class name: its, its members' and its parameters' annotations (their
// classes only, not their arguments), its members' access and parameter
// names, in source order, and its nested classes. It returns nil if the
// sources have no such class.
func (s *Set) Details(name string) *classfile.Details {
	c := s.classes[name]
	if c == nil {
		return nil
	}
	info, err := s.Info(name)
	if err != nil {
		return nil
	}
	r := &resolver{s: s, c: c}
	d := &classfile.Details{Annotations: r.annotations(c.Annotations)}
	// Info lists the declared fields and methods first, in source order.
	for i, m := range info.Fields {
		md := classfile.MemberDetails{Access: m.Access, Name: m.Name, Desc: m.Desc}
		if i < len(c.Fields) {
			md.Annotations = r.annotations(c.Fields[i].Annotations)
		}
		d.Fields = append(d.Fields, md)
	}
	for i, m := range info.Methods {
		md := classfile.MemberDetails{Access: m.Access, Name: m.Name, Desc: m.Desc}
		if i < len(c.Methods) {
			sm := c.Methods[i]
			mr := &resolver{s: s, c: c, mparams: sm.TypeParams}
			md.Annotations = mr.annotations(sm.Annotations)
			if sm.HasDefault {
				// The value isn't evaluated: an element value of no
				// kind says that there is one.
				md.Default = &classfile.ElementValue{}
			}
			annotated := false
			for _, p := range sm.Params {
				md.ParamNames = append(md.ParamNames, p.Name)
				as := mr.annotations(p.Annotations)
				md.Params = append(md.Params, as)
				annotated = annotated || len(as) > 0
			}
			if !annotated {
				md.Params = nil
			}
		} else if c.Kind == KindRecord && m.Name == "<init>" {
			for _, p := range c.Components {
				md.ParamNames = append(md.ParamNames, p.Name)
			}
		}
		d.Methods = append(d.Methods, md)
	}
	for _, n := range c.Nested {
		d.Nested = append(d.Nested, internalName(n))
	}
	return d
}

// annotations returns the classes of as, resolved where r is.
func (r *resolver) annotations(as []*Annotation) []classfile.Annotation {
	var out []classfile.Annotation
	for _, a := range as {
		t := &Type{}
		for _, seg := range strings.Split(a.Name, ".") {
			t.Segs = append(t.Segs, Seg{Name: seg})
		}
		if n, _ := r.classType(t); n != "" {
			out = append(out, classfile.Annotation{Type: "L" + n + ";"})
		}
	}
	return out
}

// nullabilityNames are the simple names of the annotations that
// nullability returns.
var nullabilityNames = map[string]bool{"NotNull": true, "Nullable": true, "NonNull": true, "Nonnull": true,
	"CheckForNull": true, "PossiblyNull": true, "NullableDecl": true, "NonNullDecl": true,
	"RecentlyNullable": true, "RecentlyNonNull": true}

// nullability returns the descriptors of those of as that may be
// nullability annotations, such as @org.jetbrains.annotations.NotNull,
// resolved where r is, as a class file's member lists them; nil if there
// are none. The checker reads them (check.Library.JavaTypes).
func (r *resolver) nullability(as []*Annotation) []string {
	var out []string
	for _, a := range as {
		if !nullabilityNames[a.Name[strings.LastIndexByte(a.Name, '.')+1:]] {
			continue
		}
		for _, c := range r.annotations([]*Annotation{a}) {
			out = append(out, c.Type)
		}
	}
	return out
}
