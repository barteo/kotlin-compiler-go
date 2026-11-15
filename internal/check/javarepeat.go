package check

import (
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// javaRepeatChecked reports whether annotation a of Java annotation class
// cls, whose targets kc doesn't know, is checked for repeating all the
// same: its @Repeatable is known, and where it goes isn't in question
// without a use-site target on anything but a property.
func javaRepeatChecked(a *syntax.Annotation, cls *types.ClassSym, site annotSite) bool {
	return isJavaClass(cls) && a.Target == "" && site.kind != siteProp && site.kind != siteCtorProp
}
