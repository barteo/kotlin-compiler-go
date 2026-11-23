// The warnings below, as TestWarningsNullSmartCasts reads them: each line
// ends with a comment naming the warnings it reports, if any. They follow
// kotlinc 2.4.21's diagnostics tests comparisonOfBuiltInTypesUnderOr.kt,
// ExhaustiveSelftype.kt, infos/SmartCasts.kt, DeepIf.kt,
// deadCodeInUnaryExpr.kt, complexComparison.kt, castchecks/basicOn.kt,
// kt701.kt, alwaysNull.kt, FlexibleTargetType.kt,
// checkCastToNullableType.kt, kt498.kt,
// ExhaustiveWhenNegatedTwice.kt and smartCasts/controlStructures/when.kt.
@file:Suppress("UNUSED_PARAMETER", "UNUSED_VARIABLE", "UNUSED_EXPRESSION", "UNREACHABLE_CODE")

open class A { fun foo() {} }
class B : A()
interface I

fun todo(): Nothing = throw Exception()

fun isAny(x: Any?) {
    if (x is Any || x == null) {} // SENSELESS_COMPARISON
    if (x !is Any || x != null) {} // SENSELESS_COMPARISON
    if (x is String || x == null) {}
}

fun isAnyWhen(x: CharSequence?, a: A?): Int {
    val i = when (x) {
        is Any -> 1
        null -> 2 // SENSELESS_COMPARISON
    }
    val j = when (x) {
        is CharSequence -> 1
        null -> 2
    }
    when (a) {
        is B -> a.foo()
        is Any -> a.foo()
        is Any? -> {} // USELESS_IS_CHECK
        else -> a?.foo() // REDUNDANT_ELSE_IN_WHEN UNNECESSARY_SAFE_CALL
    }
    return i + j
}

fun redundantElse(n: Int) = when (n) {
    is Any -> 1 // USELESS_IS_CHECK
    else -> 2 // REDUNDANT_ELSE_IN_WHEN
}

fun nothings(x: String?) {
    todo()!! // UNNECESSARY_NOT_NULL_ASSERTION
}

fun alwaysNull(x: String?, y: String?, z: String?) {
    if (x == null) {
        if (x == null) {} // SENSELESS_COMPARISON
    }
    if (y != null || (z == null && y != z)) {} // SENSELESS_COMPARISON
}

fun safeCasts(a: A?) {
    if (a as? B != null) {
        (a as B).foo() // USELESS_CAST
    }
}

fun <X : Throwable?> safeReceivers(declaredType: Class<X?>?, t: Throwable) {
    declaredType?.cast(t)!!
    declaredType?.cast(t) // UNNECESSARY_SAFE_CALL
}

fun stillNull(s: String?) {
    if (s != null) return
    s as? String // USELESS_CAST
    s as String?
}

fun platforms() {
    System.getProperty("a") as String
    System.getProperty("a") as String? // USELESS_CAST
    val f = System.getProperty("a")
    f is String? // USELESS_IS_CHECK
}

fun <T : Any> klass(): Class<T> = Class.forName("A") as Class<T> // UNCHECKED_CAST

sealed class Sealed {
    object First : Sealed()
    object Last : Sealed()
}

fun negated(s: Sealed): Int = when (s) {
    !is Sealed.First -> 1
    !is Sealed.Last -> 0 // IMPOSSIBLE_IS_CHECK_WARNING
}

fun subjectVar(x: Any) {
    when (val y = x) {
        !is A -> {}
        is Int -> {} // IMPOSSIBLE_IS_CHECK_WARNING
    }
}

// A value known to be null keeps its declared type's members where it
// isn't null: kotlinc's intersection with Nothing.
fun contradictions(b: A?, d: A?) {
    if (b !is Any?) { // USELESS_IS_CHECK
        b?.foo() // UNNECESSARY_SAFE_CALL
        b.foo()
        b!!.foo() // UNNECESSARY_NOT_NULL_ASSERTION
    }
    if (d is Any || d == null) {} else { d.foo() } // SENSELESS_COMPARISON
}
