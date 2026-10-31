// The built-in types' companion objects, enumValues and enumValueOf, and
// kotlin.Annotation.

enum class Level { LOW, MEDIUM, HIGH }

inline fun <reified T : Enum<T>> Collection<T>.toEnumSet(): Set<T> =
    enumValues<T>().filterTo(mutableSetOf(), this::contains)

inline fun <reified T : Enum<T>> parse(name: String): T = enumValueOf<T>(name)

fun annotationName(a: Annotation?): String = a.toString()

fun Int.Companion.max() = MAX_VALUE
fun Long.Companion.min() = this.MIN_VALUE

fun main() {
    println(Int.MAX_VALUE)
    println(Long.MIN_VALUE)
    println(Short.MAX_VALUE)
    println(Byte.MIN_VALUE)
    println(Char.MAX_VALUE.code)
    println(Double.NaN.isNaN())
    println(Float.POSITIVE_INFINITY)
    println(Int.SIZE_BITS + Long.SIZE_BYTES)

    println(String.format("%s=%d", "x", 3))
    println(listOf("b", "A", "c").sortedWith(String.CASE_INSENSITIVE_ORDER))

    println(listOf(Level.HIGH, Level.LOW).toEnumSet())
    println(enumValues<Level>().size)
    println(parse<Level>("MEDIUM").ordinal)
    println(enumValueOf<Level>("HIGH"))

    println(annotationName(null))

    println(Int.max())
    println(Long.Companion.min())
    println(Int.Companion === Int)
}
