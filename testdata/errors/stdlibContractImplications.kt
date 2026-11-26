// What a contract's implications don't tell through comparisons with its
// function's result, as with kotlinc: a Boolean? result not equal to
// true may be null, a safe call that gives null may not have run, and
// returns(true) says nothing of a result equal to false.
@file:OptIn(kotlin.contracts.ExperimentalContracts::class)

import kotlin.contracts.*

class A {
    fun returnsNull(x: Any?): Any? {
        contract { returns(null) implies (x is String) }
        return if (x is String) null else ""
    }
}

fun safeIsString(x: Any?): Boolean? {
    contract { returns(true) implies (x is String) }
    return x?.let { it is String }
}

fun test(a: A?, s: Any?) {
    if (safeIsString(s) != true) {
        s.length // ERROR nullable receiver of type Any?
    }
    if (safeIsString(s) == false) {
        s.length // ERROR nullable receiver of type Any?
    }
    when (safeIsString(s)) {
        true -> {}
        else -> s.length // ERROR nullable receiver of type Any?
    }
    if (a?.returnsNull(s) == null) {
        s.length // ERROR nullable receiver of type Any?
    }
}
