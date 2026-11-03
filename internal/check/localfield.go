package check

import "github.com/barteo/kotlin-compiler-go/internal/types"

// The backing field of a property in a local class or an object
// expression of its accessor: field there is the accessor's this's,
// which the class captures as it does this@Outer.

// localField returns field, the backing field symbol of property p, as
// read in a local class or an object expression's class in p's accessor:
// a copy whose OuterOf is the enclosing class's this, which it captures.
// It returns nil in p's own class.
func (c *Checker) localField(sym *types.VarSym) *types.VarSym {
	p := sym.Prop
	if c.cls == nil || c.cls == p.Owner || c.cls.IsSubclassOf(p.Owner) {
		return nil
	}
	for i := len(c.recvs) - 1; i >= 0; i-- {
		r := &c.recvs[i]
		if !r.class || r.sym.Type.K != types.Class || r.sym.Type.C != p.Owner && !r.sym.Type.C.IsSubclassOf(p.Owner) {
			continue
		}
		if r.sym != c.noCapture {
			c.capture(r.sym, r.depth)
		}
		v := *sym
		v.OuterOf = r.sym
		return &v
	}
	return nil
}
