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

// A var assigned null is a Nothing? until it is assigned again, as K2's
// smart casts: not where a path in between may assign it, nor a var
// declared with null.
class Na(var n: Int) { operator fun get(i: Int) = i; fun f() = 1 }
fun naUse(a: Any?) {}
fun assignedNull(b: Boolean) {
    var c: Na? = Na(1)
    c = null
    naUse(c ?: Na(2))
    naUse(c?.get(0))
    naUse(c?.f())
    c?.n += 1
    c?.n++
    naUse(c?.let { it.n })
    naUse(c is Na)
    naUse(c as? Na) // USELESS_CAST
    naUse(c == null) // SENSELESS_COMPARISON
    naUse(c === null) // SENSELESS_COMPARISON
    naUse(c!!)
    var s: String? = "a"
    if (b) s = null
    naUse(s != null)
    s = null
    while (b) { naUse(s != null); s = "x" }
    s = null
    for (i in 0..1) { if (s == null) s = "y" }
    s = null
    try { s = "z" } catch (e: Exception) {}
    naUse(s == null)
    s = null
    run { naUse(s == null) } // SENSELESS_COMPARISON
    var t: String? = null
    naUse(t != null)
    t = null
    naUse(t == null) // SENSELESS_COMPARISON
    var u: String?
    u = null
    naUse(u == null) // SENSELESS_COMPARISON
}

// A Java method's result is as the Kotlin member it overrides gives it:
// not null for MutableList's iterator(), which MutableCollection
// declares, ArrayList's, StringBuilder's toString() and subSequence().
// Throwable's message is nullable.
fun javaOverrides(l: List<Int>, ml: MutableList<Int>, a: ArrayList<Int>, sb: StringBuilder, o: Any, cs: CharSequence, hm: HashMap<String, Int>, t: Thread) {
    naUse(l.iterator() != null) // SENSELESS_COMPARISON
    naUse(ml.iterator() != null) // SENSELESS_COMPARISON
    naUse(a.iterator() != null) // SENSELESS_COMPARISON
    naUse(a.listIterator() != null) // SENSELESS_COMPARISON
    naUse(sb.toString()!!) // UNNECESSARY_NOT_NULL_ASSERTION
    naUse(o.toString()!!) // UNNECESSARY_NOT_NULL_ASSERTION
    naUse(cs.toString()!!) // UNNECESSARY_NOT_NULL_ASSERTION
    naUse(sb.subSequence(0, 1)!!) // UNNECESSARY_NOT_NULL_ASSERTION
    naUse(hm.keys!!) // UNNECESSARY_NOT_NULL_ASSERTION
    naUse(hm.entries!!) // UNNECESSARY_NOT_NULL_ASSERTION
    naUse(t.toString()!!) // UNNECESSARY_NOT_NULL_ASSERTION
    naUse(a.subList(0, 1)!!) // UNNECESSARY_NOT_NULL_ASSERTION
    naUse(Exception().message!!)
    naUse(java.io.File("x").toString()!!) // UNNECESSARY_NOT_NULL_ASSERTION
    naUse(java.io.File("x").getParent()!!)
    naUse(t.getName()!!)
}
