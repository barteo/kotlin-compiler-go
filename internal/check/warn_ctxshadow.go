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
			s, ok := signatureMatch(a, b, false)
			if !ok {
				continue
			}
			bctx := make([]types.Type, len(b.ctx))
			for k, t := range b.ctx {
				bctx[k] = s.Apply(t)
			}
			bShadowsA, ok1 := a.c.ctxCovers(bctx, a.ctx)
			aShadowsB, ok2 := a.c.ctxCovers(a.ctx, bctx)
			if !ok1 || !ok2 || bShadowsA == aShadowsB {
				continue
			}
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

// ctxCovers reports whether context parameters of types ctx shadow those
// of types other: each of ctx has one of other that is at least as
// specific. ok is false where that isn't certain, of a type parameter or
// a type that doesn't resolve.
func (c *Checker) ctxCovers(ctx, other []types.Type) (covers, ok bool) {
	uncertain := func(t types.Type) bool {
		return t.K == types.Invalid || t.P || hasInvalid(t, 0) || t.Has(func(t types.Type) bool { return t.IsParam() || t.K == types.TypeVar })
	}
	if slices.ContainsFunc(ctx, uncertain) || slices.ContainsFunc(other, uncertain) {
		return false, false
	}
	for _, t := range ctx {
		if !slices.ContainsFunc(other, func(u types.Type) bool { return c.subtype(u, t) }) {
			return false, true
		}
	}
	return true, true
}
