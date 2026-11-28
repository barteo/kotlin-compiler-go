package check

import "github.com/barteo/kotlin-compiler-go/internal/syntax"

// jvmDefaultModeAnnotations checks the annotations that change how a
// class's interfaces' bodies compile against the -jvm-default mode, as
// K2 does: @JvmDefaultWithCompatibility is only allowed in the
// no-compatibility mode, JVM_DEFAULT_WITH_COMPATIBILITY_NOT_IN_NO_COMPATIBILITY_MODE,
// and @JvmDefaultWithoutCompatibility only in enable,
// JVM_DEFAULT_WITHOUT_COMPATIBILITY_NOT_IN_ENABLE_MODE.
func (c *Checker) jvmDefaultModeAnnotations() {
	noCompat := !c.u.noJVMDefault && !c.u.jvmDefaultCompat
	enable := c.u.jvmDefaultCompat
	var class func(d *syntax.ClassDecl)
	class = func(d *syntax.ClassDecl) {
		if a := d.Find("", "JvmDefaultWithCompatibility"); a != nil && !noCompat {
			c.errorf(a.P, "JVM_DEFAULT_WITH_COMPATIBILITY_NOT_IN_NO_COMPATIBILITY_MODE", "this annotation is only allowed with -jvm-default=no-compatibility")
		}
		if a := d.Find("", "JvmDefaultWithoutCompatibility"); a != nil && !enable {
			c.errorf(a.P, "JVM_DEFAULT_WITHOUT_COMPATIBILITY_NOT_IN_ENABLE_MODE", "this annotation is only allowed with -jvm-default=enable")
		}
		for _, m := range d.Members {
			if n, ok := m.(*syntax.ClassDecl); ok {
				class(n)
			}
		}
	}
	for _, d := range c.file.Decls {
		if cd, ok := d.(*syntax.ClassDecl); ok {
			class(cd)
		}
	}
}
