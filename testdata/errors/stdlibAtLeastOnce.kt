// What a lambda that a contract says is called at least once assigns is
// assigned after the call.
@file:OptIn(ExperimentalContracts::class)

import kotlin.contracts.*

fun <T> runTwice(block: () -> T): T {
    contract {
        callsInPlace(block, InvocationKind.AT_LEAST_ONCE)
    }
    block()
    return block()
}

fun <T> runMaybe(block: () -> T): T {
    contract {
        callsInPlace(block, InvocationKind.AT_MOST_ONCE)
    }
    return block()
}

fun f() {
    var x: Int
    x.inc() // ERROR variable 'x' must be initialized
    runTwice { x = 42 }
    x.inc()
    var y: Int
    runMaybe { y = 42 }
    y.inc() // ERROR variable 'y' must be initialized
}
