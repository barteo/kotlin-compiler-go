// References to extensions and generic members of implicit receivers
// and to an enum's valueOf, compound assignments to objects, Unit values
// in arrays, a lambda on the line after a name, a contract that implies a
// Boolean parameter, and an object's constants read on expressions.

import kotlin.contracts.ExperimentalContracts
import kotlin.contracts.contract

var log = ""

class A {
    fun test() {
        (::ext).let { it() }
        with(B()) { (::bExt).let { it() } }
    }
    class B
}

fun A.ext() { log += "A.ext," }
fun A.B.bExt() { log += "B.ext," }

enum class E { ONE, TWO }

object Counter {
    var sum = 0
    operator fun plusAssign(n: Int) { sum += n }
}

class Logger {
    companion object {
        var text = ""
        operator fun plusAssign(s: String) { text += s }
    }
}

class Cell<T>(var value: T) {
    fun put(x: T) { value = x }
}

fun unit() {}

fun units(vararg us: Unit) = us.size

open class S
class P(val str: String) : S()

@OptIn(ExperimentalContracts::class)
fun verify(condition: Boolean) {
    contract { returns() implies condition }
    if (!condition) throw AssertionError()
}

fun main() {
    A().test()
    println(log)

    val valueOf = E::valueOf
    println(valueOf("TWO"))
    val values: () -> Array<E> = E::values
    println(values().size)

    Counter += 2
    Counter += 3
    println(Counter.sum)
    Logger += "a"
    Logger += "b"
    println(Logger.text)

    val u = Unit
    { log = "lambda" }.let { it() }
    println(u)
    println(log)
    val array = arrayOf(Unit, unit())
    array[0] = unit()
    println(array[1])
    println(units(Unit, *array, unit()))

    val s: S = P("smart")
    verify(s is P)
    println(s.str)

    var n = 0
    println((if (n++ == 0) Int else Int).MAX_VALUE)
    val ul = ULong
    println(ul.MAX_VALUE)
    println(n)

    // A generic member of an implicit receiver takes its type arguments.
    with(Cell("cell")) {
        val put = ::put
        put("put")
        println(value.length)
    }
    println(sequence { yield("y"); id(::yield) }.toList())
}

fun <T> id(x: T) = x
