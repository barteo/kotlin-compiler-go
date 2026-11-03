// Calls on nullable receivers that are still unsafe.

fun String.ext() = length
class A { fun m() = 1 }

fun main() {
    val s: String? = if (flag()) null else "x"
    s.length // ERROR only safe (?.) or non-null asserted (!!.) calls are allowed on a nullable receiver of type String?
    s.ext() // ERROR only safe (?.) or non-null asserted (!!.) calls are allowed on a nullable receiver of type String?
    val a: A? = null
    a.m() // ERROR only safe (?.) or non-null asserted (!!.) calls are allowed on a nullable receiver of type A?
    val n: Int? = null
    n.plus(1) // ERROR only safe (?.) or non-null asserted (!!.) calls are allowed on a nullable receiver of type Int?
}

fun flag() = true
