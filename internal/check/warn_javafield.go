package check

import (
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// javaFieldWarnings reports PROPERTY_HIDES_JAVA_FIELD, as kotlinc: a
// property of Kotlin class cls named as an instance field of a Java
// superclass, which a Kotlin property hides wherever both are visible.
// Kotlin classes in between don't matter, even ones that declare the
// property too. A private property hides no package-private field of
// another package.
func (c *Checker) javaFieldWarnings(cls *types.ClassSym, p *types.VarSym, pos syntax.Pos) {
	var f *types.VarSym
	for s := cls.Super; s != nil && f == nil; s = s.Super {
		if !s.External || s.FromKotlin {
			continue // a Kotlin class: its JavaFields are JvmField properties
		}
		if q := s.JavaFields[p.Name]; q != nil && (!q.PackagePrivate || !p.Private || packageOf(s) == packageOf(cls)) {
			f = q
		}
	}
	if f != nil {
		c.warnf(pos, "PROPERTY_HIDES_JAVA_FIELD", "This property hides Java field 'field %s: %v' thus making it inaccessible.", f.Name, f.Type)
	}
}
