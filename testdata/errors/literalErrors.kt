// kotlinc's errors for code its grammar accepts, which kc's parser goes on
// after.
val a = 1l // ERROR use 'L' instead of 'l'
val b = 0x1l // ERROR use 'L' instead of 'l'
val c = 00 // ERROR unsupported leading zeros
val d = 0_6 // ERROR unsupported leading zeros
val e = 002L // ERROR unsupported leading zeros
val ok = 0 + 0xF + 0L + 0b0
val okDouble = 000.555

fun f(x: Int, b: Boolean): Int {
    when (b) {
        b && b -> {} // ERROR confusing branch condition
        (b || b) -> {}
        else -> {}
    }
    return when (x) {
        x in 1..2 -> 1 // ERROR confusing branch condition
        in x..x -> 2
        x + 1 -> 3
        else -> 4
    }
}

fun g(x: Int) = when {
    x == 1 -> 1
    else -> 2
}
