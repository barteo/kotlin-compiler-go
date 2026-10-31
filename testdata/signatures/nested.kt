// Wildcards within type arguments: an invariant argument's arguments are
// a result's, a contravariant one's a parameter's (TestSignatures).
package q
open class Foo
class Fin
class Box<T>(val t: T)
fun p1(a: (List<Foo>) -> Unit) {}
fun p2(a: Comparable<List<Foo>>) {}
fun p3(a: List<Comparable<Foo>>) {}
fun p4(a: List<(Foo) -> Unit>) {}
fun p5(a: List<Array<Foo>>, b: List<Array<Fin>>, c: List<IntArray>) {}
fun p6(a: Pair<List<Foo>, Fin>) {}
fun p7(a: Comparable<Comparable<Foo>>) {}
fun p8(a: Box<Foo>, b: List<Box<Foo>>, c: List<Box<Fin>>, d: List<Box<*>>, e: Box<out Foo>) {}
fun p9(a: List<Pair<Fin, Fin>>, b: List<Pair<Foo, Fin>>, c: List<Pair<Fin, Comparable<Fin>>>) {}
fun p10(a: Array<out List<Foo>>, b: Array<in Foo>, c: Array<Array<Foo>>) {}
fun p11(a: List<Unit>, b: List<Nothing?>, c: List<Any?>, d: Comparable<Any?>, e: List<Fin?>) {}
fun p12(a: @JvmSuppressWildcards List<List<Foo>>, b: List<@JvmSuppressWildcards List<Foo>>, c: List<@JvmWildcard Fin>, d: List<List<@JvmSuppressWildcards Foo>>) {}
fun p13(a: Map<Class<*>, @JvmSuppressWildcards Foo>, b: Set<@JvmSuppressWildcards Box<*>>) {}
fun r1(): Comparable<List<Foo>>? = null
fun r2(): List<@JvmWildcard List<Foo>> = emptyList()
fun r3(): Array<List<Foo>> = arrayOf()
fun r4(): Map<Class<*>, @JvmSuppressWildcards Foo> = emptyMap()
fun <T> g1(a: List<T>, b: Comparable<T>, c: Box<T>, d: Array<T>): Array<T> = d
fun <T : Any> g2(a: List<T?>): T? = null
enum class En { X }
fun en(a: List<En>, b: Comparable<En>) {}
sealed class Sealed
object Single
fun sealedObj(a: List<Sealed>, b: List<Single>) {}
interface Iface
fun iface(a: List<Iface>) {}
abstract class Abs
fun abs(a: List<Abs>) {}
fun strs(a: List<CharSequence>, b: List<Number>, c: List<Long>, d: List<Char>, e: List<Boolean>) {}
fun javaFinal(a: List<java.util.UUID>, b: List<java.time.Instant>, c: List<java.lang.StringBuilder>) {}
fun mutable(a: MutableMap<String, MutableList<Foo>>, b: MutableCollection<out Foo>, c: Iterable<Foo>, d: Sequence<Foo>, e: Collection<Foo>) {}
var vfield: List<Comparable<Foo>> = emptyList()
var vfun: (Foo) -> Foo = { it }
class P(var a: List<Foo>, val b: List<Foo>, private var c: List<Foo>)
