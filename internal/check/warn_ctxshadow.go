package check

import (
	"slices"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// This file reports kotlinc's CONTEXTUAL_OVERLOAD_SHADOWED
// (FirConflictsHelpers, getContextParameterShadowing): two functions, or
// two properties, of a scope with the same signature but for their
// context parameters, where one needs a context parameter for each of
// the other's, at least as specific, and more: calls don't choose
// between them by their context arguments, so a call that could use the
// one that needs more is ambiguous.

// propOverloads appends the properties of a package, or of class owner,
// among decls as overloads, by their receivers and context parameters.
func (c *Checker) propOverloads(out []overload, decls []syntax.Decl, owner *syntax.ClassDecl) []overload {
	for _, d := range decls {
		d, ok := d.(*syntax.PropDecl)
		if !ok || d.Sym == nil || d.Expect || d.DelegateOf != nil {
			continue
		}
		o := overload{c: c, name: d.Name, pos: d.P, decl: d, vararg: -1, prop: true,
			private: owner == nil && d.Visibility == syntax.Private, hidden: d.Hidden()}
		if d.Recv != nil {
			if d.Getter == nil || d.Getter.Sym == nil || !d.Getter.Sym.Ext {
				continue
			}
			g := d.Getter.Sym
			t := g.Recv
			o.recv, o.tparams = &t, g.TypeParams
		} else if len(d.TypeParams) > 0 {
			// Those of a property with context parameters.
			if d.Getter == nil || d.Getter.Sym == nil {
				continue
			}
			o.tparams = d.Getter.Sym.TypeParams
		}
		for _, p := range d.Sym.Contexts {
			o.ctx = append(o.ctx, p.Type)
		}
		out = append(out, o)
	}
	return out
}

// contextualShadowing reports the overloads of one scope that another
// shadows by its context parameters. In a class, as kotlinc collects its
// members in order, one is only reported where it follows the one that
// shadows it.
func contextualShadowing(ovs []overload, ordered bool) {
	byName := map[string][]int{}
	for i := range ovs {
		byName[ovs[i].name] = append(byName[ovs[i].name], i)
	}
	for i := range ovs {
		a := &ovs[i]
		for _, j := range byName[a.name] {
			if j <= i {
				continue
			}
			b := &ovs[j]
			switch {
			case a.ctor || b.ctor, a.prop != b.prop, a.decl == b.decl,
				len(a.ctx) == 0 && len(b.ctx) == 0,
				a.c != b.c && (a.private || b.private),
				a.hidden || b.hidden, a.low != b.low:
				continue
			}
			if len(a.tparams) != len(b.tparams) || (a.recv == nil) != (b.recv == nil) || len(a.params) != len(b.params) || a.vararg != b.vararg {
				continue
			}
			// The same signature but for the contexts: each takes what
			// the other does.
			if ok1, ok2 := a.c.ctxTakes(a, b, false), a.c.ctxTakes(b, a, false); ok1 != certainly || ok2 != certainly {
				continue
			}
			// The one whose contexts the other's satisfy can be called
			// wherever the other can.
			ab, ba := a.c.ctxTakes(a, b, true), a.c.ctxTakes(b, a, true)
			if ab == uncertain || ba == uncertain || ab == ba {
				continue
			}
			bShadowsA := ab == certainly
			o := b
			if bShadowsA {
				if ordered {
					continue
				}
				o = a
			}
			o.c.warnf(o.pos, "CONTEXTUAL_OVERLOAD_SHADOWED", "the overloads of '%s' conflict with this contextual declaration: calls are ambiguous, as context arguments aren't used for overload resolution", o.name)
		}
	}
}

// localShadowing reports local function d, of symbol f, if one declared
// before it in scope s shadows it by its context parameters.
func (c *Checker) localShadowing(s *scope, f *types.FunSym, d *syntax.FunDecl) {
	if len(f.Contexts) == 0 {
		return
	}
	fo := funOverload(c, f, d.P, d)
	for _, g := range s.funs {
		if g != f && g.Name == f.Name {
			contextualShadowing([]overload{funOverload(c, g, d.P, g.Decl), fo}, true)
		}
	}
}

// How certain ctxTakes is.
const (
	uncertain = iota
	certainlyNot
	certainly
)

// ctxTakes reports whether g can be called wherever f can: with f's type
// parameters kept and g's inferred within their bounds, g takes f's
// receiver and parameters, and, with ctx, each of g's context parameters
// takes one of f's. It is uncertain of types that don't resolve, of
// platform types, and of too many context parameters to try.
func (c *Checker) ctxTakes(f, g *overload, ctx bool) int {
	all := slices.Concat(f.ctx, f.params, g.ctx, g.params)
	if f.recv != nil {
		all = append(all, *f.recv, *g.recv)
	}
	if slices.ContainsFunc(all, func(t types.Type) bool { return t.K == types.Invalid || t.P || hasInvalid(t, 0) }) {
		return uncertain
	}
	// from[k] is the context parameter of f that g's k-th takes.
	var from []int
	if ctx {
		if len(g.ctx) > 0 && len(f.ctx) == 0 {
			return certainlyNot
		}
		if n := len(f.ctx) * len(g.ctx); n > 16 {
			return uncertain
		}
		from = make([]int, len(g.ctx))
	}
	for {
		sys := newSystem(c)
		sub := sys.fresh(g.tparams, nil, syntax.Pos{})
		for _, p := range g.tparams {
			v := sub[p].T
			for _, b := range v.Param().Bounds {
				sys.sub(v, sub.ApplyIn(b))
			}
		}
		for i := range f.params {
			sys.sub(f.params[i], sub.ApplyIn(g.params[i]))
		}
		if f.recv != nil {
			sys.sub(*f.recv, sub.ApplyIn(*g.recv))
		}
		for k, j := range from {
			sys.sub(f.ctx[j], sub.ApplyIn(g.ctx[k]))
		}
		if sys.ok {
			return certainly
		}
		// The next way to pair them.
		k := 0
		for ; k < len(from); k++ {
			if from[k]++; from[k] < len(f.ctx) {
				break
			}
			from[k] = 0
		}
		if k == len(from) {
			return certainlyNot
		}
	}
}
