fun <T> b1(f: MutableList<T>.() -> T): T = mutableListOf<T>().f()
fun <T> b3(f: MutableList<T>.() -> List<T>): List<T> = mutableListOf<T>().f()
fun <T> b4(f: MutableList<T>.() -> (T) -> Int): Int { val l = mutableListOf<T>(); val g = l.f(); return g(l[0]) }
fun <T> b5(f: MutableList<T>.() -> T?): T? = mutableListOf<T>().f()
fun <T> b6(f: MutableList<T>.() -> Comparable<T>): Comparable<T> = mutableListOf<T>().f()
fun interface Sink<T> { fun put(t: T): Int }
fun <T> b7(f: MutableList<T>.() -> Sink<T>): Int { val l = mutableListOf<T>(); val g = l.f(); return g.put(l[0]) }

fun main() {
    val a = b1 { add(1L); 2 }
    println(a)
    val c = b3 { add("x"); emptyList() }
    println(c)
    val c2 = b3 { add("x"); if (size > 1) emptyList() else listOf("y") }
    println(c2)
    val d = b4 { add("abc"); { it.length } }
    println(d)
    val e = b5 { add(3); null }
    println(e)
    val e2 = b5 { add(3); if (isEmpty()) null else get(0) }
    println(e2)
    val g = b6 { add("k"); "z" }
    println(g.compareTo("a") > 0)
    val h = b7 { add("abcd"); Sink { it.length } }
    println(h)
    val i = b1 { add(1.5); return@b1 2.5 }
    println(i)
    val j = b1 { add('c'); if (isEmpty()) 'd' else get(0) }
    println(j)
}
