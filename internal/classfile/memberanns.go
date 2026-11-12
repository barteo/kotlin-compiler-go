package classfile

import "fmt"

// MethodAnnotations are the annotations of a method in a class file and
// of its parameters, by its descriptor's parameters, and whether it has
// the Deprecated attribute.
type MethodAnnotations struct {
	Visible, Invisible             []Annotation
	VisibleParams, InvisibleParams [][]Annotation
	Deprecated                     bool
}

// ReadMethodAnnotations returns the annotations of the method of class
// file data with the given name and descriptor.
func ReadMethodAnnotations(data []byte, name, desc string) (*MethodAnnotations, error) {
	r := &reader{b: data}
	p, err := readPool(r)
	if err != nil {
		return nil, err
	}
	r.skip(6) // access, this, super
	r.skip(2 * int(r.u2()))
	skipAttrs := func() {
		for range r.u2() {
			r.skip(2)
			r.skip(int(r.u4()))
		}
	}
	for range r.u2() { // fields
		r.skip(6)
		skipAttrs()
	}
	for range r.u2() {
		r.skip(2)
		n, err1 := p.utf8(r.u2())
		d, err2 := p.utf8(r.u2())
		if err1 != nil || err2 != nil || n != name || d != desc {
			skipAttrs()
			continue
		}
		out := &MethodAnnotations{}
		for range r.u2() {
			attr, err := p.utf8(r.u2())
			if err != nil {
				return nil, err
			}
			size := int(r.u4())
			end := r.pos + size
			switch attr {
			case "Deprecated":
				out.Deprecated = true
			case "RuntimeVisibleAnnotations":
				out.Visible, err = readAnnotations(r, p)
			case "RuntimeInvisibleAnnotations":
				out.Invisible, err = readAnnotations(r, p)
			case "RuntimeVisibleParameterAnnotations":
				out.VisibleParams, err = readParamAnnotations(r, p)
			case "RuntimeInvisibleParameterAnnotations":
				out.InvisibleParams, err = readParamAnnotations(r, p)
			}
			if err != nil {
				return nil, err
			}
			r.pos = min(end, len(r.b))
		}
		if r.err != nil {
			return nil, r.err
		}
		return out, nil
	}
	if r.err != nil {
		return nil, r.err
	}
	return nil, fmt.Errorf("%w: %s%s", ErrNotFound, name, desc)
}

func readAnnotations(r *reader, p *pool) ([]Annotation, error) {
	var out []Annotation
	for range r.u2() {
		t, err := p.utf8(r.u2())
		if err != nil {
			return nil, err
		}
		a := Annotation{Type: t}
		for range r.u2() {
			name, err := p.utf8(r.u2())
			v, ok := readValue(r, p, 1)
			if err != nil || !ok {
				return nil, fmt.Errorf("unreadable annotation %s", t)
			}
			a.Values = append(a.Values, ElementPair{Name: name, Value: v})
		}
		out = append(out, a)
	}
	return out, nil
}

func readParamAnnotations(r *reader, p *pool) ([][]Annotation, error) {
	out := make([][]Annotation, r.u1())
	for i := range out {
		as, err := readAnnotations(r, p)
		if err != nil {
			return nil, err
		}
		out[i] = as
	}
	return out, nil
}
