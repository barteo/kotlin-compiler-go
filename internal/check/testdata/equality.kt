// EQUALITY_NOT_APPLICABLE_WARNING: a comparison inapplicable to the
// smart-cast types of its operands, but not to their declared types.
class B
@JvmInline value class V(val x: Int)
fun f1(a: Any, b: Any) {
    if (a is String && b is Int) {
        println(a == b) // EQUALITY_NOT_APPLICABLE_WARNING
        println(a != b) // EQUALITY_NOT_APPLICABLE_WARNING
    }
}
fun f2(a: Any?, b: Any?) {
    if (a is Int && b is B) println(a == b) // EQUALITY_NOT_APPLICABLE_WARNING
    if (a is Int? && b is String?) println(a == b) // EQUALITY_NOT_APPLICABLE_WARNING
    if (a is V && b is String) println(a == b) // EQUALITY_NOT_APPLICABLE_WARNING
    if (a is B && b is String) println(a === b) // EQUALITY_NOT_APPLICABLE_WARNING
    if (a is CharSequence && b is Number) println(a == b)
}
fun f3(a: Any) {
    if (a is String) println(a == 1) // EQUALITY_NOT_APPLICABLE_WARNING
    if (a is Int) println(a == 1L) // EQUALITY_NOT_APPLICABLE_WARNING
    if (a is Int) println(a == 1.0) // EQUALITY_NOT_APPLICABLE_WARNING
    if (a is Int) println(a == 1)
    if (a is Int) {
        @Suppress("EQUALITY_NOT_APPLICABLE_WARNING")
        println(a == "x")
    }
}
fun f4(a: Any, s: String) {
    if (a is Int) println(s == a) // EQUALITY_NOT_APPLICABLE_WARNING
}
