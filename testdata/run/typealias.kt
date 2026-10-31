// Typealiases: of function types, of classes with and without type
// arguments, nested, of other aliases, and as supertypes and constructors.

typealias Handler = (String) -> Int
typealias Pred<T> = (T) -> Boolean
typealias Names = java.util.ArrayList<String>
typealias Box2<T> = Box<T>
typealias IntBox = Box<Int>
typealias MaybeBox = Box<String>?
typealias AliasOfAlias = Handler
typealias Partial<V> = java.util.HashMap<String, V>
typealias MaybeName = String?
typealias Recv<T> = StringBuilder.(T) -> Unit

class Box<T>(val v: T) {
    companion object {
        fun <T> of(v: T) = Box(v)
    }
}

class Registry {
    typealias Key = String
    val keys = java.util.ArrayList<Key>()
    fun add(k: Key) { keys.add(k) }
}

open class Base(val n: Int)
typealias BaseAlias = Base
class Derived : BaseAlias(3)

fun run(h: AliasOfAlias, s: String) = h(s)
fun <T> count(xs: java.util.ArrayList<T>, p: Pred<T>): Int {
    var n = 0
    for (x in xs) if (p(x)) n++
    return n
}

fun main() {
    val h: Handler = { it.length }
    println(run(h, "four"))
    val names = Names()
    names.add("a"); names.add("bb")
    println(count(names) { it.length > 1 })
    val b: Box2<Int> = Box2(5)
    val ib: IntBox = IntBox(6)
    println(b.v + ib.v)
    val m: MaybeBox = null
    println(m == null)
    println(Box2.of("x").v)
    val r = Registry()
    r.add("k")
    val k: Registry.Key = "q"
    println(r.keys.size.toString() + k)
    println(Derived().n)
    val any: Any = ib
    println(any is Box2<*>)
    println((any as Box2<*>).v)
    val p = Partial<Int>()
    p.put("one", 1)
    println(p.get("one"))
    val n: MaybeName = if (p.isEmpty()) "full" else null
    println(n ?: "none")
    val r2: Recv<Int> = { append(it * 2) }
    val sb = StringBuilder()
    sb.r2(21)
    println(sb)
}
