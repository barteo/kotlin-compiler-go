package check

import "github.com/barteo/kotlin-compiler-go/internal/syntax"

// javaRedeclaration reports a class of the program, d with the JVM name
// name, that the program's Java sources declare too, as K2 does
// (CLASSIFIER_REDECLARATION): a class Hello in Hello.kt and Hello.java, or
// a nested class of such a class that the Java class nests too.
func (c *Checker) javaRedeclaration(d *syntax.ClassDecl, name string) {
	if d.EnumEntry || hasExpectActual(&d.Modifiers) {
		return
	}
	if cp := c.u.ext.cp; cp != nil && cp.IsSource(name) {
		c.errorf(d.P, "redeclaration: class %s is also declared in the Java sources", d.Name)
	}
}
