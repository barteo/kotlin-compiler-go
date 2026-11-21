// Of the syntax that K2 fixes a builder's variable on demand for, a bare
// type in an is or as check takes its arguments from the variable's value,
// an increment or decrement fixes nothing (KT-71778), and a smart cast of a
// value of the variable's type is an intersection with it, whose members
// fix it.
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

interface IntersectionArgument

open class OpenScopeOwner : BaseType {
    fun function() {}
}

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

fun smartCastIntersection() {
    pcla { otvOwner ->
        otvOwner.constrain(OpenScopeOwner())
        val otvValue = otvOwner.provide()
        if (otvValue is IntersectionArgument) {
            otvValue.function()
        }
        otvOwner.constrain(Interloper) // ERROR type mismatch: inferred type is Interloper but OpenScopeOwner was expected
    }
}
