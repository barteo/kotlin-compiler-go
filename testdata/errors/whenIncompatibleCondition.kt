// A when's condition that can't equal its subject is kotlinc's
// INCOMPATIBLE_TYPES; an == of the same types is EQUALITY_NOT_APPLICABLE.

interface A

fun f(a: A, b: Boolean) {
    when (a) {
        true -> {} // ERROR incompatible types A and Boolean
        b -> {} // ERROR incompatible types A and Boolean
        else -> {}
    }
    if (a == true) {} // ERROR operator '==' cannot be applied to A and Boolean
}
