// An enum's companion object read in its constructor code, in a lambda
// that a contract says is called in place, is uninitialized.
@file:OptIn(ExperimentalContracts::class)

import kotlin.contracts.ExperimentalContracts
import kotlin.contracts.contract

enum class E {
    A {
        val ab = inPlaceRun { foo() } // ERROR companion object of enum class 'E' is uninitialized here
        val ac = nonInPlaceRun { foo() }
    };

    val a = foo() // ERROR companion object of enum class 'E' is uninitialized here
    val b = inPlaceRun { foo() } // ERROR companion object of enum class 'E' is uninitialized here
    val c = nonInPlaceRun { foo() }

    companion object {
        fun foo(): String = "foo()"
    }
}

inline fun <T> inPlaceRun(block: () -> T): T {
    contract { callsInPlace(block) }
    return block()
}

fun <T> nonInPlaceRun(block: () -> T): T = block()
