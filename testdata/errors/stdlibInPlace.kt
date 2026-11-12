// A lambda that a callsInPlace contract, of any kind, says the function
// calls in place doesn't run after it returns.
@file:OptIn(kotlin.contracts.ExperimentalContracts::class)

import kotlin.contracts.*

fun inPlace(f: () -> Unit) {
    contract { callsInPlace(f) }
    f()
}

fun noContract(f: () -> Unit) = f()

fun readInPlace(arg: Int?) {
    var x = arg
    if (x == null) return
    inPlace { x.inc() }
    x = 42
}

fun nestedInPlace(arg: Int?) {
    var x = arg
    if (x == null) return
    inPlace { inPlace { x.inc() } }
    x = 42
}

fun inPlaceInEscaping(arg: Int?) {
    var x = arg
    if (x == null) return
    noContract { inPlace { x.inc() } } // ERROR only safe (?.) or non-null asserted
    x = 42
}

fun readInForEach(arg: String?) {
    var x = arg
    if (x == null) return
    listOf(1).forEach { x.length }
    x = null
}

fun readThenAssignInRun(arg: Int?) {
    var x = arg
    if (x == null) return
    run {
        x.inc() // run calls it once: x is checked where it starts
        x = null
    }
    if (x != null) x = 42
    x.inc() // ERROR only safe (?.) or non-null asserted
}
