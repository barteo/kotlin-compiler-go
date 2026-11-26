// A contract can't say a context parameter is called in place: a context
// argument is never a lambda.
@file:OptIn(ExperimentalContracts::class)

import kotlin.contracts.*

context(block: () -> R)
fun <R> myRun(): R {
    contract { // ERROR callsInPlace contract cannot be applied to context parameter 'block'
        callsInPlace(block, InvocationKind.EXACTLY_ONCE)
    }
    return block()
}

fun <R> run2(block: () -> R): R {
    contract {
        callsInPlace(block, InvocationKind.EXACTLY_ONCE)
    }
    return block()
}
