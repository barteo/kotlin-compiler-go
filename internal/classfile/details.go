package classfile

import "slices"

// Details are the parts of a class file that Parse skips and the Analysis
// API reports (analysis/ka): the arguments of the annotations of the class,
// its members and their parameters, the names of methods' parameters, and
// the class's nested classes.
type Details struct {
	// Annotations are the class's annotations, those kept in the class
	// file first, then those visible at run time.
	Annotations []Annotation
	// Invisible is the number of Annotations kept in the class file only.
	Invisible int
	Fields    []MemberDetails
	Methods   []MemberDetails
	// Nested are the internal names of the class's named nested classes,
	// in InnerClasses order.
	Nested []string
	// TypeAnnotations are the annotations of the types in the class's
	// signature: its type parameters, their bounds and its supertypes.
	TypeAnnotations []TypeAnnotation
}

// MemberDetails are a field's or method's Details.
type MemberDetails struct {
	Access      uint16
	Name, Desc  string
	Annotations []Annotation
	// Params are a method's parameters' annotations, by parameter; nil if
	// it has none. Its descriptor may have more parameters (javac leaves
	// out synthetic ones).
	Params [][]Annotation
	// ParamNames are the names in its MethodParameters attribute, if it
	// has one.
	ParamNames []string
	// Default is an annotation method's AnnotationDefault, or nil.
	Default *ElementValue
	// TypeAnnotations are the annotations of the types in its signature,
	// those visible at run time first, then those kept in the class file
	// only, as ASM reads them.
	TypeAnnotations []TypeAnnotation
}

// TypeAnnotation is an annotation of a type in a field's or method's
// signature (JVMS 4.7.20): @A in int @A [], List<@A String> or @A int f.
type TypeAnnotation struct {
	// Target is the kind of type it annotates: TargetField and so on.
	Target byte
	// Index is a TargetParam's parameter, a type parameter's (the
	// TargetTypeParam targets and their bounds'), a TargetThrows'
	// exception, or a TargetSupertype's interface (65535 for the
	// superclass).
	Index int
	// Bound is a type parameter bound's bound: 0 is the class bound, the
	// interface bounds follow, from 1 even with no class bound.
	Bound int
	// Path is where in that type the annotated type is.
	Path       []PathStep
	Annotation Annotation
}

// The targets of class and member type annotations (JVMS table 4.7.20-A).
const (
	TargetClassTypeParam      = 0x00
	TargetTypeParam           = 0x01
	TargetSupertype           = 0x10
	TargetClassTypeParamBound = 0x11
	TargetTypeParamBound      = 0x12
	TargetField          = 0x13
	TargetReturn         = 0x14
	TargetReceiver       = 0x15
	TargetParam          = 0x16
	TargetThrows         = 0x17
)

// PathStep is a step of a type annotation's path (JVMS 4.7.20.2): into an
// array's element type (PathArray), a nested type (PathNested), a
// wildcard's bound (PathWildcard) or type argument Arg (PathTypeArg).
type PathStep struct {
	Kind, Arg byte
}

// The kinds of a PathStep.
const (
	PathArray    = 0
	PathNested   = 1
	PathWildcard = 2
	PathTypeArg  = 3
)

