interface Source<T> { fun next(): T; val last: T }
abstract class Base<T> { abstract fun make(seed: T): T; open fun twice(t: T): List<T> = listOf(t, t) }
class Buildee<T> { val items = mutableListOf<T>(); fun put(t: T) { items.add(t) }; fun peek(): T = items[0] }
fun <T> build(f: Buildee<T>.() -> Unit): Buildee<T> = Buildee<T>().apply(f)

fun main() {
    val l = buildList {
        add(1)
        fun first() = get(0)
        fun plus(n: Int) = first() + n
        fun same(e: Int) = indexOf(e)
        val o = object {
            val p = get(0)
            fun f() = get(0)
            fun g(x: Long) = x + f()
        }
        class L(val x: Int) : Comparable<L> {
            val head = this@buildList[0]
            fun g() = first() + x
            override fun compareTo(other: L) = x - other.x
        }
        add(plus(2))
        add(o.f() + o.p)
        add(o.g(5L).toInt())
        add(L(4).g() + L(1).head)
        add(L(3).compareTo(L(1)))
        add(same(1))
    }
    println(l)

    val b = build {
        put(2.5)
        val src = object : Source<Double> {
            override fun next() = peek() * 2
            override val last get() = peek()
        }
        class Impl : Base<Double>() {
            override fun make(seed: Double) = seed + peek()
            override fun twice(t: Double): List<Double> = listOf(t, peek())
        }
        fun local(d: Double = peek()) = d / 2
        put(src.next())
        put(src.last + 1)
        put(Impl().make(1.0))
        put(Impl().twice(7.0)[1])
        put(local())
        val viaBase: Base<Double> = Impl()
        put(viaBase.make(10.0))
        val viaSrc: Source<Double> = src
        put(viaSrc.next())
        put(viaSrc.last)
        val again = object : Source<Double> {
            override fun next() = peek()
            override val last = peek() + 100
        }
        val viaAgain: Source<Double> = again
        put(viaAgain.next() + viaAgain.last)
    }
    println(b.items)

    val s = buildSet {
        add('a')
        val cmp = object : Comparator<Char> {
            override fun compare(a: Char, b: Char) = b - a
        }
        val sorted = listOf('x', first(), 'm').sortedWith(cmp)
        addAll(sorted)
        class Holder(var c: Char = first()) { fun bump() { c = c + 1 } }
        val h = Holder()
        h.bump()
        add(h.c)
        fun String.tag() = this + first()
        add("q".tag()[1])
    }
    println(s)
}
