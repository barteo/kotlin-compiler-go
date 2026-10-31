// The deprecated inline class is a @JvmInline value class.

interface Measured {
    val size: Int
}

inline class Meters(val value: Int) : Measured {
    operator fun plus(o: Meters) = Meters(value + o.value)
    override val size: Int get() = value
    override fun toString() = "${value}m"
}

private inline class Name(private val s: String) {
    val length get() = s.length
}

inline class Box<T>(val item: T)

inline class Wrapped(val x: Long)

fun total(xs: List<Meters>): Meters = xs.fold(Meters(0)) { a, b -> a + b }

fun sizeOf(m: Measured) = m.size

fun main() {
    val m = Meters(3) + Meters(4)
    println(m)
    println(total(listOf(Meters(1), Meters(2))))
    val any: Any = m
    println(any is Meters)
    println(sizeOf(m))
    println(Name("abc").length)
    val n: Meters? = if (m.value > 0) m else null
    println(n?.value)
    println(Box("box").item)
    println(Wrapped(5L))
    println(Meters::class.java.isAnnotationPresent(JvmInline::class.java))
}
