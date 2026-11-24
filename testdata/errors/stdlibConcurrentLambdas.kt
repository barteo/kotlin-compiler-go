// Definite assignment as K2 has it: the lambdas of one call that its
// contract calls exactly once may run in either order, and dead code is
// checked too.
import kotlin.contracts.*

@OptIn(ExperimentalContracts::class)
inline fun <T> run2(x: () -> T, y: () -> T) {
    contract {
        callsInPlace(x, InvocationKind.EXACTLY_ONCE)
        callsInPlace(y, InvocationKind.EXACTLY_ONCE)
    }
    x()
    y()
}

fun bothAssign(p: Boolean) {
    val x: Int
    run2(
        {
            if (p) {
                x = 1 // ERROR captured values cannot be initialized because of possible reassignments
            } else {
                x = 2 // ERROR captured values cannot be initialized because of possible reassignments
            }
        },
        { x = 3 }, // ERROR captured values cannot be initialized because of possible reassignments
    )
    x.inc()
}

fun oneAssigns() {
    val x: Int
    run2({ x = 1 }, {})
    run2({}, { x = 2 }) // ERROR val cannot be reassigned
    x.inc()
}

fun otherReads() {
    val x: String
    run2({ x = ""; x.length }, { x.length }) // ERROR variable 'x' must be initialized
    x.length
}

fun deadRead() {
    val x: Int
    return
    println(x) // ERROR variable 'x' must be initialized
}

fun deadAfterBranches(c: Boolean) {
    val x: Int
    if (c) {
        x = 1
        return
    } else {
        x = 2
        return
    }
    println(x)
}

fun deadAfterLoop() {
    val x: Int
    x = 1
    while (true) {
    }
    println(x)
}

fun deadReassignment() {
    val x: Int
    x = 1
    TODO()
    x = 2 // ERROR val cannot be reassigned
}

class NoPrimary {
    val once: Int
    val twice: Int

    constructor() {
        once = 1
        twice = 2
    }

    constructor(s: String) : this() {
        twice = 3 // ERROR val cannot be reassigned
        println(once)
    }
}
