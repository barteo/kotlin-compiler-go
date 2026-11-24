package classfile

// Annotated holds the annotations of a class file with their values: the
// class's, its fields', its methods' and its methods' parameters', both those kept in
// the class file only and those visible at run time. Parse keeps only
// their descriptors.
type Annotated struct {
	Class   []Annotation
	Methods []AnnotatedMethod
	// Fields are the fields' annotations, as methods without parameters.
	Fields []AnnotatedMethod
	// Nested are the internal names of the classes nested in the class,
	// from its InnerClasses attribute.
	Nested []string
}

// AnnotatedMethod is a method's annotations and its parameters'.
type AnnotatedMethod struct {
	Name, Desc string
	Access     uint16
	Annots     []Annotation
	Params     [][]Annotation
}

// ParseAnnotated reads the annotations of a class file.
func ParseAnnotated(data []byte) (*Annotated, error) {
	r := &reader{b: data}
	p, err := readPool(r)
	if err != nil {
		return nil, err
	}
	a := &Annotated{}
	r.skip(2) // access
	this := r.u2()
	r.skip(2) // super
	r.skip(2 * int(r.u2()))
	annots := func(size int) []Annotation {
		end := r.pos + size
		var as []Annotation
		for range r.u2() {
			v, ok := readAnnotation(r, p)
			if !ok {
				break
			}
			as = append(as, *v.Ann)
		}
		r.pos = min(end, len(r.b))
		return as
	}
	params := func(size int, into [][]Annotation) [][]Annotation {
		end := r.pos + size
		n := int(r.u1())
		if len(into) < n {
			into = append(into, make([][]Annotation, n-len(into))...)
		}
		for i := range n {
			for range r.u2() {
				v, ok := readAnnotation(r, p)
				if !ok {
					r.pos = min(end, len(r.b))
					return into
				}
				into[i] = append(into[i], *v.Ann)
			}
		}
		r.pos = min(end, len(r.b))
		return into
	}
	for kind := range 2 {
		for range r.u2() {
			access := r.u2()
			name, _ := p.utf8(r.u2())
			desc, _ := p.utf8(r.u2())
			m := AnnotatedMethod{Name: name, Desc: desc, Access: access}
			for range r.u2() {
				attr, _ := p.utf8(r.u2())
				size := int(r.u4())
				switch attr {
				case "RuntimeVisibleAnnotations", "RuntimeInvisibleAnnotations":
					m.Annots = append(m.Annots, annots(size)...)
				case "RuntimeVisibleParameterAnnotations", "RuntimeInvisibleParameterAnnotations":
					m.Params = params(size, m.Params)
				default:
					r.skip(size)
				}
			}
			if kind == 0 {
				a.Fields = append(a.Fields, m)
			} else {
				a.Methods = append(a.Methods, m)
			}
		}
	}
	for range r.u2() {
		attr, _ := p.utf8(r.u2())
		size := int(r.u4())
		switch attr {
		case "RuntimeVisibleAnnotations", "RuntimeInvisibleAnnotations":
			a.Class = append(a.Class, annots(size)...)
		case "InnerClasses":
			end := r.pos + size
			for range r.u2() {
				inner, outer := r.u2(), r.u2()
				r.skip(4) // the simple name and the access
				if inner != 0 && outer == this {
					if name, err := p.class(inner); err == nil {
						a.Nested = append(a.Nested, name)
					}
				}
			}
			r.pos = min(end, len(r.b))
		default:
			r.skip(size)
		}
	}
	if r.err != nil {
		return nil, r.err
	}
	return a, nil
}
