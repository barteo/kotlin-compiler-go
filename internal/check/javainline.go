package check

import "github.com/barteo/kotlin-compiler-go/internal/types"

// javaProtected reports whether a protected member of owner, read
// without its declaration's modifiers, is one a public-API inline
// function can't use: as in K2, a Java class's protected members, static
// ones too, are protected to Kotlin wherever the class comes from.
func javaProtected(owner *types.ClassSym) bool {
	return isJavaClass(owner)
}
