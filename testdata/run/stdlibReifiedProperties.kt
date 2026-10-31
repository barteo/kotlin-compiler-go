// Extension properties with reified type parameters, whose inline getters
// reads copy, with the read's type arguments: on a receiver, a nullable
// one with ?., an implicit one, and one of a reified type parameter of
// the inline function the read is copied into. Writes copy the setter,
// and compound ones both.
val <reified T> T.kind: String
    inline get() = T::class.simpleName + (if (this is CharSequence) " chars" else "")

inline val <reified T : Any> List<T>.elementKind: String
    get() = "${T::class.simpleName} x$size"

inline val <reified T> T.self: T
    get() = (this as Any) as T

val counts = HashMap<String, Int>()

inline var <reified T> T.count: Int
    get() = counts[T::class.simpleName!!] ?: 0
    set(v) {
        counts[T::class.simpleName!!] = v
    }

inline fun <reified T> describe(x: T) = "describe " + x.kind

fun apply(f: (String) -> String) = f("ref")

fun main() {
    println("abc".kind)
    println(42.kind)
    println(listOf(1.5, 2.5).elementKind)
    val s: String? = null
    println(s?.kind)
    println("x".let { it.kind })
    with(3L) { println(kind) }
    println(describe('c'))
    println(apply(String::self))

    "a".count = 5
    "b".count += 2
    1.count++
    with(2) { count += 10 }
    println(counts.toSortedMap())
}