// ParseDetails reads the Details of a class file.
func ParseDetails(data []byte) (*Details, error) {
	r := &reader{b: data}
	p, err := readPool(r)
	if err != nil {
		return nil, err
	}
	d := &Details{}
	r.skip(2) // access
	this := r.u2()
	r.skip(2) // super
	r.skip(2 * int(r.u2()))
	name, _ := p.class(this)
	members := func() []MemberDetails {
		ms := make([]MemberDetails, r.u2())
		for i := range ms {
			m := &ms[i]
			m.Access = r.u2()
			m.Name, _ = p.utf8(r.u2())
			m.Desc, _ = p.utf8(r.u2())
			visible := 0 // the number of visible TypeAnnotations, which come first
			for range r.u2() {
				attr, _ := p.utf8(r.u2())
				size := int(r.u4())
				end := r.pos + size
				switch attr {
				case "RuntimeInvisibleAnnotations", "RuntimeVisibleAnnotations":
					m.Annotations = append(m.Annotations, readAnnotations(r, p)...)
				case "RuntimeInvisibleParameterAnnotations", "RuntimeVisibleParameterAnnotations":
					n := int(r.u1())
					if len(m.Params) < n {
						m.Params = append(m.Params, make([][]Annotation, n-len(m.Params))...)
					}
					for j := range n {
						m.Params[j] = append(m.Params[j], readAnnotations(r, p)...)
					}
				case "MethodParameters":
					for range r.u1() {
						pn, _ := p.utf8(r.u2()) // 0: no name
						r.skip(2)               // access
						m.ParamNames = append(m.ParamNames, pn)
					}
				case "RuntimeVisibleTypeAnnotations":
					as := readTypeAnnotations(r, p)
					m.TypeAnnotations = slices.Insert(m.TypeAnnotations, visible, as...)
					visible += len(as)
				case "RuntimeInvisibleTypeAnnotations":
					m.TypeAnnotations = append(m.TypeAnnotations, readTypeAnnotations(r, p)...)
				case "AnnotationDefault":
					if v, ok := readValue(r, p, 0); ok {
						m.Default = &v
					}
				}
				if r.err != nil {
					return ms
				}
				r.pos = end
			}
		}
		return ms
	}
	d.Fields = members()
	d.Methods = members()
	var visible []Annotation
	visibleTypes := 0
	for range r.u2() {
		attr, _ := p.utf8(r.u2())
		size := int(r.u4())
		end := r.pos + size
		switch attr {
		case "RuntimeInvisibleAnnotations":
			d.Annotations = append(d.Annotations, readAnnotations(r, p)...)
		case "RuntimeVisibleAnnotations":
			visible = append(visible, readAnnotations(r, p)...)
		case "RuntimeVisibleTypeAnnotations":
			as := readTypeAnnotations(r, p)
			d.TypeAnnotations = slices.Insert(d.TypeAnnotations, visibleTypes, as...)
			visibleTypes += len(as)
		case "RuntimeInvisibleTypeAnnotations":
			d.TypeAnnotations = append(d.TypeAnnotations, readTypeAnnotations(r, p)...)
		case "InnerClasses":
			for range r.u2() {
				inner, outer, simple := r.u2(), r.u2(), r.u2()
				access := r.u2()
				if inner == 0 || outer == 0 || simple == 0 || access&AccSynthetic != 0 {
					continue
				}
				if o, err := p.class(outer); err != nil || o != name {
					continue
				}
				if in, err := p.class(inner); err == nil {
					d.Nested = append(d.Nested, in)
				}
			}
		}
		if r.err != nil {
			return nil, r.err
		}
		r.pos = end
	}
	d.Invisible = len(d.Annotations)
	d.Annotations = append(d.Annotations, visible...)
	if r.err != nil {
		return nil, r.err
	}
	return d, nil
}

// readAnnotations reads a Runtime*Annotations attribute's annotations,
// dropping the ones it can't decode.
func readAnnotations(r *reader, p *pool) []Annotation {
	var out []Annotation
	for range r.u2() {
		v, ok := readAnnotation(r, p)
		if r.err != nil {
			break
		}
		if ok {
			out = append(out, *v.Ann)
		}
	}
	return out
}

// readTypeAnnotations reads a class's, field's or method's
// Runtime*TypeAnnotations attribute's annotations, dropping the ones it
// can't decode; it stops at a target they can't have (a Code attribute's).
func readTypeAnnotations(r *reader, p *pool) []TypeAnnotation {
	var out []TypeAnnotation
	for range r.u2() {
		a := TypeAnnotation{Target: r.u1()}
		switch a.Target {
		case TargetClassTypeParam, TargetTypeParam, TargetParam:
			a.Index = int(r.u1())
		case TargetClassTypeParamBound, TargetTypeParamBound:
			a.Index, a.Bound = int(r.u1()), int(r.u1())
		case TargetField, TargetReturn, TargetReceiver:
		case TargetSupertype, TargetThrows:
			a.Index = int(r.u2())
		default:
			return out
		}
		for range r.u1() {
			a.Path = append(a.Path, PathStep{Kind: r.u1(), Arg: r.u1()})
		}
		v, ok := readAnnotation(r, p)
		if r.err != nil {
			break
		}
		if ok {
			a.Annotation = *v.Ann
			out = append(out, a)
		}
	}
	return out
}

// readAnnotation reads one annotation as an '@' element value; r is past
// it even if it can't be decoded.
func readAnnotation(r *reader, p *pool) (ElementValue, bool) {
	typ, err := p.utf8(r.u2())
	v := ElementValue{Tag: '@', Ann: &Annotation{Type: typ}}
	ok := err == nil
	for range r.u2() {
		name, err := p.utf8(r.u2())
		at := r.pos
		e, eok := readValue(r, p, 1)
		if !eok {
			// Skip what readValue left: it stops at the first bad
			// constant.
			r.pos = at
			skipValue(r)
			ok = false
			continue
		}
		if err != nil {
			ok = false
			continue
		}
		v.Ann.Values = append(v.Ann.Values, ElementPair{Name: name, Value: e})
	}
	return v, ok && r.err == nil
}
