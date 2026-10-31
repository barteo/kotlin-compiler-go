// References adapted to the expected function type: arguments after a
// vararg go to the parameters with defaults at their positions, an array
// to the vararg, and a reference to the only function of its name gives a
// type parameter its type before an integer literal does. A var smart
// cast where it is declared loses the cast in a lambda that may run again
// after assigning it, and a contract's is R, for a reified R, narrows to
// the call's type argument.
import kotlin.contracts.*

fun join(vararg xs: String, sep: String = ",", end: String = "."): String = xs.joinToString(sep) + end

fun call3(f: (String, String, String) -> String) = f("a", "b", "c")
fun callArr(f: (Array<String>, String) -> String) = f(arrayOf("x", "y"), "-")

fun <T> pass(x: T, f: (T) -> String) = f(x)
fun ofByte(b: Byte) = "byte $b"
fun ofShort(s: Short) = "short $s"

open class Node(val name: String, val up: Node?)

class Leaf(name: String, up: Node?) : Node(name, up)

fun climb(start: Node): String {
    var n: Node = start
    if (n is Leaf) {
        var cur = n
        repeat(2) {
            cur = cur.up ?: cur
        }
        return cur.name
    }
    return "none"
}

class Check<R>

@OptIn(ExperimentalContracts::class)
inline fun <reified R> Check<R>.ensure(value: Any): Boolean {
    contract { returns() implies (value is R) }
    if (value !is R) throw IllegalStateException()
    return true
}

fun length(c: Check<String>, x: Any): Int {
    c.ensure(x)
    return x.length
}

fun main() {
    println(call3(::join))
    println(callArr(::join))
    println(pass(1, ::ofByte))
    println(pass(2, ::ofShort))
    println(climb(Leaf("leaf", Node("mid", Node("root", null)))))
    println(length(Check(), "four"))
}
