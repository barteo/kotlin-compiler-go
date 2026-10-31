package types

// Retention is when an annotation class's annotations are kept, as
// kotlin.annotation.AnnotationRetention: Runtime ones are visible to
// reflection, Binary ones are in the class file only, and Source ones
// aren't written.
type Retention uint8

const (
	NoRetention      Retention = iota // not an annotation class
	RetentionSource                   // SOURCE
	RetentionBinary                   // BINARY (Java's CLASS)
	RetentionRuntime                  // RUNTIME
)

// Targets is a set of kotlin.annotation.AnnotationTarget values: the kinds
// of declarations an annotation class applies to.
type Targets uint32

// AnnotationTarget's entries, in its order.
const (
	TargetClass Targets = 1 << iota
	TargetAnnotationClass
	TargetTypeParameter
	TargetProperty
	TargetField
	TargetLocalVariable
	TargetValueParameter
	TargetConstructor
	TargetFunction
	TargetGetter
	TargetSetter
	TargetType
	TargetExpression
	TargetFile
	TargetTypealias
)

// targetNames are AnnotationTarget's entries' names, in order.
var targetNames = []string{"CLASS", "ANNOTATION_CLASS", "TYPE_PARAMETER", "PROPERTY", "FIELD", "LOCAL_VARIABLE",
	"VALUE_PARAMETER", "CONSTRUCTOR", "FUNCTION", "PROPERTY_GETTER", "PROPERTY_SETTER", "TYPE", "EXPRESSION", "FILE", "TYPEALIAS"}

// DefaultTargets are the targets of a Kotlin annotation class without
// @Target.
const DefaultTargets = TargetClass | TargetProperty | TargetField | TargetLocalVariable | TargetValueParameter |
	TargetConstructor | TargetFunction | TargetGetter | TargetSetter

// KotlinTargets returns the set of AnnotationTarget entries named.
func KotlinTargets(names []string) Targets {
	var t Targets
	for _, n := range names {
		for i, tn := range targetNames {
			if n == tn {
				t |= 1 << i
			}
		}
	}
	return t
}

// JavaTargets returns the Kotlin targets of java.lang.annotation.ElementType
// entries, as Kotlin applies Java annotations; all declarations for none.
func JavaTargets(names []string) Targets {
	if names == nil {
		return DefaultTargets | TargetAnnotationClass | TargetTypeParameter
	}
	var t Targets
	for _, n := range names {
		switch n {
		case "TYPE":
			t |= TargetClass | TargetAnnotationClass
		case "ANNOTATION_TYPE":
			t |= TargetAnnotationClass
		case "FIELD":
			t |= TargetField
		case "METHOD":
			t |= TargetFunction | TargetGetter | TargetSetter
		case "PARAMETER":
			t |= TargetValueParameter
		case "CONSTRUCTOR":
			t |= TargetConstructor
		case "LOCAL_VARIABLE":
			t |= TargetLocalVariable
		case "TYPE_PARAMETER":
			t |= TargetTypeParameter
		case "TYPE_USE":
			t |= TargetType
		}
	}
	return t
}

// RetentionOf returns the retention named by an AnnotationRetention or
// java.lang.annotation.RetentionPolicy entry.
func RetentionOf(name string) Retention {
	switch name {
	case "SOURCE":
		return RetentionSource
	case "BINARY", "CLASS":
		return RetentionBinary
	case "RUNTIME":
		return RetentionRuntime
	}
	return NoRetention
}
