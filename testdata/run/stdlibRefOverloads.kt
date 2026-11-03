// Overloads chosen by the callables a reference argument may name: the
// functions of its name, or a property, at the top level or of a class.
import kotlin.reflect.KProperty0
import kotlin.reflect.KProperty1

class Inv<K>(val k: K)
interface Base

fun bar(): Int = 1
fun bar(x: Double): Int = 2
fun foo(x: (String) -> Int) = println("string " + x("s"))
fun foo(x: () -> Int) = println("none " + x())

fun inv(x: Int): Inv<Int> = Inv(x)
fun inv(y: String): Inv<String> = Inv(y)
fun <T, R : Number> pick(f: (T) -> Inv<R>, p: String = "") = println("number")
fun <T, R : Base> pick(f: (T) -> Inv<R>, p: Int = 4) = println("base")

fun <R> property(property: KProperty0<R>): String = "top " + property.get()
fun <T, R> property(property: KProperty1<T, R>): String = "member " + property.name

val subject = "s0"
class O {
    val subject = "s1"
}

suspend fun refA(input: String): String = input
suspend fun refB(): String = "b"
fun method(f: suspend (String) -> String) = println("one")
fun method(f: suspend () -> String) = println("none")

fun main() {
    foo(::bar)
    pick(::inv)
    println(property(::subject))
    println(property(O::subject))
    method(::refA)
    method(::refB)
}
