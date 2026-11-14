// As K2 fixes a builder's variable on demand, a member of a value of its
// type fixes it to what the lambda gave it so far, and what the lambda does
// after is checked with that type: the mismatch is reported where it is,
// not as a variable that can't be inferred.

class TypeVariableOwner<T> {
    fun constrain(subtypeValue: T) {}
    fun provide(): T = null!!
}

fun <OT> pcla(lambda: (TypeVariableOwner<OT>) -> Unit): OT = null!!

interface BaseType
class ScopeOwner : BaseType {
    fun memberFunction() {}
}
object Interloper : BaseType

fun fixedByMember() {
    val r = pcla { owner ->
        owner.constrain(ScopeOwner())
        owner.provide().memberFunction()
        owner.constrain(Interloper) // ERROR type mismatch: inferred type is Interloper but ScopeOwner was expected
    }
}

fun fixedByMemberFine(): ScopeOwner = pcla { owner ->
    owner.constrain(ScopeOwner())
    owner.provide().memberFunction()
}
