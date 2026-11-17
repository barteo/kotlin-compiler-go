@file:OptIn(ExperimentalContracts::class)

// Smart casts that K2 makes from: a non-null (x as? T), an equality with
// a String, a bound value checked in a when, and contracts with a
// conjunction, returnsNotNull() and a call that initializes a val.

import kotlin.contracts.ExperimentalContracts
import kotlin.contracts.contract

open class Base
class Sub(val data: String) : Base()

fun Sub.isOk() = data.isNotEmpty()

fun viaSafeCast(base: Base): String = when {
    (base as? Sub)?.isOk() == true -> "sub " + base.data
    else -> "base"
}

fun insideCall(a: Any): Int? = (a as? String)?.indexOf(a.uppercase())

interface State
interface Complex {
    val superClass: Complex?
}
class Both(override val superClass: Complex?) : State, Complex

fun orReturn(q: State?): String {
    if (q == null || (q as? Complex)?.superClass == null) {
        return "none"
    }
    return "super " + (q.superClass === q)
}

fun equalsString(ts: Array<String>, a: Any?): Int = if (ts[0] == a) a.length else -1

class Box(val data: Any?)

fun boundWhen(x: Box): Int {
    val y = x.data
    return when (y) {
        is String -> x.data.length
        else -> 0
    }
}

fun Any.receiverWhen(): Int = when (val x = this) {
    is List<*> -> this.size + x.size
    is String -> length
    else -> 0
}

fun both(x: Any, y: Any) {
    contract { returns() implies (x is Int && y is String) }
    if (x !is Int || y !is String) throw IllegalStateException()
}

fun conjunction(x: Any, y: Any): String {
    both(y = y, x = x)
    return "${x + 1} ${y.length}"
}

fun notNullIf(value: Int?): Boolean? {
    contract { returnsNotNull() implies (value != null) }
    return value != null
}

fun assertedResult(value: Int?): Int {
    notNullIf(value)!!
    return value.inv()
}

inline fun <reified T> cast(value: Any?): T {
    contract { returns() implies (value is T) }
    return value as T
}

fun initializer(x: Any): Int {
    val s: String = cast(x)
    return x.length + s.length
}

fun main() {
    println(viaSafeCast(Sub("d")))
    println(viaSafeCast(Base()))
    println(insideCall("ab"))
    println(insideCall(1))
    println(orReturn(null))
    println(orReturn(Both(null)))
    val b = Both(null)
    println(orReturn(Both(b)))
    println(equalsString(arrayOf("abc"), "abc"))
    println(equalsString(arrayOf("abc"), 1))
    println(boundWhen(Box("four")))
    println(boundWhen(Box(4)))
    println(listOf(1, 2).receiverWhen())
    println("xyz".receiverWhen())
    println(conjunction(41, "s"))
    println(assertedResult(5))
    println(initializer("str"))
}
