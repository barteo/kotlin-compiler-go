package check

import "github.com/barteo/kotlin-compiler-go/internal/types"

// jdkHiddenMembers are the members of JDK classes mapped to Kotlin
// classes that K2 hides (JvmBuiltInsSignatures' hidden signatures), by
// class and name+descriptor: an extension of the same name is called.
var jdkHiddenMembers = map[string]map[string]bool{
	"java/lang/Enum": {"describeConstable()Ljava/util/Optional;": true},
}

// hideJDK hides f, a member of the JDK class named cls, if K2 does.
func hideJDK(cls string, f *types.FunSym) {
	if jdkHiddenMembers[cls][f.Name+f.Desc] {
		f.Hidden = true
	}
}
