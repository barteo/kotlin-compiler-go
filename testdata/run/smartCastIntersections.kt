// Smart casts as intersections: b is A && b is C keeps both, t is T
// for a t: T? is a T, and t == v of two generic types is both.

open class B
interface A { fun a() = "a" }
interface C { fun c() = "c" }
class D : B(), A, C
class Box<V>(val value: V)
open class W<out T>
data class Wrapped<out T>(val value: T) : W<T>()
object Default : W<Nothing>()
inline fun <reified T> wrap(t: T?): W<T> = if (t is T) Wrapped(t) else Default
fun gav(b: B): A? = if (b is A && b is C) b else null
fun both(b: B): String = if (b is A && b is C) b.a() + b.c() else "-"
fun <T, V> same(t: Box<out T>, v: Box<out V>): String {
    if (t == v) { val x: T = v.value; val y: V = t.value; return "" + x + y }
    return "no"
}
fun main() {
    println(gav(D()) != null)
    println(both(D()))
    println(wrap<String>("s"))
    println(wrap<String?>(null))
    val b = Box("q")
    println(same(b, b))
}
