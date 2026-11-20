package check

import (
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// This file reports kotlinc's NO_REFLECTION_IN_CLASS_PATH: a use of a
// member of a kotlin.reflect type that needs kotlin-reflect, which isn't
// on the class path. At run time such a call throws
// KotlinReflectionNotSupportedError. Members the standard library
// implements itself are allowed: a KClass's simpleName, qualifiedName and
// isInstance, a property's get and set, any name, invoke, equals,
// hashCode and toString, and the members of the types of KType. This
// follows kotlinc's AbstractFirReflectionApiCallChecker, which judges a
// member by its receiver's type.

// reflectionAllowedClasses are the kotlin.reflect classes whose members
// need no kotlin-reflect.
var reflectionAllowedClasses = map[string]bool{
	"kotlin/reflect/KType": true, "kotlin/reflect/KTypeParameter": true, "kotlin/reflect/KTypeProjection": true,
	"kotlin/reflect/KTypeProjection$Companion": true, "kotlin/reflect/KVariance": true,
}

var reflectionPropertyClasses = map[string]bool{
	"kotlin/reflect/KProperty0": true, "kotlin/reflect/KProperty1": true, "kotlin/reflect/KProperty2": true,
	"kotlin/reflect/KMutableProperty0": true, "kotlin/reflect/KMutableProperty1": true, "kotlin/reflect/KMutableProperty2": true,
}

// reflectionWarning reports member access n, a member property's read or
// the callee of a member function's call x (nil for a read), if it needs
// kotlin-reflect, which isn't on the class path.
func (c *Checker) reflectionWarning(n *syntax.Member, x *syntax.Call) {
	if n.X == nil || n.Static != nil || n.Object != nil || !c.u.noReflection {
		return
	}
	name := n.Name
	switch {
	case x != nil:
		if x.Sym == nil || x.Sym.Ext || x.Sym.Static || x.Invoke {
			return
		}
		name = x.Sym.Name
	case n.Getter != nil:
		if n.Getter.Ext || n.Getter.Static {
			return
		}
	case n.Prop != nil:
		if n.Prop.Kind != types.Prop {
			return
		}
	default:
		return
	}
	switch name {
	case "equals", "hashCode", "toString", "invoke", "name":
		return
	}
	for _, cls := range c.boundClasses(n.X.Type().NonNull(), nil, 0) {
		if !reflectionClass(cls.Name) || reflectionAllowed(cls.Name, name) {
			continue
		}
		c.warnf(n.P, "NO_REFLECTION_IN_CLASS_PATH", "Call uses reflection API which is not found in compilation classpath. Make sure you have kotlin-reflect.jar in the classpath.")
		return
	}
}

// reflectionClass reports whether class name is of the package
// kotlin.reflect.
func reflectionClass(name string) bool {
	rest, ok := strings.CutPrefix(name, "kotlin/reflect/")
	return ok && !strings.Contains(rest, "/")
}

// reflectionAllowed reports whether member name of class cls, of
// kotlin.reflect, needs no kotlin-reflect.
func reflectionAllowed(cls, name string) bool {
	switch {
	case reflectionAllowedClasses[cls]:
		return true
	case cls == "kotlin/reflect/KClass":
		return name == "simpleName" || name == "qualifiedName" || name == "isInstance"
	case reflectionPropertyClasses[cls]:
		return name == "get" || name == "set"
	}
	return false
}
