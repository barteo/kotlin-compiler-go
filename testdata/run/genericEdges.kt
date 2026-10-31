// Type parameters used as values: null checks, smart casts, equality, nested generic calls.
import java.util.ArrayList

class Cell<T>(var v: T) {
    fun self(): T = this.get()
    fun get(): T = v
    fun same(o: Cell<T>): Boolean = o.v == v
    fun <R> with(r: R): Pair2<T, R> = Pair2(v, r)
}
class Pair2<A, B>(val a: A, val b: B)

fun <T> describe(x: T): String {
    if (x != null) {
        return "non-null " + x.hashCode()
    }
    return "null"
}
fun <T> safeHash(x: T): Int? = x?.hashCode()
fun <T> orElse(x: T?, d: T): T = x ?: d
fun <T> eqOne(x: T): Boolean = x == 1
fun <T> nn(x: T): T = x!!
fun <T> listOf2(a: T, b: T): ArrayList<T> {
    val l = ArrayList<T>()
    l.add(a)
    l.add(b)
    return l
}
fun inferred() = listOf2(1, 2)

fun main() {
    println(describe(5))
    println(describe(null))
    println(safeHash("a"))
    println(safeHash(null))
    println(orElse(null, 3) + 1)
    println(eqOne(1))
    println(eqOne(2L))
    println(nn("s"))
    val c = Cell(10)
    println(c.self() + c.get())
    println(c.same(Cell(10)))
    val p = c.with("r")
    println(p.a + p.b.length)
    println(inferred().get(1) * 10)
    val mixed = listOf2(1, 2.5)
    println(mixed.get(0))
    val strs = listOf2("x", null)
    println(strs.get(1))
    var total = 0L
    total += listOf2(1L, 2L).get(1)
    println(total)
    val longs = ArrayList<Long>()
    longs.add(3)
    println(longs.get(0) + 1)
}
