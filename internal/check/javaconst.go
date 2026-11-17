package check

import "github.com/barteo/kotlin-compiler-go/internal/types"

// javaNonConst reports whether v, a static final field, is one of a Java
// class's whose initializer isn't a constant expression: a field of a
// primitive type or String without a ConstantValue, which a class file
// would have, and which kc reads from a Java source's initializer. A
// Java source's field that reads a Kotlin class's const val may still be
// constant: kc doesn't know its value.
func javaNonConst(v *types.VarSym) bool {
	return v.Owner != nil && isJavaClass(v.Owner) && v.Constant == nil && !v.MaybeConst && constType(v.Type) && v.Type.K != types.Null && v.Type.K != types.Nothing
}
