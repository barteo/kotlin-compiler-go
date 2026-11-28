package check

import (
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// appliedAnnotation is an annotation written on an element, with its
// class.
type appliedAnnotation struct {
	a   *syntax.Annotation
	cls *types.ClassSym
}

// repeatedWithContainer reports a repeatable annotation written more than
// once on an element that its container annotation is written on too
// (REPEATED_ANNOTATION_WITH_CONTAINER): the class file couldn't hold both
// containers. Source-retained annotations aren't written, so they may.
func (c *Checker) repeatedWithContainer(applied []appliedAnnotation) {
	type key struct {
		cls    *types.ClassSym
		target string
	}
	count := map[key]int{}
	for _, x := range applied {
		count[key{x.cls, x.a.Target}]++
	}
	seen := map[key]bool{}
	for _, x := range applied {
		k := key{x.cls, x.a.Target}
		if count[k] < 2 || !x.cls.Repeatable || x.cls.Retention == types.RetentionSource {
			continue
		}
		if !seen[k] {
			seen[k] = true
			continue
		}
		for _, y := range applied {
			if y.a.Target == x.a.Target && isContainerOf(y.cls, x.cls) {
				c.errorf(x.a.P, "REPEATED_ANNOTATION_WITH_CONTAINER", "repeated annotation cannot be used with its container annotation")
				break
			}
		}
	}
}

// isContainerOf reports whether annotation class k is the container of
// repeatable annotation class cls: the one its @Repeatable names, or,
// for the program's, which kc doesn't record, one whose value is an
// array of cls.
func isContainerOf(k, cls *types.ClassSym) bool {
	if k == cls {
		return false
	}
	if cls.Container != nil {
		return k == cls.Container
	}
	if cls.External {
		return false
	}
	for _, p := range k.AnnotationParams {
		if p.Name == "value" && p.Type.IsArray() {
			e := types.ArrayElem(p.Type)
			return e.K == types.Class && e.C == cls
		}
	}
	return false
}
