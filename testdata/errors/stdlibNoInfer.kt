// An argument of a @NoInfer parameter, an integer literal too, and the
// expected type of a call whose result type has @NoInfer in it, constrain
// nothing: they are checked against what the rest infers.

class In<in T>

@Suppress("INVISIBLE_MEMBER", "INVISIBLE_REFERENCE")
fun <E> id(e: E): In<@kotlin.internal.NoInfer E> = TODO()

@Suppress("INVISIBLE_MEMBER", "INVISIBLE_REFERENCE")
fun <T> assertEquals1(e1: T, e2: @kotlin.internal.NoInfer T): Boolean = true

fun test5(x: In<String>) {}

fun usage(y: Int, s: String) {
    val x: In<String> = id(y) // ERROR type mismatch
    test5(id(y)) // ERROR type mismatch
    assertEquals1(s, 11) // ERROR type mismatch
    assertEquals1(s, "")
}
