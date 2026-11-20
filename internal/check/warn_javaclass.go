package check

import (
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// javaClassOnCompanion reports kotlinc's JAVA_CLASS_ON_COMPANION
// (FirJavaClassOnCompanionChecker) on x, if it is A.javaClass for a
// class A named for its companion object: the standard library's
// javaClass of the companion, Class<A.Companion>, not A's class.
func (c *Checker) javaClassOnCompanion(x *syntax.Member) {
	if x.Name != "javaClass" || x.Getter == nil || x.Getter.Facade != "kotlin/jvm/JvmClassMappingKt" {
		return
	}
	var name string
	var obj *types.ClassSym
	switch r := x.X.(type) {
	case *syntax.Ident:
		name, obj = r.Name, r.Object
	case *syntax.Member:
		name, obj = r.Name, r.Object
	}
	if obj == nil {
		return
	}
	var class string
	switch {
	case obj.CompanionOf != nil:
		// Named for the companion itself, A.Companion, it is the object.
		if name == obj.Name[strings.LastIndexAny(obj.Name, "/$")+1:] {
			return
		}
		class = obj.CompanionOf.DisplayName()
		if n := len(obj.CompanionOf.TypeParams); n > 0 {
			class += "<" + strings.Repeat("*, ", n-1) + "*>"
		}
	case builtinCompanions[obj.Name]:
		// Int.javaClass, of IntCompanionObject: the built-in types'
		// companions are only named for their classes.
		for k, jc := range companionJVM {
			if jc == obj.Name {
				class = strings.TrimSuffix(strings.TrimPrefix(k, "kotlin/"), ".Companion")
			}
		}
		if name != class {
			return
		}
	default:
		return
	}
	c.warnf(x.P, "JAVA_CLASS_ON_COMPANION", "the resulting type of this 'javaClass' call is '%s' and not 'Class<%s>'; use '::class.java' to avoid confusion", x.Type(), class)
}
