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
	if cls.Annotation || cls.AnnotationParams != nil || len(cls.Ctors) > 0 || !isJavaClass(cls) {
		return
	}
	ps := c.annotationParams(cls)
	for i := range a.Args {
		arg := &a.Args[i]
		if arg.Name != "" {
			break
		}
		if len(ps) > 0 && ps[0].Name == "value" && (i == 0 || ps[0].Vararg) {
			continue
		}
		p := a.P
		if len(arg.Toks) > 0 {
			p = arg.Toks[0].Pos
		}
		c.errorf(p, "only named arguments are available for Java annotations")
	}
}
