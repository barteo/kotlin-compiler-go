package javasrc

import "strings"

// impliedOuter returns javac's signature of t, a simple name of class n,
// if n is an inner class of a generic class that the enclosing classes
// are or extend: Inner in Outer<H> is Outer<H>.Inner, and BaseInner in a
// subclass of BaseOuter<H> is BaseOuter<H>.BaseInner.
func (r *resolver) impliedOuter(t *Type, n string) (string, bool) {
	if len(t.Segs) != 1 || r.c == nil {
		return "", false
	}
	inner := r.s.classes[n]
	if inner == nil || static(inner) {
		return "", false
	}
	outer := internalName(inner.Outer)
	for e := r.c; e != nil; e = e.Outer {
		if internalName(e) == outer {
			if len(e.TypeParams) == 0 {
				return "", false
			}
			var b strings.Builder
			b.WriteString("L" + outer + "<")
			for _, tp := range e.TypeParams {
				b.WriteString("T" + tp.Name + ";")
			}
			b.WriteString(">")
			return r.innerOf(&b, t, inner), true
		}
		sr := superResolver(r.s, e)
		for _, st := range append(append([]*Type{}, e.Extends...), e.Implements...) {
			if name, _ := sr.classType(st); name == outer && st.Dims == 0 && sr.generic(st) {
				var b strings.Builder
				b.WriteString(strings.TrimSuffix(sr.sig(st), ";"))
				return r.innerOf(&b, t, inner), true
			}
		}
		if static(e) {
			break
		}
	}
	return "", false
}

// innerOf finishes the signature of inner, the class of t, after its
// outer class's in b.
func (r *resolver) innerOf(b *strings.Builder, t *Type, inner *Class) string {
	b.WriteString("." + inner.Name)
	r.argsSig(b, t.Segs[0].Args)
	b.WriteByte(';')
	return b.String()
}
