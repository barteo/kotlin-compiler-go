// An integer literal passed for a builder's variable is of the type the
// variable is inferred as, from the expected type or the other calls.

fun names(xs: List<Any?>) = xs.map { it?.javaClass?.simpleName }

fun main() {
    val a: List<Long> = buildList { add(1); add(2) }
    println(names(a))
    val c: List<Any> = buildList { add(1) }
    println(names(c))
    val d: List<Byte> = buildList { add(1); add(-2) }
    println(names(d))
    val e = buildList { add(1); add("x") }
    println(names(e))
    val g: Map<Short, Long?> = buildMap { put(1, 2); put(3, null) }
    println(names(g.keys.toList()) + names(g.values.toList()))
    val h = buildSet { add(1); add(2.5) }
    println(names(h.toList()))
    val v1 = buildList { add(1); add(get(0) + 1) }
    println(names(v1))
    val v2 = buildList { add(1L); add(get(0) * 2) }
    println(names(v2))
    val v3 = buildList { add('a'); add(get(0) + 1) }
    println(names(v3))
    val v4 = buildList { add(1); add(2); println(get(0) < 2); println(get(1) == 2) }
    println(names(v4))
}
