package check

import (
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// valueClass checks the declaration of a value class, with kotlinc's
// messages, before its members are resolved.
func (c *Checker) valueClass(d *syntax.ClassDecl) {
	switch {
	case d.Interface:
		c.errorf(d.P, "modifier 'value' is not applicable to 'interface'")
		return
	case d.Object:
		c.errorf(d.P, "modifier 'value' is not applicable to 'object'")
		return
	case !d.JvmInline:
		c.errorf(d.P, "value classes without '@JvmInline' annotation are not yet supported")
	}
	if d.Inline {
		for _, a := range d.Annotations {
			if a.Target == "" && a.Is("JvmInline") && !a.Implicit {
				c.errorf(a.P, "'@JvmInline' annotation is applicable only to value classes")
			}
		}
	}
	if d.Open || d.Abstract || d.Sealed {
		c.errorf(d.P, "value class can be only final")
	}
	if d.Data || d.Enum || d.Annotation {
		c.errorf(d.P, "modifier 'value' is incompatible with data, enum and annotation classes")
	}
	for _, st := range d.Supers {
		if st.Delegate != nil && delegateProp(d, st.Delegate) == nil {
			c.errorf(st.Delegate.Pos(), "value class cannot implement an interface by delegation if expression is not a parameter")
		}
	}
	for _, m := range d.Members {
		switch m := m.(type) {
		case *syntax.PropDecl:
			if m.Recv == nil && (m.Init != nil || m.Getter == nil && !m.Abstract) {
				c.errorf(m.P, "value class cannot have properties with backing fields")
			}
		}
	}
}

// valueParam checks the one parameter of a value class's primary
// constructor, and makes its property the class's underlying value.
func (c *Checker) valueParam(d *syntax.ClassDecl) {
	cls := d.Sym
	if len(d.CtorParams) != 1 {
		c.errorf(d.P, "value class must have exactly one primary constructor parameter")
		cls.Value = false
		return
	}
	cp := d.CtorParams[0]
	if !cp.IsProp || cp.Mutable {
		c.errorf(cp.P, "value class primary constructor must only have final read-only ('val') property parameters")
	}
	if cls.Super != nil && cls.Super.Name != "java/lang/Object" {
		c.errorf(d.P, "value class cannot extend classes")
	}
	t := cp.Sym.Type
	switch {
	case t.K == types.Invalid:
		cls.Value = false
	case t.K == types.Unit || t.K == types.Nothing:
		c.errorf(cp.Type.P, "value class cannot have value parameter of type 'kotlin/%s'", t.NonNull())
		cls.Value = false
	case t.K == types.Class && t.C == cls:
		c.errorf(cp.Type.P, "value class cannot be recursive")
		cls.Value = false
	case cp.PropSym != nil:
		cls.Underlying = cp.PropSym
	}
}
