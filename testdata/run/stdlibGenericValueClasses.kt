// Generic value classes, and kotlin.Number's conversions and @UnsafeVariance,
// which they use.

@JvmInline
value class Box<T>(val value: T) {
    fun get(): T = value
    fun <R> map(f: (T) -> R): Box<R> = Box(f(value))
    override fun toString() = "Box($value)"
}

@JvmInline
value class Nel<out A>(val all: List<A>) : Iterable<A> {
    val head: A get() = all.first()
    val size: Int get() = all.size
    override fun iterator(): Iterator<A> = all.iterator()
    operator fun plus(a: @UnsafeVariance A): Nel<A> = Nel(all + a)
}

@JvmInline
value class Result2<out T>(private val holder: Any?) {
    val isFailure: Boolean get() = holder is Failed
    fun getOrNull(): T? = if (holder is Failed) null else holder as T
    override fun toString() = if (holder is Failed) "Failed(${holder.cause})" else "Value($holder)"
    internal class Failed(@JvmField val cause: String) {
        override fun equals(other: Any?) = other is Failed && other.cause == cause
        override fun hashCode() = cause.hashCode()
    }
    companion object {
        fun <E> success(value: E): Result2<E> = Result2(value)
        fun <E> failure(cause: String): Result2<E> = Result2(Failed(cause))
    }
}

@JvmInline
value class Num<N : Number>(val n: N) {
    fun twice() = n.toDouble() * 2
}

fun <T> unwrap(b: Box<T>): T = b.value
fun sumBoxes(bs: List<Box<Int>>) = bs.sumOf { it.value }
fun nullableBox(b: Box<String>?) = b?.value ?: "none"
fun takeNel(n: Nel<String>) = n.head + n.size

@JvmInline
value class Id<T : Any>(val raw: T)

fun idOrNull(i: Id<String>?) = i?.raw ?: "no id"

fun <N : Number> widen(n: N) = n.toLong() + n.toInt() + n.toShort() + n.toByte()

class Bag<out E>(private val items: List<E>) {
    fun has(e: @UnsafeVariance E) = e in items
    fun with(e: @UnsafeVariance E): Bag<E> = Bag(items + e)
    fun size() = items.size
}

fun main() {
    val b = Box(41)
    println(b.value + 1)
    println(b.get() + 1)
    println(b.map { it.toString() + "!" })
    println(unwrap(Box("s")))
    println(sumBoxes(listOf(Box(1), Box(2))))
    println(nullableBox(null) + " " + nullableBox(Box("x")))
    val any: Any = b
    println(any is Box<*>)
    println(b == Box(41))
    println(Box("a").hashCode() == Box("a").hashCode())
    val n = Nel(listOf("a", "b")) + "c"
    println(n.head + " " + n.size + " " + n.toList())
    println(takeNel(n))
    for (s in n) print(s)
    println()
    val r = Result2.success(3)
    val f = Result2.failure<Int>("bad")
    println((r.getOrNull() ?: 0) + 1)
    println(f.isFailure.toString() + " " + f.getOrNull() + " " + f)
    println(Num(2.5).twice() + Num(3).twice())
    val list = listOf(Box(1), Box(2))
    println(list.map { it.value * 10 })
    println(idOrNull(Id("x")) + " " + idOrNull(null))
    val ids: List<Id<Int>> = listOf(Id(1), Id(2))
    println(ids.map { it.raw + 1 })
    println(widen(300) + widen(2.9))
    val bag = Bag(listOf("a")).with("b")
    println(bag.has("b").toString() + " " + bag.size())
}
