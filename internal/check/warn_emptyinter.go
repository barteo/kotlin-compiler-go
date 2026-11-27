package check

import (
	"slices"
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// emptyInter is a type argument a call infers that is an intersection
// that is or may be empty.
type emptyInter struct {
	tp       *types.TypeParamSym
	comps    []types.Type // the intersection's components
	possible bool         // only may be empty: a final class and an interface it doesn't implement
}

// noteEmptyIntersection records call x, once its system s is solved, if
// one of the type parameters tps it infers, through sub, is an
// intersection of types no value can be (see emptyIntersection), or may
// not be (see mayBeEmptyIntersection), as K2's
// checkInferredEmptyIntersection finds it when it fixes the variable:
// an intersection kc infers, or one of a variable's upper constraints and
// its declared bounds. The last check of x decides.
func (c *Checker) noteEmptyIntersection(x *syntax.Call, tps []*types.TypeParamSym, sub types.Subst, s *system) {
	for _, tp := range tps {
		t := s.final(sub.Apply(types.ParamType(tp)))
		tv := s.varOf(sub[tp].T)
		if t.K == types.Invalid || tv == nil || !ownUpper(tv) {
			// Without a constraint of its own, a variable is a captured
			// star in K2, not the intersection of its bounds.
			continue
		}
		comps := []types.Type{t}
		if t.IsIntersection() {
			comps = t.Param().Bounds
		}
		if s.onlyUpper(tv) {
			// Fixed from its upper constraints alone, a variable is the
			// intersection of them with its declared bounds, as in K2:
			// kc's fixing leaves out the bounds its value isn't below.
			var bounds []types.Type
			for _, b := range tp.Bounds {
				if b = s.final(sub.Apply(b)); b.K != types.Invalid && !b.HasVars() && !c.subtype(t, b) && !slices.Contains(comps, b) {
					bounds = append(bounds, b)
				}
			}
			comps = append(bounds, comps...) // as kotlinc lists them
		}
		if len(comps) < 2 {
			continue
		}
		var e *emptyInter
		for i, a := range comps {
			for _, b := range comps[i+1:] {
				if a.HasVars() || b.HasVars() || a.K == types.Invalid || b.K == types.Invalid {
					continue
				}
				switch {
				case c.emptyIntersection(a, b):
					e = &emptyInter{tp: tp, comps: comps}
				case e == nil && c.mayBeEmptyIntersection(a, b):
					e = &emptyInter{tp: tp, comps: comps, possible: true}
				}
			}
		}
		if e != nil {
			if c.emptyInters == nil {
				c.emptyInters = map[*syntax.Call]emptyInter{}
			}
			c.emptyInters[x] = *e
			return
		}
	}
	delete(c.emptyInters, x)
}

// noteUnitIntersection records call x, the last of a lambda that a bare
// return@label leaves too, whose type parameter tp K2 infers as the
// intersection of Unit and its declared bound, which kc infers as
// bound alone (see bareTail): as noteEmptyIntersection, if no value can
// be both, or may not be.
func (c *Checker) noteUnitIntersection(x *syntax.Call, tp *types.TypeParamSym, bound types.Type) {
	if bound.K == types.Invalid || bound.K == types.Nothing || bound.HasVars() {
		return
	}
	comps := []types.Type{bound, types.UnitType}
	// Unit is a final object that implements no interface: a class
	// bound can't be it, an interface one may not be.
	var e *emptyInter
	for _, b := range c.boundClasses(bound, nil, 0) {
		if !b.Interface {
			e = &emptyInter{tp: tp, comps: comps}
			break
		}
		if e == nil {
			e = &emptyInter{tp: tp, comps: comps, possible: true}
		}
	}
	if e == nil {
		return
	}
	if c.unitInters == nil {
		c.unitInters = map[*syntax.Call]emptyInter{}
	}
	c.unitInters[x] = *e
}

// emptyIntersectionWarningAt warns at p of call x if
// noteEmptyIntersection recorded it: INFERRED_TYPE_VARIABLE_INTO_EMPTY_INTERSECTION_WARNING, an
// error in a later language version, or, if the intersection only may
// be empty, INFERRED_TYPE_VARIABLE_INTO_POSSIBLE_EMPTY_INTERSECTION.
func (c *Checker) emptyIntersectionWarningAt(x *syntax.Call, p syntax.Pos) {
	e, ok := c.emptyInters[x]
	if !ok {
		// The call's own system finds no empty intersection: the one
		// a bare return adds Unit to (see noteUnitIntersection).
		if e, ok = c.unitInters[x]; !ok {
			return
		}
	}
	var names []string
	for _, t := range e.comps {
		names = append(names, t.String())
	}
	if e.possible {
		c.warnf(p, "INFERRED_TYPE_VARIABLE_INTO_POSSIBLE_EMPTY_INTERSECTION", "type argument for type parameter '%s' has possible incompatible upper bounds: %s (final class and interface)", e.tp.Name, strings.Join(names, ", "))
		return
	}
	c.warnf(p, "INFERRED_TYPE_VARIABLE_INTO_EMPTY_INTERSECTION_WARNING", "type argument for type parameter '%s' cannot be inferred because it has incompatible upper bounds: %s (multiple incompatible classes). This will become an error in a future release", e.tp.Name, strings.Join(names, ", "))
}

// varOf returns the variable of system s that t is, or nil.
func (s *system) varOf(t types.Type) *tvar {
	if t.K != types.TypeVar {
		return nil
	}
	return s.byVar[t.Param()]
}

// onlyUpper reports whether variable tv of solved system s had only upper
// constraints: no lower one but Nothing, no integer literal, and it
// stands for no captured projection.
func (s *system) onlyUpper(tv *tvar) bool {
	if tv.star || tv.proj || tv.synth || len(tv.lits) > 0 || len(tv.upper) == 0 {
		return false
	}
	for _, l := range tv.lower {
		if s.final(l).K != types.Nothing {
			return false
		}
	}
	return true
}

// ownUpper reports whether variable tv has an upper constraint that
// isn't a declared bound, its own or another variable's.
func ownUpper(tv *tvar) bool {
	for _, u := range tv.upper {
		if !slices.Contains(tv.declUp, u) && !slices.Contains(tv.boundUp, u) {
			return true
		}
	}
	return false
}

// delegateAt returns the position kotlinc reports call x at if it is a
// delegated property's operator call, x$delegate.getValue(this, ref),
// which isn't written: its delegate's; p otherwise.
func delegateAt(x *syntax.Call, p syntax.Pos) syntax.Pos {
	m, ok := x.Fun.(*syntax.Member)
	if !ok || !x.Operator {
		return p
	}
	var v *types.VarSym
	switch f := m.X.(type) {
	case *syntax.Ident:
		v = f.Sym
	case *syntax.Member:
		v = f.Prop
	}
	if v == nil {
		return p
	}
	if d, ok := v.Decl.(*syntax.PropDecl); ok && d.DelegateOf != nil && d.Init != nil {
		return syntax.StartPos(d.Init)
	}
	return p
}
