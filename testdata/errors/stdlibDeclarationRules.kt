// Rules about declarations and their annotations that need the standard
// library: its annotations' arguments, contracts, inline suspend
// parameters, @JvmExposeBoxed and @IntroducedAt.
@file:OptIn(kotlin.contracts.ExperimentalContracts::class, ExperimentalStdlibApi::class, ExperimentalVersionOverloading::class)

import kotlin.contracts.*
import kotlin.reflect.KProperty

@Deprecated class Old // ERROR no value passed for parameter 'message'
class OldProp(@property:Deprecated val p: Int) // ERROR no value passed for parameter 'message'
class OldGetter { @get:Deprecated val g = 1 } // ERROR no value passed for parameter 'message'
@Deprecated("with message") class Fine

annotation class Names(vararg val s: String)
annotation class Arr(val a: Array<String>)
@Names(s = "x") fun single() {} // ERROR assigning single elements to varargs in named form is prohibited
@Arr(a = "x") fun singleArray() {} // ERROR argument type mismatch
@Names("x", "y") fun positional() {}
@Names(s = ["x"]) fun literal() {}
@Names(s = *arrayOf("x")) fun spread() {}
@Deprecated("x", ReplaceWith("y", imports = "a")) fun nested() {} // ERROR assigning single elements to varargs in named form is prohibited

fun inContract(block: () -> Unit) {
    contract {
        @Suppress("WRONG_INVOCATION_KIND") // ERROR annotations are prohibited inside contracts
        callsInPlace(block, InvocationKind.AT_MOST_ONCE)
    }
}
fun outsideContract(block: () -> Unit) {
    @Suppress("WRONG_INVOCATION_KIND")
    contract { callsInPlace(block, InvocationKind.AT_MOST_ONCE) }
}

inline fun plainInline(s: suspend () -> String = { "" }) {} // ERROR must be 'noinline' or 'crossinline'
suspend inline fun suspendDefault(s: suspend () -> String = { "" }) {} // ERROR suspend functional parameters with default values
suspend inline fun crossDefault(crossinline s: suspend () -> String = { "" }) {} // ERROR suspend functional parameters with default values
suspend inline fun noinlineDefault(noinline s: suspend () -> String = { "" }) {}
suspend inline fun plainDefault(s: () -> String = { "" }) {}
val stored: suspend () -> String = { "" }
inline fun badDefault(s: () -> String = stored2) {} // ERROR invalid default value for an inline functional parameter
val stored2: () -> String = { "" }

@JvmInline
value class IC(val s: String)
@JvmExposeBoxed private fun hidden(ic: IC) {} // ERROR '@JvmExposeBoxed' cannot be applied to a private declaration
@JvmExposeBoxed fun shown(ic: IC) {}
private class Hidden {
    @JvmExposeBoxed fun inHidden(ic: IC) {} // ERROR '@JvmExposeBoxed' cannot be applied to a private declaration
}
class Shown {
    @get:JvmExposeBoxed("getIC") val ic: IC get() = IC("")
    @set:JvmExposeBoxed var ic2: IC = IC("") // ERROR '@JvmExposeBoxed' cannot be applied to a private declaration
        private set
}

class Versioned {
    var x: Int = 0
        set(@IntroducedAt("1") value) { field = value } // ERROR '@IntroducedAt' can only be used on optional parameters
}

class NotOperator { fun getValue(t: Any?, p: KProperty<*>) = 1 }
class Delegating { val a: Int by NotOperator() } // ERROR 'operator' modifier is required on 'getValue'
fun localDelegate() { val b: Int by NotOperator() } // ERROR 'operator' modifier is required on 'getValue'
class ValueOf
val ValueOf.getValue: (Any?, KProperty<*>) -> Int get() = { _, _ -> 1 }
class ByProperty { val c: Int by ValueOf() } // ERROR property 'getValue' cannot be used as operator
