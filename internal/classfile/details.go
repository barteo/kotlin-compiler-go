package classfile

// Details are the parts of a class file that Parse skips and the Analysis
// API reports (analysis/ka): the arguments of the annotations of the class,
// its members and their parameters, the names of methods' parameters, and
// the class's nested classes.
type Details struct {
	// Annotations are the class's annotations, those kept in the class
	// file first, then those visible at run time.
	Annotations []Annotation
	Fields      []MemberDetails
	Methods     []MemberDetails
	// Nested are the internal names of the class's named nested classes,
	// in InnerClasses order.
	Nested []string
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
}

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
	for range r.u2() {
		attr, _ := p.utf8(r.u2())
		size := int(r.u4())
		end := r.pos + size
		switch attr {
		case "RuntimeInvisibleAnnotations":
			d.Annotations = append(d.Annotations, readAnnotations(r, p)...)
		case "RuntimeVisibleAnnotations":
			visible = append(visible, readAnnotations(r, p)...)
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
