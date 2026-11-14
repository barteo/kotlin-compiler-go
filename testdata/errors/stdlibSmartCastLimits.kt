@file:OptIn(ExperimentalContracts::class)

// Smart casts K2 doesn't make: of a variable assigned the result of a
// call whose contract is about its old value, from a contract's or, from
// returns(false) through !!, and from == where the left side's equals
// isn't trusted for them.

import kotlin.contracts.ExperimentalContracts
import kotlin.contracts.contract

fun notNull(v: String?): String? {
    contract { returns() implies (v != null) }
    return null
}

fun reassigned(a: String?) {
    var v = a
    v = notNull(v)
    v.length // ERROR only safe (?.) or non-null asserted (!!.) calls are allowed
}

fun either(x: Any, y: Any) {
    contract { returns() implies (x is Int || y is String) }
}

fun disjunction(x: Any, y: Any) {
    either(x, y)
    x.inc() // ERROR unresolved reference: inc
}

fun falseIfNotNull(v: Int?): Boolean? {
    contract { returns(false) implies (v != null) }
    return v == null
}

fun trueOnly(v: Int?) {
    if (falseIfNotNull(v)!!) v.inv() // ERROR only safe (?.) or non-null asserted (!!.) calls are allowed
}

open class Open {
    val name = "o"
}
data class Data(val s: String)

fun untrusted(o: Open, d: Data, i: Int, a: Any?, b: String?) {
    if (o == a) a.name // ERROR unresolved reference: name
    if (d == a) a.s // ERROR unresolved reference: s
    if (i == a) a.inc() // ERROR unresolved reference: inc
    if (b == a) a.length // ERROR Any?
}
