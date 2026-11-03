package check

import (
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// outerPropLabel returns, for ::p in an inner class, where p, read as e,
// is a property of an outer class whose instance the code has, that
// class's name, which this@Outer::p, the reference ::p is, names; or "".
func (c *Checker) outerPropLabel(e *syntax.Ident) string {
	p := e.Sym
	if m := e.Member; m != nil {
		if _, ok := m.X.(*syntax.This); !ok {
			return ""
		}
		p = m.Prop
	}
	if p == nil || p.Kind != types.Prop || p.Owner == nil || c.cls == nil || c.cls.IsSubclassOf(p.Owner) {
		return ""
	}
	for k := c.cls; k.Inner && k.Outer != nil; k = k.Outer {
		if o := k.Outer; o.IsSubclassOf(p.Owner) {
			if d, ok := o.Decl.(*syntax.ClassDecl); ok && d.Name != "" {
				return d.Name
			}
			return ""
		}
	}
	return ""
}
