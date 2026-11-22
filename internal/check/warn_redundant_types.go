package check

import (
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// This file reports three of kotlinc's extra checks of what is written
// to no effect: REDUNDANT_RETURN_UNIT_TYPE, REDUNDANT_SETTER_PARAMETER_TYPE
// and REDUNDANT_SINGLE_EXPRESSION_STRING_TEMPLATE.

// redundantDeclTypes reports, of declaration d, a function with a block
// body, or with none, whose return type is written as Unit, and a
// setter's parameter with a type, which is its property's.
func (c *Checker) redundantDeclTypes(d syntax.Decl) {
	switch d := d.(type) {
	case *syntax.FunDecl:
		// An annotated type isn't reported, nor an accessor's.
		if d.Generated || d.Ret == nil || d.ExprBody != nil || d.Sym == nil || d.AccessorOf != nil || len(d.Ret.Annotations) > 0 {
			return
		}
		if t := d.Sym.Ret; t.K == types.Unit && !t.N && !t.P {
			c.warnf(d.Ret.P, "REDUNDANT_RETURN_UNIT_TYPE", "redundant 'Unit' return type")
		}
	case *syntax.PropDecl:
		if s := d.Setter; s != nil && !d.Generated && len(s.Params) == 1 && s.Params[0].Type != nil {
			c.warnf(s.Params[0].Type.P, "REDUNDANT_SETTER_PARAMETER_TYPE", "redundant setter parameter type")
		}
	}
}

// redundantTemplate reports string template x if it is one expression of
// type String and nothing else, "$s", which is the expression.
func (c *Checker) redundantTemplate(x *syntax.StringLit) {
	if len(x.Parts) != 1 || x.Parts[0].X == nil {
		return
	}
	if t := x.Parts[0].X.Type(); t == types.StringType {
		c.warnf(x.Parts[0].X.Pos(), "REDUNDANT_SINGLE_EXPRESSION_STRING_TEMPLATE", "redundant string template")
	}
}
