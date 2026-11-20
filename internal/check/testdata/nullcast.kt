// The warnings below, as TestWarningsNullCast reads them: each line ends
// with a comment naming the warnings it reports, if any. They follow
// kotlinc 2.4.21's diagnostics tests elvisNotProcessed.kt,
// lambdasInExclExclAndElvis.kt, constants.kt, deadCallInReceiver.kt,
// SenselessNullInWhen.kt, whenOnNothing.kt, earlyJumps.kt and
// explicitFieldWithFunctionType.kt.
@file:Suppress("UNUSED_PARAMETER", "UNUSED_VARIABLE", "UNUSED_EXPRESSION", "UNREACHABLE_CODE")

fun todo(): Nothing = throw Exception()
fun Any.bar(i: Int) {}

fun lambdas() {
    val v = { true } ?: 1 // USELESS_ELVIS
    val w = { true }!! // NOT_NULL_ASSERTION_ON_LAMBDA_EXPRESSION
    val f = fun() {} ?: 2 // USELESS_ELVIS
}

fun casts(f: (Int) -> Number, a: Any) {
    val n = null as Nothing? // USELESS_CAST
    val u = 1UL as ULong // USELESS_CAST
    val l = 1L as Long // USELESS_CAST
    val i = 1 as Long // NUMERIC_CAST_NEVER_SUCCEEDS_BUT_CAN_BE_REPLACED_WITH_TO_CALL
    val g = f as (Any) -> Int // UNCHECKED_CAST
    val h = a as () -> Unit // UNCHECKED_CAST
    val k = { x: Int -> x } as (Int) -> Int
}

fun nothing() {
    todo()?.bar(1) // UNNECESSARY_SAFE_CALL
}

fun whens(i: Int, n: Nothing?, s: String?, x: Any) {
    when (i) {
        1 -> 1
        null -> 2 // SENSELESS_NULL_IN_WHEN
        else -> 3
    }
    when (n) {
        null -> 1 // SENSELESS_COMPARISON
    }
    when (s) {
        null -> 1
        else -> 2
    }
    if (n == null) {} // SENSELESS_COMPARISON
    when (x) {
        is String if x == "10" ?: throw Exception() -> 3 // USELESS_ELVIS
        else -> 4
    }
}
