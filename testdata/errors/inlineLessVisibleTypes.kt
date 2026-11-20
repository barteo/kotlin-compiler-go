// An inline function that isn't private, nor public API, can't use the
// types that are less visible than it, nor members whose signatures name
// them, as kotlinc's LESS_VISIBLE_TYPE_ACCESS_IN_INLINE_ERROR and
// LESS_VISIBLE_TYPE_IN_INLINE_ACCESSED_SIGNATURE_ERROR, or members of a
// private class, PRIVATE_CLASS_MEMBER_FROM_INLINE_ERROR.
@file:Suppress("NOTHING_TO_INLINE", "UNUSED_VARIABLE", "UNUSED_EXPRESSION", "USELESS_IS_CHECK", "EXPOSED_FUNCTION_RETURN_TYPE", "EXPOSED_RECEIVER_TYPE", "EXPOSED_PARAMETER_TYPE")

private class S { companion object { fun c() = 1 } }
private fun mk(): S = S()

internal inline fun r1(): S? = null // ERROR 'private-in-file' type 'S?' is accessed from 'internal' inline declaration
internal inline fun S.r2() {} // ERROR 'private-in-file' type 'S' is accessed from 'internal' inline declaration
internal inline fun r3(a: Any) {
    a is S // ERROR 'private-in-file' type 'S' is accessed from 'internal' inline declaration
    val f = { s: S -> } // ERROR 'private-in-file' type 'S' is accessed from 'internal' inline declaration
    mk() // ERROR the signature of 'fun mk(): S' contains 'private-in-file' type 'S'
    S.c() // ERROR cannot access members of private class 'companion object Companion : Any'
}
internal inline fun r4() {
    val o = object { fun g() = 1 }
    o.g() // its own local class, which is fine
}

// Private inline functions can, as can those in private classes.
private inline fun p1(): S? = mk()
private class P {
    inline fun p2(): S? = mk()
}

// The default value of a noinline parameter isn't inlined.
internal inline fun r5(
    noinline a: (Any) -> Unit = { it is S },
    b: (Any) -> Unit = { it is S }, // ERROR 'private-in-file' type 'S' is accessed from 'internal' inline declaration
) {}
