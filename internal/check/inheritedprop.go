package check

import (
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// inheritedPropImpl reports the property of supertype s, p, implemented
// in class cls, declared by d, by q, a concrete property cls inherits
// from another supertype, if q can't: a val can't implement a var, and
// q's type must be a subtype of p's.
func (c *Checker) inheritedPropImpl(d *syntax.ClassDecl, cls, s *types.ClassSym, p, q *types.VarSym) {
	if q == nil || q.Abstract || q.Owner == nil || q.Owner == cls || q.Owner.IsSubclassOf(s) {
		return
	}
	this := cls.This()
	pt, qt := c.memberSubst(this, s).Apply(p.Type), c.memberSubst(this, q.Owner).Apply(q.Type)
	if pt.K == types.Invalid || qt.K == types.Invalid {
		return
	}
	switch {
	case p.Mutable && !q.Mutable:
		c.errorf(d.P, "var '%s' of %s is implemented by an inherited val of %s", p.Name, s.Name, q.Owner.Name)
	case p.Mutable && pt != qt && !(c.subtype(pt, qt) && c.subtype(qt, pt)):
		c.errorf(d.P, "the types of the inherited var '%s' of %s and %s don't match: %s and %s", p.Name, s.Name, q.Owner.Name, pt, qt)
	case !c.subtype(qt, pt):
		c.errorf(d.P, "the types of the inherited property '%s' of %s and %s don't match: %s isn't a subtype of %s", p.Name, s.Name, q.Owner.Name, qt, pt)
	}
}
