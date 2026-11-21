// A call in a builder's lambda whose variable nothing gives a type yet,
// createDerivativeTypeVariable's PNT, is inferred with the builder's
// variables, as in K2's PCLA, and a member of a value of their types fixes
// them on demand by K2's rules.
class TypeVariableOwner<T> {
    fun constrain(subtypeValue: T) {}
    fun provide(): T = null!!
    fun <PNT> createDerivativeTypeVariable(): TypeVariableOwner<PNT> = TypeVariableOwner()
    fun provideBox(): Box<T> = Box()
}

class Box<BXT> {
    fun unbox(): BXT = null!!
}

fun <OT> pcla(lambda: (TypeVariableOwner<OT>) -> Unit): OT = null!!

interface BaseType

class ScopeOwner : BaseType {
    fun function() {}
}

object Interloper : BaseType

// ScopeOwner <: PNT <: OT: OT is fixed to ScopeOwner through PNT, below it.
fun throughLower() {
    pcla { otvOwner ->
        val pntvOwner = otvOwner.createDerivativeTypeVariable()
        otvOwner.constrain(pntvOwner.provide())
        pntvOwner.constrain(ScopeOwner())
        otvOwner.provide().function()
        otvOwner.constrain(Interloper) // ERROR type mismatch: inferred type is Interloper but ScopeOwner was expected
    }
}

// PNT <: OT only: PNT can't be fixed for a member (KT-72030), and stays a
// variable that Interloper constrains.
fun onlyAbove() {
    pcla { otvOwner ->
        val pntvOwner = otvOwner.createDerivativeTypeVariable()
        otvOwner.constrain(pntvOwner.provide())
        otvOwner.constrain(ScopeOwner())
        pntvOwner.provide().function() // ERROR unresolved reference: function
        otvOwner.constrain(Interloper)
        pntvOwner.constrain(Interloper)
    }
}

// Box<PNT> <: OT: OT is semi-fixed to Box<PNT>.
fun semiFixed() {
    pcla { otvOwner ->
        val pntvOwner = otvOwner.createDerivativeTypeVariable()
        otvOwner.constrain(pntvOwner.provideBox())
        pntvOwner.constrain(ScopeOwner())
        otvOwner.provide().unbox().function()
        otvOwner.constrain(Interloper) // ERROR type mismatch: inferred type is Interloper but Box<ScopeOwner> was expected
    }
}
