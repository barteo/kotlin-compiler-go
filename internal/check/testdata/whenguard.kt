// The warnings kotlinc 2.4.21 reports of the checks after guarded when
// branches below, as TestWarningsWhenGuards reads them: each line ends
// with a comment naming the warnings it reports, if any. A branch that
// isn't taken tells later ones that its condition failed or its guard
// did.
sealed class Holder(val value: Boolean)
object Yes : Holder(true)

// x !== null can't fail: x is an Int after the first branch.
fun f1(x: Any) = when (x) {
    !is Int if x !== null -> 1 // SENSELESS_COMPARISON
    !is String if x > 0 -> 2 // IMPOSSIBLE_IS_CHECK_WARNING
    is Holder if x.value -> 3 // IMPOSSIBLE_IS_CHECK_WARNING
    else -> 4
}

// The guard can fail: nothing is known after it.
fun f2(x: Any, b: Boolean) = when (x) {
    !is Int if b -> 1
    is Holder -> 2
    else -> 3
}

// A nullable subject is an Int or null where x !== null fails.
fun f3(x: Any?) = when (x) {
    !is Int if x !== null -> 1
    is Holder -> 2 // IMPOSSIBLE_IS_CHECK_WARNING
    else -> 3
}

// is T if ... tells nothing of the branches after it.
fun f4(x: Any) = when (x) {
    is Int if x > 0 -> 1
    is Holder -> 2
    else -> 3
}

// Nor does a guard on another value.
fun f5(x: Any, y: Any?) = when (x) {
    !is Int if y !== null -> 1
    is Holder -> 2
    else -> 3
}

// x is String & Int in the last branch, which compares as a String.
fun f6(x: Any) = when (x) {
    !is Int if x !== null -> 1 // SENSELESS_COMPARISON
    is String if x == "" -> 2 // IMPOSSIBLE_IS_CHECK_WARNING
    else -> 3
}
