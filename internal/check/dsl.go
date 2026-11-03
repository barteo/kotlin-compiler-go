package check

import (
	"slices"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// DSL markers: an annotation class annotated @DslMarker marks the classes
// it annotates, and their subclasses. A use that takes a marked implicit
// value, as its receiver or a context argument, is an error when another
// implicit value with the same marker is nearer, or of the same level, as
// in kotlinc: the outer receiver of nested builders has to be explicit.
// Markers on types, (@M A).() -> Unit, aren't seen: kc's types don't keep
// annotations.

// dslKey is a use for dslUse: a call or a property read, and which value
// it takes, -1 its receiver, -2 a member extension's dispatch receiver, or
// the index of a context argument.
type dslKey struct {
	x syntax.Expr
	k int
}

// classAnnotations resolves the classes of the annotations of a class of
// the program, as as, for dslMarkers.
func (c *Checker) classAnnotations(as []syntax.Annotation) []*types.ClassSym {
	var out []*types.ClassSym
	for i := range as {
		a := &as[i]
		if a.Name == "" || a.Target != "" {
			continue
		}
		if cls := c.lookupClass(a.Name); cls != nil {
			out = append(out, cls)
		}
	}
	return out
}

// dslMarkers returns the DSL markers of values of type t: those of its
// class and of the class's supertypes, or of a type parameter's bounds.
func (c *Checker) dslMarkers(t types.Type) []*types.ClassSym {
	switch t.K {
	case types.TypeParam, types.TypeVar:
		var out []*types.ClassSym
		for _, b := range t.Param().Bounds {
			for _, m := range c.dslMarkers(b) {
				if !slices.Contains(out, m) {
					out = append(out, m)
				}
			}
		}
		return out
	case types.Class:
	default:
		return nil
	}
	if ms, ok := c.markers[t.C]; ok {
		return ms
	}
	var out []*types.ClassSym
	t.C.Supertypes(func(s *types.ClassSym) bool {
		for _, a := range s.Annotations {
			if !slices.Contains(out, a) && isDslMarker(a) {
				out = append(out, a)
			}
		}
		return true
	})
	if c.markers == nil {
		c.markers = map[*types.ClassSym][]*types.ClassSym{}
	}
	c.markers[t.C] = out
	return out
}

// isDslMarker reports whether annotation class a is annotated @DslMarker.
func isDslMarker(a *types.ClassSym) bool {
	a.Ensure()
	for _, m := range a.Annotations {
		if m.Name == "kotlin/DslMarker" {
			return true
		}
	}
	return false
}

// implicitVal is an implicit value in scope: a receiver or a context
// parameter, or the class's this, whose sym is nil. Level 0 is the
// innermost; a lambda's or a function's receiver and context parameters
// are of one level.
type implicitVal struct {
	sym   *types.VarSym
	t     types.Type
	level int
}

// implicitVals returns the implicit values in scope, innermost first, in
// contextValue's order.
func (c *Checker) implicitVals() []implicitVal {
	var out []implicitVal
	i, j := len(c.recvs)-1, len(c.ctxs)-1
	level := -1
	var last [2]int
	for i >= 0 || j >= 0 {
		var r *implicitRecv
		var lv [2]int
		if j >= 0 && (i < 0 || c.ctxs[j].at > i) {
			r = &c.ctxs[j]
			lv = [2]int{r.depth, r.at}
			j--
		} else {
			r = &c.recvs[i]
			lv = [2]int{r.depth, i}
			i--
		}
		if level < 0 || lv != last {
			level, last = level+1, lv
		}
		out = append(out, implicitVal{r.sym, r.sym.Type, level})
	}
	if c.cls != nil {
		out = append(out, implicitVal{nil, c.cls.This(), level + 1})
	}
	return out
}

// dslViolation reports whether implicit value v, or the class's this if
// v is nil, has a DSL marker that another implicit value as near or
// nearer has too.
func (c *Checker) dslViolation(v *types.VarSym) bool {
	var ms []*types.ClassSym
	switch {
	case v != nil:
		ms = c.dslMarkers(v.Type)
	case c.cls != nil:
		ms = c.dslMarkers(c.cls.This())
	}
	if len(ms) == 0 {
		return false
	}
	vals := c.implicitVals()
	k := slices.IndexFunc(vals, func(iv implicitVal) bool { return iv.sym == v })
	if k < 0 {
		return false
	}
	for i, w := range vals {
		if i == k || w.level > vals[k].level {
			continue
		}
		for _, m := range c.dslMarkers(w.t) {
			if slices.Contains(ms, m) {
				return true
			}
		}
	}
	return false
}

// dslUse records use key, of name at p, if it takes implicit value v (nil:
// the class's this) where dslViolation says it can't, to report it once
// checked if still says it resolves as it did. As with shadowedUse, the
// error must not make the calls that try candidates try another.
func (c *Checker) dslUse(key dslKey, v *types.VarSym, name string, p syntax.Pos, still func() bool) {
	delete(c.dslUses, key)
	if !c.dslViolation(v) {
		return
	}
	if c.dslUses == nil {
		c.dslUses = map[dslKey]shadowedUse{}
	}
	c.dslUses[key] = shadowedUse{name, p, still}
}

// dslCall checks the implicit receivers that call x of callee cl takes.
func (c *Checker) dslCall(x *syntax.Call, cl *callee) {
	f := cl.f
	if f.Static || f.Name == "<init>" {
		return
	}
	still := func() bool { return x.Sym == f }
	switch fn := x.Fun.(type) {
	case *syntax.Member:
		c.dslMember(x, fn, f.Ext, f.Owner, f.Name, still)
	case *syntax.Ident:
		if !f.Ext && f.Owner != nil && c.cls != nil && c.cls.IsSubclassOf(f.Owner) {
			c.dslUse(dslKey{x, -1}, nil, f.Name, fn.P, still)
		}
	}
}

// dslProp checks the implicit receiver of x, a property read.
func (c *Checker) dslProp(x *syntax.Ident) {
	still := func() bool { return x.Member == nil }
	switch m := x.Member; {
	case m == nil:
		if p := x.Sym; p != nil && p.Kind == types.Prop && p.Owner != nil && c.cls != nil && c.cls.IsSubclassOf(p.Owner) {
			c.dslUse(dslKey{x, -1}, nil, x.Name, x.P, func() bool { return x.Member == nil && x.Sym == p })
		}
	case m.Prop != nil && m.Prop.Owner != nil:
		still = func() bool { return x.Member == m }
		c.dslMember(x, m, false, m.Prop.Owner, x.Name, still)
	case m.Getter != nil && !m.Getter.Static:
		still = func() bool { return x.Member == m }
		c.dslMember(x, m, m.Getter.Ext, m.Getter.Owner, x.Name, still)
	}
}

// dslTarget checks the implicit receiver of x, a property assigned.
func (c *Checker) dslTarget(x *syntax.Member) {
	p, g := x.Prop, x.Getter
	still := func() bool { return x.Prop == p && x.Getter == g }
	switch {
	case p != nil && p.Owner != nil:
		c.dslMember(x, x, false, p.Owner, x.Name, still)
	case g != nil && !g.Static:
		c.dslMember(x, x, g.Ext, g.Owner, x.Name, still)
	}
}

// dslMember checks the implicit receivers of use x, of m, a member of
// class owner or, if ext, an extension: its receiver, and a member
// extension's dispatch receiver.
func (c *Checker) dslMember(x syntax.Expr, m *syntax.Member, ext bool, owner *types.ClassSym, name string, still func() bool) {
	if th, _ := m.X.(*syntax.This); th != nil && th.Implicit {
		c.dslUse(dslKey{x, -1}, th.Recv, name, m.P, still)
	}
	if !ext || owner == nil || m.Dispatch != nil {
		return
	}
	switch {
	case m.DispatchThis != nil:
		if m.DispatchThis.Implicit {
			c.dslUse(dslKey{x, -2}, m.DispatchThis.Recv, name, m.P, still)
		}
	case c.cls != nil && c.cls.IsSubclassOf(owner):
		c.dslUse(dslKey{x, -2}, nil, name, m.P, still)
	}
}

// dslContext checks context argument k, v, of call x of f: named by its
// property, for an accessor's.
func (c *Checker) dslContext(x *syntax.Call, k int, v syntax.Expr, f *types.FunSym) {
	th, ok := v.(*syntax.This)
	if !ok {
		return
	}
	name, p := f.Name, x.P
	if prop, ok := c.accessed[x]; ok {
		name = prop
	}
	switch fn := x.Fun.(type) {
	case *syntax.Ident:
		p = fn.P
	case *syntax.Member:
		p = fn.P
	}
	c.dslUse(dslKey{x, k}, th.Recv, name, p, func() bool { return k < len(x.Ctx) && x.Ctx[k] == v })
}

// dslErrors reports the uses that dslUse recorded, once per use.
func (c *Checker) dslErrors() {
	if c.file.SuppressDSL {
		return
	}
	seen := map[syntax.Expr]bool{}
	for key, u := range c.dslUses {
		if !seen[key.x] && u.still() {
			seen[key.x] = true
			c.errorf(u.p, "'%s' cannot be called in this context with an implicit receiver: use an explicit receiver if necessary", u.name)
		}
	}
}
