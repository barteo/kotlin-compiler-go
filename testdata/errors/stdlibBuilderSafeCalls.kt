// A safe call of an inner class's constructor or of a property's invoke,
// checked as a let, fixes a builder's variable in its receiver's type for
// the member, as K2's scope navigation does.
class TypeVariableOwner<T> {
    fun constrain(subtypeValue: T) {}
    fun provide(): T = null!!
    val instance: T = null!!
}

fun <OT> pcla(lambda: (TypeVariableOwner<OT>) -> Unit): OT = null!!
fun <OT> nullablePcla(lambda: (TypeVariableOwner<OT>?) -> Unit): OT = null!!

object Value

class ScopeOwner {
    operator fun invoke(value: Value) {}
    inner class Inner
}

object Interloper

fun safeInnerConstructor() {
    pcla { otvOwner ->
        otvOwner.constrain(ScopeOwner())
        otvOwner.provide()?.Inner()
        otvOwner.constrain(Interloper) // ERROR type mismatch: inferred type is Interloper but ScopeOwner was expected
    }
}

fun safeInvoke() {
    nullablePcla { otvOwner ->
        otvOwner?.constrain(ScopeOwner())
        otvOwner?.instance(Value)
        otvOwner?.constrain(Interloper) // ERROR type mismatch: inferred type is Interloper but ScopeOwner was expected
    }
}
