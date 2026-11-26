// A contract's implications through comparisons with its function's
// result: a safe call, or a Boolean? result, equal to true or false;
// a when over a Boolean? result; a result that isn't null, or is null,
// given to another contract function; and a call compared with Unit.
@file:OptIn(kotlin.contracts.ExperimentalContracts::class)

import kotlin.contracts.*

class A {
    fun returnsTrue(x: Any?): Boolean {
        contract { returns(true) implies (x is String) }
        return x is String
    }

    fun returnsFalse(x: Any?): Boolean {
        contract { returns(false) implies (x is String) }
        return x !is String
    }

    fun returnsNotNull(x: Any?): Any? {
        contract { returnsNotNull() implies (x is String) }
        return if (x is String) x else null
    }
}

fun safeIsString(x: Any?): Boolean? {
    contract { returns(true) implies (x is String) }
    return x?.let { it is String }
}

fun nullWhenNotString(x: Any?): String? {
    contract { returnsNotNull() implies (x is String) }
    return x as? String
}

fun nullWhenString(x: Any?): Any? {
    contract { returns(null) implies (x is String) }
    return if (x is String) null else x
}

fun notEqualsNull(x: Any?): Boolean {
    contract { returns(true) implies (x != null) }
    return x != null
}

fun requireNotNull2(x: Any?) {
    contract { returns() implies (!(x == null)) }
    if (x == null) throw IllegalStateException()
}

fun myAssert(condition: Boolean) {
    contract { returns() implies (condition) }
    if (!condition) throw IllegalArgumentException("assertion failed")
}

fun myRequireAnd(b1: Boolean, b2: Boolean) {
    contract { returns() implies (b1 && b2) }
    if (!(b1 && b2)) throw IllegalStateException()
}

fun trueWhenString(x: Any?): Boolean {
    contract { returns(true) implies (x is String) }
    return x is String
}

fun falseWhenString(x: Any?): Boolean {
    contract { returns(false) implies (x is String) }
    return x !is String
}

interface Named { val name: String }
interface Sized { val size: Int }
class Box(override val name: String, override val size: Int) : Named, Sized

fun safeCalls(a: A?, s: Any?): String {
    var out = ""
    if (a?.returnsTrue(s) == true) out += s.length
    if (a?.returnsFalse(s) == false) out += s.length
    if (a?.returnsNotNull(s) != null) out += s.length
    return out
}

fun nullableResult(s: Any?): String {
    var out = ""
    if (safeIsString(s) == true) out += s.length
    if (safeIsString(s) != true) out += "-" else out += s.length
    when (safeIsString(s)) {
        true -> out += s.length
        else -> out += "-"
    }
    when (safeIsString(s)) {
        false -> out += "f"
        null -> out += "n"
        true -> out += s.length
    }
    return out
}

fun nonNullResults(s: Any?): String {
    var out = ""
    if (nullWhenNotString(s) != null) out += s.length
    if (notEqualsNull(nullWhenNotString(s))) out += s.length
    if (nullWhenString(s) == null) out += s.length
    return out
}

fun returned(s: Any?, t: Any?, n: Any?): String {
    requireNotNull2(n)
    if (myAssert(s is String) == Unit) {
        myRequireAnd(t is Named, t is Sized)
        return s.length.toString() + t.name + t.size + n.hashCode()
    }
    return ""
}

fun constants(s: Any?): String {
    if (falseWhenString(s) && true) return "-"
    return s.length.toString() + if (trueWhenString(s) || false) s.length else 0
}

fun main() {
    println(safeCalls(A(), "abc"))
    println(safeCalls(null, "abc"))
    println(nullableResult("ab"))
    println(nullableResult(1))
    println(nullableResult(null))
    println(nonNullResults("abcd"))
    println(returned("xyz", Box("b", 7), 0))
    println(constants("hello"))
}
