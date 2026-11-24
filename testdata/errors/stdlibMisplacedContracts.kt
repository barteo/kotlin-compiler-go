@file:OptIn(ExperimentalContracts::class)

// A contract must be the first statement of a block body of a function
// that isn't local (CONTRACT_NOT_ALLOWED).
import kotlin.contracts.*

fun ok(y: Boolean) {
    contract { returns() implies y }
}

fun notFirst(y: Boolean) {
    val x: Int = 42
    contract { returns() implies y } // ERROR contract should be the first statement
}

fun exprBody(block: () -> Unit) = contract { callsInPlace(block, InvocationKind.EXACTLY_ONCE) } // ERROR contracts are only allowed in function body blocks

fun local(cond: Boolean): Boolean {
    fun innerFun() {
        contract { returns() implies cond } // ERROR contracts are not allowed for local functions
    }
    class Local {
        fun member(b: () -> Unit) {
            contract { callsInPlace(b, InvocationKind.EXACTLY_ONCE) } // ERROR contracts are not allowed for local functions
            b()
        }
    }
    return true
}

fun inLambda(cond: Boolean): Boolean {
    run {
        contract { returns(true) implies cond } // ERROR contract should be the first statement
    }
    return true
}
