// Signatures of constructors of inner classes, which leave out the outer
// instance, of overloads, extension properties and interface bodies, and
// of collection members that override Java's (TestSignatures).
package r
open class Foo
class Box<T>(val t: T)
fun i1(a: Box<List<Foo>>, b: MutableList<List<Foo>>, c: java.util.ArrayList<List<Foo>>, d: Box<Comparable<Foo>>, e: MutableMap<String, (Foo) -> Unit>) {}
fun i2(): Box<Comparable<List<Foo>>>? = null
fun i3(a: Box<out List<Foo>>, b: Box<in List<Foo>>) {}
class Outer<O> {
    inner class Plain(val x: Int)
    inner class Gen(val l: List<Foo>)
    inner class UsesO(val o: O)
    fun local(n: Int): Any {
        class Loc(val y: Int) { fun get() = y + n }
        val anon = object { val l: List<Foo> = emptyList(); fun f(a: List<Foo>) = a }
        return Loc(1)
    }
}
class Defaults(val a: List<Foo> = emptyList(), val b: Int = 0)
class Overloads @JvmOverloads constructor(val a: List<Foo>, val b: Int = 0)
@JvmOverloads fun ov(a: List<Foo>, b: Int = 0) {}
val List<Foo>.extProp: List<Foo> get() = this
var <T> Box<T>.extVar: List<T>
    get() = emptyList()
    set(v) {}
interface WithDefault { fun d(a: List<Foo>): List<Foo> = a }

abstract class Coll<E> : AbstractCollection<E>() {
    override fun contains(element: E): Boolean = false
    override fun containsAll(elements: Collection<E>): Boolean = false
}
class Lookup<K, V>(val m: Map<K, V>) : Map<K, V> by m
abstract class ReadOnly<E> : List<E> {
    override fun indexOf(element: E): Int = -1
}
