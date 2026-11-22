// As in K2, x[i]++ on a value of a builder's variable's type doesn't fix
// the variable, as a member call on it does: its get and its inc aren't
// found, and the constraints after it are the variable's still.

class TypeVariableOwner<T> {
    fun constrain(subtypeValue: T) {}
    val immutableReference: T = null!!
}

interface BaseType

object Index

class ScopeOwner : BaseType {
    operator fun inc(): ScopeOwner = this
    operator fun get(index: Index): ScopeOwner = this
    operator fun set(index: Index, value: ScopeOwner) {}
}

object Interloper : BaseType

fun <OT> pcla(lambda: (TypeVariableOwner<OT>) -> Unit): OT = null!!

fun test() {
    val postfix: BaseType = pcla { otvOwner ->
        otvOwner.constrain(ScopeOwner())
        otvOwner.immutableReference[Index]++ // ERROR unresolved reference
        otvOwner.constrain(Interloper)
    }
    val prefix: BaseType = pcla { otvOwner ->
        otvOwner.constrain(ScopeOwner())
        ++otvOwner.immutableReference[Index] // ERROR unresolved reference
        otvOwner.constrain(Interloper)
    }
}
