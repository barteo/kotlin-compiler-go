interface Node {
    var index: Int
}

class Item(val v: Int) : Node, Comparable<Item> {
    override var index = -1
    override fun compareTo(other: Item) = v.compareTo(other.v)
    override fun toString() = "Item($v@$index)"
}

// Both bounds are interfaces: T erases to the first, Node.
class Heap<T> where T : Node, T : Comparable<T> {
    private var a: Array<T?> = arrayOfNulls<Node>(4) as Array<T?>
    private var size = 0

    fun add(x: T) {
        x.index = size
        a[size++] = x
        var i = x.index
        while (i > 0) {
            val p = (i - 1) / 2
            val pi = a[p]!!
            val xi = a[i]!!
            if (pi <= xi) break
            a[p] = xi; a[i] = pi
            xi.index = p; pi.index = i
            i = p
        }
    }

    fun min(): T? = a[0]
}

interface Copyable<T> where T : Throwable, T : Copyable<T> {
    fun copy(): T?
}

class MyError(msg: String) : Exception(msg), Copyable<MyError> {
    override fun copy() = MyError(message + "!")
}

// Two interfaces: T erases to the first, CharSequence.
fun <T> describe(x: T): String where T : CharSequence, T : Comparable<T> =
    "${x.length} ${x.compareTo(x)}"

// A class bound after an interface bound: T erases to the class, Number.
fun <T> twice(x: T): Int where T : Comparable<T>, T : Number = x.toInt() * 2 + x.compareTo(x)

class Holder<T>(val x: T) where T : Comparable<T>, T : Number {
    fun get(): T = x
}

fun <T> copyOf(e: T): String where T : Throwable, T : Copyable<T> = e.copy()?.message ?: "none"

fun <K, V> pick(m: Map<K, V>, k: K): V?
    where K : Comparable<K>,
          V : Any = m[k]

fun main() {
    val h = Heap<Item>()
    for (v in listOf(5, 3, 8, 1)) h.add(Item(v))
    println(h.min())
    println(describe("abc"))
    println(copyOf(MyError("boom")))
    println(pick(mapOf("a" to 1), "a"))
    println(twice(21))
    println(Holder(2.5).get())
}
