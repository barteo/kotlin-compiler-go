class Box(val value: Int)

fun f(x: Int): String {
    val s: String = x // ERROR inferred type is Int but String was expected
    val n: Int = null // ERROR inferred type is Nothing? but Int was expected
    val l: Long = 5
    val i: Int = l // ERROR inferred type is Long but Int was expected
    val d: Double = 1 // ERROR inferred type is Int but Double was expected
    println(x == l) // ERROR operator '==' cannot be applied to Int and Long
    println(undefined) // ERROR unresolved reference: undefined
    println(missing(1)) // ERROR unresolved reference: missing
    val b = Box(1)
    println(b.nope) // ERROR unresolved reference: nope
    b.value = 2 // ERROR val cannot be reassigned
    x = 3 // ERROR val cannot be reassigned
    println("a" - 1) // ERROR operator '-' cannot be applied to String and Int
    return s
}

fun g(): Int { // ERROR missing return statement
    println("no return")
}

fun h(flag: Boolean): Int = if (flag) 1 // ERROR 'if' must have both main and 'else' branches

fun k(n: Int): String = when (n) { // ERROR must be exhaustive
    1 -> "one"
}

fun loopOnly() {
    break // ERROR 'break' is not allowed outside a loop
}

fun calls() {
    f() // ERROR no function 'f' takes 0 arguments
    f("x") // ERROR inferred type is String but Int was expected
    Box(1, 2) // ERROR no function '<init>' takes 2 arguments
}

val r = 1..10 // ERROR ranges are only supported

fun identities(x: Int, l: Long, s: String, b: Box) {
    println(x === l) // ERROR operator '===' cannot be applied to Int and Long
    println(s !== b) // ERROR operator '===' cannot be applied to String and Box
}
