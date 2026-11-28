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
		c.errorf(d.P, "WRONG_MODIFIER_TARGET", "modifier 'value' is not applicable to 'interface'")
		return
	case d.Object:
		c.errorf(d.P, "WRONG_MODIFIER_TARGET", "modifier 'value' is not applicable to 'object'")
		return
	case d.Local:
		c.errorf(valueModPos(d), "VALUE_CLASS_NOT_TOP_LEVEL", "value class cannot be local or inner")
		c.errorf(valueModPos(d), "WRONG_MODIFIER_TARGET", "modifier '%s' is not applicable to 'local class'", valueModName(d))
	case !d.JvmInline:
		c.errorf(d.P, "VALUE_CLASS_WITHOUT_JVM_INLINE_ANNOTATION", "value classes without '@JvmInline' annotation are not yet supported")
	}
	if d.Inner {
		c.errorf(valueModPos(d), "VALUE_CLASS_NOT_TOP_LEVEL", "value class cannot be local or inner")
	}
	if d.Inline {
		at := d.P
		for _, m := range d.Written {
			if m.Name == "inline" {
				at = m.P
			}
		}
		c.warnf(at, "INLINE_CLASS_DEPRECATED", "'inline' modifier is deprecated. Use 'value' instead.")
		for _, a := range d.Annotations {
			if a.Target == "" && a.Is("JvmInline") && !a.Implicit {
				c.errorf(a.P, "JVM_INLINE_WITHOUT_VALUE_CLASS", "'@JvmInline' annotation is applicable only to value classes")
			}
		}
	}
	if d.Open || d.Abstract || d.Sealed {
		c.errorf(d.P, "VALUE_CLASS_NOT_FINAL", "value class can be only final")
	}
	if d.Data || d.Enum || d.Annotation {
		c.errorf(d.P, "WRONG_MODIFIER_TARGET", "modifier 'value' is incompatible with data, enum and annotation classes")
	}
	for _, st := range d.Supers {
		if st.Delegate != nil && delegateProp(d, st.Delegate) == nil {
			c.errorf(st.Delegate.Pos(), "VALUE_CLASS_CANNOT_IMPLEMENT_INTERFACE_BY_DELEGATION", "value class cannot implement an interface by delegation if expression is not a parameter")
		}
	}
	for _, m := range d.Members {
		switch m := m.(type) {
		case *syntax.PropDecl:
			if m.Recv == nil && (m.Init != nil || m.Getter == nil && !m.Abstract) {
				c.errorf(m.P, "PROPERTY_WITH_BACKING_FIELD_INSIDE_VALUE_CLASS", "value class cannot have properties with backing fields")
			}
		}
	}
}

// valueParam checks the one parameter of a value class's primary
// constructor, and makes its property the class's underlying value.
func (c *Checker) valueParam(d *syntax.ClassDecl) {
	cls := d.Sym
	if len(d.CtorParams) != 1 {
		name := "INLINE_CLASS_CONSTRUCTOR_WRONG_PARAMETERS_SIZE"
		if !d.HasPrimary {
			name = "ABSENCE_OF_PRIMARY_CONSTRUCTOR_FOR_VALUE_CLASS"
		}
		c.errorf(d.P, name, "value class must have exactly one primary constructor parameter")
		cls.Value = false
		return
	}
	cp := d.CtorParams[0]
	if !cp.IsProp || cp.Mutable || cp.Open {
		c.errorf(cp.P, "VALUE_CLASS_CONSTRUCTOR_NOT_FINAL_READ_ONLY_PARAMETER", "value class primary constructor must only have final read-only ('val') property parameters")
	}
	if cls.Super != nil && cls.Super.Name != "java/lang/Object" {
		c.errorf(d.P, "VALUE_CLASS_CANNOT_EXTEND_CLASSES", "value class cannot extend classes")
	}
	if c.isSubclassOf(cls, "java/lang/Cloneable") {
		c.errorf(valueModPos(d), "VALUE_CLASS_CANNOT_BE_CLONEABLE", "value class cannot be 'Cloneable'")
	}
	t := cp.Sym.Type
	switch {
	case t.K == types.Invalid:
		cls.Value = false
	case t.K == types.Unit || t.K == types.Nothing:
		c.errorf(cp.Type.P, "VALUE_CLASS_HAS_INAPPLICABLE_PARAMETER_TYPE", "value class cannot have value parameter of type 'kotlin/%s'", t.NonNull())
		cls.Value = false
	case t.K == types.Class && t.C == cls:
		c.errorf(cp.Type.P, "VALUE_CLASS_CANNOT_BE_RECURSIVE", "value class cannot be recursive")
		cls.Value = false
	case cp.PropSym != nil:
		cls.Underlying = cp.PropSym
		if types.RecursiveValue(cls) { // through other value classes
			// The other classes of the cycle, checked before cls, are
			// recursive too: kotlinc reports each.
			for o := cls.Underlying.Type.C; o != cls && o != nil && o.Underlying != nil; {
				next := o.Underlying.Type.C
				if od, ok := o.Decl.(*syntax.ClassDecl); ok && len(od.CtorParams) == 1 && od.CtorParams[0].Type != nil {
					c.errorf(od.CtorParams[0].Type.P, "VALUE_CLASS_CANNOT_BE_RECURSIVE", "value class cannot be recursive")
				}
				o.Value, o.Underlying = false, nil
				o = next
			}
			c.errorf(cp.Type.P, "VALUE_CLASS_CANNOT_BE_RECURSIVE", "value class cannot be recursive")
			cls.Value, cls.Underlying = false, nil
		}
	}
}

// valueModPos returns the position of a value class's 'value' or 'inline'
// modifier, where kotlinc reports what is wrong with the class as a
// value class.
func valueModPos(d *syntax.ClassDecl) syntax.Pos {
	for _, m := range d.Written {
		if m.Name == "value" || m.Name == "inline" {
			return m.P
		}
	}
	return d.P
}

// valueModName returns the modifier that makes d a value class.
func valueModName(d *syntax.ClassDecl) string {
	for _, m := range d.Written {
		if m.Name == "value" || m.Name == "inline" {
			return m.Name
		}
	}
	return "value"
}

// valueReservedNames are the names of functions that value classes may
// get in future releases, which kotlinc 2.4 (without the language
// feature CustomEqualsInValueClasses) forbids them to declare or inherit
// with a body.
var valueReservedNames = []string{"box", "unbox", "equals", "hashCode"}

// reservedValueMembers reports the functions of a value class with a
// reserved name: its own, and those it inherits with a body from an
// interface, as K2's value class checker does.
func (c *Checker) reservedValueMembers(d *syntax.ClassDecl) {
	cls := d.Sym
	this := cls.This()
	for _, name := range valueReservedNames {
		for _, m := range d.Members {
			if fd, ok := m.(*syntax.FunDecl); ok && fd.Name == name && fd.Recv == nil && !fd.Abstract {
				c.errorf(fd.P, "RESERVED_MEMBER_INSIDE_VALUE_CLASS", "member name '%s' is reserved for future releases", name)
			}
		}
		cls.Supertypes(func(s *types.ClassSym) bool {
			if s == cls || !s.Interface {
				return true
			}
			for _, g := range s.Methods[name] {
				if g.Abstract || g.Static || g.Private || g.Ext {
					continue
				}
				if c.implementation(cls, this, g) == nil {
					c.errorf(d.P, "RESERVED_MEMBER_FROM_INTERFACE_INSIDE_VALUE_CLASS", "member name '%s' is reserved for future releases but is implemented in supertype '%s'", name, simpleName(s))
				}
			}
			return true
		})
	}
}
