// Expressions on a builder's elements, whose type is the builder's
// variable while its lambda is checked: operators, comparisons, safe
// calls, casts, branches, loops, destructuring, indexing and lambdas
// are compiled for the type it is inferred as.

data class P(val a: Int, val b: String)

class Box<T>(var v: T) {
    operator fun plus(o: Box<T>) = Box(o.v)
    operator fun unaryMinus() = this
    operator fun compareTo(o: Box<T>) = 0
    operator fun get(i: Int) = v
    operator fun contains(x: T) = x == v
    override fun toString() = "Box($v)"
}

fun main() {
    println(buildList { add(2); add(get(0) * 3 - 1); add(get(1) % 4); add(get(0) shl 2) })
    println(buildList { add(1.5); add(get(0) / 3); add(-get(0)); add(get(0) + 1) })
    println(buildList { add("a"); add(get(0) + 1 + 'c'); add("${get(0)}!") })
    println(buildList { add('x'); add(get(0) + 2); println(get(0) < 'y'); println(get(1) - get(0)) })
    println(buildList { add(Box(1)); add(get(0) + Box(2)); add(-get(0)); println(get(0) < get(1)); println(get(0)[0] + 1); println(1 in get(0)) })
    println(buildList { add("x"); add(null); println(get(1)?.length); println(get(0)?.length); println(get(1) ?: "none") })
    println(buildList<Any> { add(1); add("s"); val x = get(1); if (x is String) println(x.length); println((get(0) as Int) + 1) })
    println(buildList { add(1); val f = { x: Int -> x + get(0) }; add(f(2)); run { add(get(1) + 1) } })
    println(buildList { add(1); listOf(5, 6).forEach { if (it == 6) return@forEach; add(it + get(0)) } })
    println(buildMap { put("a", 1); put("b", getValue("a") + 1); for ((k, v) in this) println(k + v); println(this["a"]!! + 1) })
    println(buildString { append(1); append(length + 1); append(this[0] + 1) })
    println(sequence { yield(1); yield(2) }.map { it + 1 }.toList())
}
