package check

import (
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// javaPositionalArgs reports the positional arguments of annotation a of
// Java annotation class cls but those for its value method: Java names
// an annotation's other elements, and Kotlin does too, as it knows no
// order for them (POSITIONED_VALUE_ARGUMENT_FOR_JAVA_ANNOTATION).
func (c *Checker) javaPositionalArgs(a *syntax.Annotation, cls *types.ClassSym) {
	if !javaAnnotation(cls) || !c.u.ext.kotlin {
		return // without the standard library, @Deprecated is Java's
	}
	ps := c.annotationParams(cls)
	named := false
	for i := range a.Args {
		arg := &a.Args[i]
		if arg.Name != "" {
			named = true
			continue
		}
		p := a.P
		if len(arg.Toks) > 0 {
			p = arg.Toks[0].Pos
		}
		switch {
		case named:
			// A Java annotation's parameters have no order for a
			// positional argument after a named one to follow.
			c.errorf(p, "mixing named and positional arguments is not allowed")
		case len(ps) > 0 && ps[0].Name == "value" && (i == 0 || ps[0].Vararg):
		default:
			c.errorf(p, "only named arguments are available for Java annotations")
		}
	}
}

// javaAnnotation reports whether cls is a Java annotation class, whose
// parameters are its methods.
func javaAnnotation(cls *types.ClassSym) bool {
	return !cls.Annotation && cls.AnnotationParams == nil && len(cls.Ctors) == 0 && isJavaClass(cls)
}
