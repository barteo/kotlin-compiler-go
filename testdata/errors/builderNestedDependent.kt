// A nested builder's variable that depends on an outer builder's is fixed
// on demand with it, as in K2's one constraint system for nested builders:
// fixing either fixes the one it depends on first.
class Anchor<AT>

class TypeVariableOwner<T> {
    fun constrain(subtypeValue: T) {}
    fun provide(): T = null!!
    fun provideAnchor(): Anchor<T> = null!!
}

fun <FOT> pcla(lambda: (TypeVariableOwner<FOT>) -> Any?): FOT = null!!

interface BaseType

class ScopeOwner : BaseType {
    fun function() {}
}

object Interloper : BaseType

fun <SOT> shallowPcla(anchor: SOT, lambda: (TypeVariableOwner<SOT>) -> Any?): SOT = null!!
fun <SOT> contravariantPcla(anchor: Anchor<in SOT>, lambda: (TypeVariableOwner<SOT>) -> Any?): SOT = null!!

// ScopeOwner <: FOT <: SOT: SOT's member fixes FOT, then SOT.
fun innerFixesOuter() {
    pcla { fotvOwner ->
        shallowPcla(fotvOwner.provide()) { sotvOwner ->
            fotvOwner.constrain(ScopeOwner())
            sotvOwner.provide().function()
            fotvOwner.constrain(Interloper) // ERROR type mismatch: inferred type is Interloper but ScopeOwner was expected
            sotvOwner.constrain(Interloper) // ERROR type mismatch: inferred type is Interloper but ScopeOwner was expected
        }
    }
}

// ScopeOwner <: SOT <: FOT: FOT's member sees the inner builder's bound.
fun outerFixedThroughInner() {
    pcla { fotvOwner ->
        contravariantPcla(fotvOwner.provideAnchor()) { sotvOwner ->
            sotvOwner.constrain(ScopeOwner())
            fotvOwner.provide().function()
            fotvOwner.constrain(Interloper) // ERROR type mismatch: inferred type is Interloper but ScopeOwner was expected
            sotvOwner.constrain(Interloper) // ERROR type mismatch: inferred type is Interloper but ScopeOwner was expected
        }
    }
}
