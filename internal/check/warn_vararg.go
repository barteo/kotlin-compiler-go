package check

import (
	"slices"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// This file reports kotlinc's REDUNDANT_SPREAD_OPERATOR_IN_NAMED_FORM_IN_FUNCTION
// and _IN_ANNOTATION (FirNamedVarargChecker): a named argument of a
// vararg parameter passes an array without the spread operator, so
// xs = *arr is xs = arr.

// redundantSpreads reports the named arguments of resolved call x that
// spread an array into a vararg parameter.
func (c *Checker) redundantSpreads(x *syntax.Call) {
	if x.Sym == nil || x.Names == nil {
		return
	}
	for i, a := range x.Args {
		if i >= len(x.Names) || x.Names[i] == "" {
			continue
		}
		s, ok := a.(*syntax.Spread)
		if !ok {
			continue
		}
		// kotlinc doesn't check a call whose argument it rejects: *a of
		// a nullable or mistyped array.
		if t := s.X.Type(); t.K == types.Invalid || types.MayBeNull(t) {
			continue
		}
		if p := paramNamed(x.Sym.Params, x.Names[i]); p != nil && p.Vararg {
			c.warnf(s.P, "REDUNDANT_SPREAD_OPERATOR_IN_NAMED_FORM_IN_FUNCTION", "redundant spread (*) operator")
		}
	}
}

// annotationSpreads reports the named arguments args of an annotation of
// class cls that spread an array into a vararg parameter, and those of
// the calls in their values.
func (c *Checker) annotationSpreads(cls *types.ClassSym, args []annotArg, raw []syntax.AnnotationArg) {
	for i, arg := range args {
		if arg.name != "" && i < len(raw) && len(raw[i].Toks) > 0 && raw[i].Toks[0].Kind == syntax.STAR {
			if p := paramNamed(c.annotationParams(cls), arg.name); p != nil && p.Vararg {
				c.warnf(raw[i].Toks[0].Pos, "REDUNDANT_SPREAD_OPERATOR_IN_NAMED_FORM_IN_ANNOTATION", "redundant spread (*) operator")
			}
		}
		for _, e := range arg.elems {
			syntax.Inspect(e, func(n any) bool {
				call, ok := n.(*syntax.Call)
				switch {
				case !ok:
				case call.Sym == nil && syntax.IsArrayOf(call):
					// An array literal, which isn't resolved as a call:
					// intArrayOf(elements = *a) of the vararg elements.
					for i, a := range call.Args {
						if s, ok := a.(*syntax.Spread); ok && i < len(call.Names) && call.Names[i] == "elements" {
							c.warnf(s.P, "REDUNDANT_SPREAD_OPERATOR_IN_NAMED_FORM_IN_FUNCTION", "redundant spread (*) operator")
						}
					}
				default:
					c.redundantSpreads(call)
				}
				return true
			})
		}
	}
}

// paramNamed returns the parameter of ps named name, or nil.
func paramNamed(ps []*types.VarSym, name string) *types.VarSym {
	if i := slices.IndexFunc(ps, func(p *types.VarSym) bool { return p.Name == name }); i >= 0 {
		return ps[i]
	}
	return nil
}
