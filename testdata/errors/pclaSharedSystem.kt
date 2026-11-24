// K2 analyzes a builder's lambda once, on a constraint system that the
// builder's call shares with the calls in the lambda (PCLA). kc checks
// the lambda by the same rules.

class TypeVariableOwner<T> {
    fun constrain(subtypeValue: T) {}
    fun provide(): T = null!!
    fun provideAnchor(): Anchor<T> = null!!
    fun provideContainer(): ContravariantContainer<T> = null!!
    fun <PNT> createDerivativeTypeVariable(anchor: ContravariantContainer<PNT>): TypeVariableOwner<PNT> = null!!
}

class Anchor<AT>
class ContravariantContainer<in CT>

fun <OT> pcla(lambda: (TypeVariableOwner<OT>) -> Unit): OT = null!!
fun <FOT> outerPCLA(lambda: (TypeVariableOwner<FOT>) -> Any?): FOT = null!!

interface BaseType
class ScopeOwner : BaseType {
    fun function() {}
}
object Interloper : BaseType

open class GenericScopeOwner<A> : BaseType
class GenericScopeOwnerSubtype<B> : GenericScopeOwner<B>()
class TypeArgument

fun <PH> PH.fix() {}
fun <MT> materialize(): MT = null!!

// A member fixes its receiver's variable to what its constraints give
// there: a later constraint that disagrees is the error, not the member.
fun fixedOnDemand() {
    pcla { owner ->
        owner.constrain(ScopeOwner())
        owner.provide().function()
        owner.constrain(Interloper) // ERROR type mismatch: inferred type is Interloper but ScopeOwner was expected
    }
}

// A variable that only another variable bounds isn't fixed: it has no
// members, though the statements after give it a type. The lambda's
// error doesn't keep its variables from being inferred.
fun notFixable() {
    val result = pcla { owner ->
        // PNT <: OT
        val derivative = owner.createDerivativeTypeVariable(owner.provideContainer())
        owner.constrain(ScopeOwner())
        derivative.provide().function() // ERROR unresolved reference: function
        owner.constrain(Interloper)
    }
    val base: BaseType = result
}

// One that a variable with a type bounds from below is: ScopeOwner <:
// PNT <: OT fixes OT.
fun fixedThroughDerivative() {
    pcla { owner ->
        val derivative = owner.createDerivativeTypeVariable(owner.provideContainer())
        derivative.constrain(ScopeOwner())
        owner.provide().function()
        owner.constrain(Interloper) // ERROR type mismatch: inferred type is Interloper but ScopeOwner was expected
    }
}

// A builder in a builder's lambda shares the system: fixing the inner
// one's variable fixes the outer one's that is below it.
fun nested() {
    fun <SOT> nestedPCLA(anchor: SOT, lambda: (TypeVariableOwner<SOT>) -> Any?): SOT = null!!
    outerPCLA { outer ->
        nestedPCLA(outer.provide()) { inner ->
            outer.constrain(ScopeOwner())
            inner.provide().function()
            outer.constrain(Interloper) // ERROR type mismatch: inferred type is Interloper but ScopeOwner was expected
            inner.constrain(Interloper) // ERROR type mismatch: inferred type is Interloper but ScopeOwner was expected
        }
    }
}

// An extension that takes any receiver isn't called on a variable that
// nothing fixes, nor on one that only another call's variable bounds.
fun stubReceiver() {
    pcla { owner ->
        owner.provide().fix() // ERROR the type of a receiver hasn't been inferred yet
        owner.constrain(Interloper)
    }
    pcla { owner ->
        owner.constrain(materialize())
        owner.provide().fix() // ERROR the type of a receiver hasn't been inferred yet
        owner.constrain(Interloper)
    }
}

// A type written without its arguments in an is fixes the variable too,
// for the arguments; one with them doesn't.
fun bareType() {
    pcla { owner ->
        owner.constrain(GenericScopeOwner<TypeArgument>())
        owner.provide() is GenericScopeOwnerSubtype
        owner.constrain(Interloper) // ERROR type mismatch: inferred type is Interloper but GenericScopeOwner<TypeArgument> was expected
    }
    pcla { owner ->
        owner.constrain(GenericScopeOwner<TypeArgument>())
        owner.provide() is GenericScopeOwnerSubtype<*>
        owner.constrain(Interloper)
    }
}
