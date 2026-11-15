// Constraints with a Java method's T! as K2 2.4 simplifies them: a
// nullable argument keeps its ?, a non-null one is a flexible lower
// bound, which the variable is fixed first for, and T! <: X gives an
// upper bound X!.

class Key<T>

fun <S> select(x: S, y: S): S = x

fun eat(s: String) {}

fun test(s: String?, n: String, arr: Array<String?>, list: ArrayList<String>) {
    eat(J.transform(s)) // ERROR type mismatch: inferred type is String? but String was expected
    eat(J.transform(n))
    val res = J.transform(s)
    eat(res) // ERROR type mismatch: inferred type is String? but String was expected
    val c: Collection<String> = J.singleton(s) // ERROR type mismatch: inferred type is MutableSet<String?>! but Collection<String> was expected
    val d: Collection<String> = J.singleton(s!!)
    val e: List<String> = J.asList(list.toArray(arr))
}

fun <T> setValue(key: Key<T>, value: T, j: J) {
    j.put(key, select(value, null))
}
