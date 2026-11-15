package check

import (
	"slices"

	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// recordComponent reports whether name is a component of Java record
// cls: as in K2, Kotlin reads it as a property, r.x, of accessor x().
func recordComponent(cls *types.ClassSym, name string) bool {
	return isJavaClass(cls) && slices.Contains(cls.RecordComponents, name)
}
