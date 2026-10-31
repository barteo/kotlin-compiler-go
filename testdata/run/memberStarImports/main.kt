package model

import model.Overflow.*
import model.Outcome.*
import model.Shape.*

fun describe(o: Overflow) = when (o) {
    SUSPEND -> "suspend"
    DROP_OLDEST -> "drop oldest"
    DROP_LATEST -> "drop latest"
}

internal fun outcome(i: Int): Outcome = when (i) {
    0 -> SUCCESSFUL
    1 -> REREGISTER
    else -> CANCELLED
}

fun main() {
    println(describe(SUSPEND) + ", " + describe(DROP_OLDEST) + ", " + describe(Overflow.DROP_LATEST))
    println(SUSPEND !== DROP_OLDEST)
    println(SUSPEND.ordinal)
    println(outcome(0).toString() + " " + outcome(1) + " " + outcome(2))
    println(Box(3).side)
    println(Kind.SQUARE)
    println(privateSuspend())
}
