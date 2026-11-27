// The warnings kotlinc 2.4.21 reports of type arguments inferred as an
// intersection that is or may be empty: each line ends with a comment
// naming the warnings it reports, if any.
import kotlin.reflect.KProperty

interface Base
class NotBase
open class Foo
class Bar
class In<in K>
class Box<B : Box<B>>
interface Self<S : Self<S>>

fun <T, V> takesFun(f: V) where T : Base, V : (T) -> Unit {}
fun <T : Foo> foo(): T? = null
fun <T : Appendable> appendable(): T = TODO()
fun <E> intersect(vararg x: In<E>): E = TODO()
fun <B> both(): B where B : Box<B>, B : Self<B> = TODO()
fun <T> id(x: T): T = x
operator fun <T, S : T> In<T>.getValue(t: Any?, p: KProperty<*>): S = TODO()

fun test(f: (NotBase) -> Unit, m: Map<String, Int>, i: In<Int>) {
    takesFun(f) // INFERRED_TYPE_VARIABLE_INTO_POSSIBLE_EMPTY_INTERSECTION
    val a: Bar? = foo() // INFERRED_TYPE_VARIABLE_INTO_EMPTY_INTERSECTION_WARNING
    val b: Int = appendable() // INFERRED_TYPE_VARIABLE_INTO_POSSIBLE_EMPTY_INTERSECTION
    val c: StringBuilder = appendable()
    val d = intersect(In<Int>(), In<String>()) // INFERRED_TYPE_VARIABLE_INTO_EMPTY_INTERSECTION_WARNING
    val e = intersect(In<Int>(), In<Number>())
    val g: String by m // INFERRED_TYPE_VARIABLE_INTO_EMPTY_INTERSECTION_WARNING
    val h: Int by m
    val k = id(1)
    both()
    println(listOf(a, b, c, d, e, g, h, k))
}
val strings: Map<String, Int> = mapOf()
val top: String by strings // INFERRED_TYPE_VARIABLE_INTO_EMPTY_INTERSECTION_WARNING
class Holder { val member: String by strings } // INFERRED_TYPE_VARIABLE_INTO_EMPTY_INTERSECTION_WARNING

interface BareI
open class BareC
fun <T> bareAny(): T = null!!
fun <T : BareI> bareI(): T = null!!
fun <T : BareC> bareC(): T = null!!
fun <T : Any> bareNonNull(): T = null!!
val bareP = false
fun bare1() = run {
    if (bareP) return@run
    bareI() // INFERRED_TYPE_VARIABLE_INTO_POSSIBLE_EMPTY_INTERSECTION
}
fun bare2() = run {
    if (bareP) return@run
    bareC() // INFERRED_TYPE_VARIABLE_INTO_EMPTY_INTERSECTION_WARNING
}
fun bare3() = run {
    if (bareP) return@run
    bareAny<Unit>()
}
fun bare4() = run {
    if (bareP) return@run
    bareNonNull()
}
fun bare5() = run {
    bareI<BareI>()
}
