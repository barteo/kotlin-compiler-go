// Of the syntax that K2 fixes a builder's variable on demand for, a bare
// type in an is or as check takes its arguments from the variable's value,
// and an increment or decrement fixes nothing (KT-71778).
class TypeVariableOwner<T> {
    fun constrain(subtypeValue: T) {}
    fun provide(): T = null!!
    val immutableReference: T = null!!
}

fun <OT> pcla(lambda: (TypeVariableOwner<OT>) -> Unit): OT = null!!

interface BaseType

open class GenericScopeOwner<A> : BaseType
open class GenericScopeOwnerSubtype<B> : GenericScopeOwner<B>()

class TypeArgument

object Index

class ScopeOwner : BaseType {
    operator fun inc(): ScopeOwner = this
    operator fun get(index: Index): ScopeOwner = this
    operator fun set(index: Index, value: ScopeOwner) {}
}

object Interloper : BaseType

fun bareType() {
    pcla { otvOwner ->
        otvOwner.constrain(GenericScopeOwner<TypeArgument>())
        otvOwner.provide() is GenericScopeOwnerSubtype
        otvOwner.constrain(Interloper) // ERROR type mismatch: inferred type is Interloper but GenericScopeOwner<TypeArgument> was expected
    }
}

fun increment() {
    pcla { otvOwner ->
        otvOwner.constrain(ScopeOwner())
        otvOwner.immutableReference[Index]++ // ERROR receiver of type OT
        otvOwner.constrain(Interloper)
    }
}
