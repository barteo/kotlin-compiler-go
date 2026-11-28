package check

import (
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// ambiguousAnon reports declaration d, of class cls or the top level,
// whose inferred type t has an object expression's type with more than
// one supertype, if code outside it sees the type, which would then be
// ambiguous: kotlinc's AMBIGUOUS_ANONYMOUS_TYPE_INFERRED. A getter is
// reported with its property's visibility.
func (c *Checker) ambiguousAnon(d syntax.Decl, cls *types.ClassSym, t types.Type) {
	if cls != nil && (cls.Local || cls.Anonymous) {
		return
	}
	var m *syntax.Modifiers
	var at syntax.Pos
	switch d := d.(type) {
	case *syntax.FunDecl:
		m, at = &d.Modifiers, d.P
		if d.AccessorOf != nil {
			pd, ok := d.AccessorOf.Decl.(*syntax.PropDecl)
			if !ok {
				return
			}
			m = &pd.Modifiers
		}
	case *syntax.PropDecl:
		if d.DelegateOf != nil {
			return // its property, whose type is the delegate's value's, is
		}
		m, at = &d.Modifiers, d.P
	default:
		return
	}
	if m.Visibility == syntax.Private && !m.Inline {
		return
	}
	if t.Has(ambiguousAnonType) {
		c.errorf(at, "AMBIGUOUS_ANONYMOUS_TYPE_INFERRED", "right-hand side has anonymous type. Please specify type explicitly")
	}
}

// ambiguousAnonType reports whether t is an object expression's type
// with more than one supertype but Any.
func ambiguousAnonType(t types.Type) bool {
	if t.K != types.Class || !t.C.Anonymous {
		return false
	}
	n := 0
	for _, s := range t.C.Supers {
		if s.K == types.Class {
			n++
		}
	}
	return n > 1
}
