// A val declared without a value and assigned in a lambda that a function
// which isn't inline calls exactly once, as its contract says: the lambda
// shares it, as it does a captured var.
@file:OptIn(kotlin.contracts.ExperimentalContracts::class)

import kotlin.contracts.*

fun runOnce(action: () -> Unit) {
    contract { callsInPlace(action, InvocationKind.EXACTLY_ONCE) }
    action()
}

fun <T> callOnce(f: () -> T): T {
    contract { callsInPlace(f, InvocationKind.EXACTLY_ONCE) }
    return f()
}

fun strings(b: Boolean): String {
    val res: String
    runOnce {
        b
        res = if (b) "yes" else "no"
    }
    return res
}

fun wide(): Long {
    val l: Long
    val d: Double
    runOnce { l = 1L shl 40 }
    runOnce { d = 2.5 }
    return l + d.toLong()
}

fun nested(): Int {
    val x: Int
    runOnce {
        runOnce { x = 42 }
    }
    return x
}

fun inWhen(k: Int): String {
    val s: String
    when (k) {
        0 -> runOnce { s = "zero" }
        else -> runOnce { s = "other $k" }
    }
    return s
}

fun captured(): Int {
    val v: Int
    runOnce { v = 7 }
    val read = { v * 2 } // read later by another lambda
    return callOnce { read() + v }
}

fun main() {
    println(strings(true))
    println(strings(false))
    println(wide())
    println(nested())
    println(inWhen(0))
    println(inWhen(3))
    println(captured())
}
