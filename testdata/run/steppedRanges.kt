// Stepped for loops that end at the progression's last element, without
// overflowing at the type's bounds, and fail on a step that isn't
// positive; in checks that evaluate both bounds, in order, before the
// value; and floating-point equality as IEEE 754 says, nullable too.

var order = ""

fun low(i: Int): Int {
    order += "L"
    return i
}

fun high(i: Int): Int {
    order += "H"
    return i
}

fun x(i: Int): Int {
    order += "X"
    return i
}

fun stepped(step: Int) {
    try {
        for (i in 1..3 step step) print("$i ")
        println()
    } catch (e: IllegalArgumentException) {
        println(e.message)
    }
}

fun eq(a: Double?, b: Double?) = a == b
fun eqF(a: Float, b: Float?) = a == b
fun <T : Double> eqT(a: T, b: Double) = a == b

const val MAX = 2147483647
const val MIN = -2147483647 - 1
const val LMAX = 9223372036854775807L

fun main() {
    for (i in MAX - 5..MAX step 3) print("$i ")
    println()
    for (i in MIN + 5 downTo MIN step 3) print("$i ")
    println()
    for (i in MAX..MAX step 1) print("$i ")
    println()
    for (i in 0 until MIN step 2) print("$i ")
    for (i in 1 until 10 step 4) print("$i ")
    println()
    for (i in LMAX - 4..LMAX step 2L) print("$i ")
    println()
    for (i in 10L downTo 1L step 4L) print("$i ")
    println()
    for (i in -7..7 step 5) print("$i ")
    println()
    for (i in 7 downTo -7 step 5) print("$i ")
    println()
    stepped(2)
    stepped(0)
    stepped(-1)

    println(x(0) in low(1)..high(3))
    println(order)
    order = ""
    println(0 in low(1)..high(3))
    println(order)
    order = ""
    println(x(5) !in high(9) downTo low(4))
    println(order)
    order = ""
    println(x(2) in low(1) until high(3))
    println(order)

    println(eq(0.0, -0.0))
    println(eq(null, -0.0))
    println(eq(null, null))
    println(eq(0.0 / 0.0, 0.0 / 0.0))
    println(eqF(0.0f, -0.0f))
    println(eqT(-0.0, 0.0))
}
