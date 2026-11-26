// A safe call's receiver isn't null where the safe call's value, cast
// with as?, isn't, and a stable property a val holds the safe read of
// isn't null where the val isn't, as in K2.
class Holder(val bar: CharSequence?, val foo: Int) {
    fun baz(): Any = true
}

fun castThenElvis(x: Holder?): Int {
    val bar = x?.bar as? String ?: return -1
    return x.foo + bar.length
}

fun callCastThenElvis(x: Holder?): Int {
    val baz = x?.baz() as? Boolean ?: return -1
    return if (baz) x.foo else 0
}

fun castInCondition(x: Holder?): Int {
    if ((x?.bar as? String) != null) return x.foo
    return -1
}

fun valOfSafeRead(x: Holder?): Int {
    val bar = x?.bar
    if (bar != null) {
        return x.bar.length
    }
    return -1
}

fun main() {
    println(castThenElvis(Holder("abc", 1)))
    println(castThenElvis(Holder(StringBuilder("abc"), 1)))
    println(castThenElvis(null))
    println(callCastThenElvis(Holder(null, 2)))
    println(castInCondition(Holder("x", 3)))
    println(castInCondition(Holder(null, 3)))
    println(valOfSafeRead(Holder("four", 0)))
    println(valOfSafeRead(Holder(null, 0)))
}
