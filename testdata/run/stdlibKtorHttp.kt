// The constructs that ktor-http needed.

// A nullable value in a range written in place: the standard library's
// inline IntRange.contains(Int?), as a when condition and with in.
class DateBuilder { var year: Int? = null }

fun normalize(b: DateBuilder) {
    when (b.year) {
        in 70..99 -> b.year = b.year!! + 1900
        in 0..69 -> b.year = b.year!! + 2000
    }
}

fun letter(c: Char?) = c in 'a'..'z'

// Ranges of other types than Int and Char: a Double's is a
// ClosedFloatingPointRange, a Long and an Int give a LongRange, Shorts an
// IntRange.
fun quality(s: String?): Double = s?.toDoubleOrNull()?.takeIf { it in 0.0..1.0 } ?: 1.0

sealed class Span {
    class Bounded(val from: Long, val to: Long) : Span()
    class TailFrom(val from: Long) : Span()
    class Suffix(val lastCount: Long) : Span()
}

fun List<Span>.toLongRanges(length: Long) = map {
    when (it) {
        is Span.Bounded -> it.from..it.to.coerceAtMost(length - 1)
        is Span.TailFrom -> it.from until length
        is Span.Suffix -> (length - it.lastCount).coerceAtLeast(0L) until length
    }
}.filterNot { it.isEmpty() }

fun single(spans: List<Span>, length: Long): LongRange? {
    val mapped = spans.toLongRanges(length)
    if (mapped.isEmpty()) return null
    val start = mapped.minByOrNull { it.first }!!.first
    val end = mapped.maxByOrNull { it.last }!!.last.coerceAtMost(length - 1)
    return start..end
}

// String.plus called by name, as a safe call.
fun vary(h: String?): String = h?.plus(", gzip") ?: "gzip"

// Pair<String, Nothing?> and Pair<String, String> in lists join to
// List<Pair<String, String?>>.
fun Map<String, List<String>>.pairs(): List<Pair<String, String?>> =
    entries.flatMap { (key, value) -> if (value.isEmpty()) listOf(key to null) else value.map { key to it } }

// A labeled return's value whose type arguments come from the lambda's
// other result.
fun evens(xs: List<Int>) = xs.flatMap {
    if (it % 2 != 0) return@flatMap emptyList()
    listOf(it, it)
}

fun mimes(): List<Pair<String, Int>> =
    "a,1\n\n# note\nb,2".lineSequence().flatMapTo(ArrayList(4)) {
        val line = it.trim()
        if (line.isEmpty() || line.startsWith("#")) return@flatMapTo emptySequence()
        val (name, n) = line.split(",", limit = 2)
        sequenceOf(name to n.toInt())
    }

// A companion's initializers run once the companion exists: they can read
// it through the outer class.
class Status(val value: Int) {
    companion object {
        val OK = Status(200)
        val NotFound = Status(404)
        val all = listOf(Status.OK, Status.NotFound)
        private val byValue = all.associateBy { it.value }
        fun of(value: Int) = byValue[value]
    }
}

// An interface's, whose lambdas and local functions are the interface's
// methods.
interface Codes {
    companion object {
        val first = Status(1)
        val second = Codes.first.value + 1
        val third: Int by lazy { second + 1 }
        val fourth: Int

        init {
            fun next() = third + 1
            fourth = next()
        }
    }
}

// An init block sets a val directly: the val has no setter.
class Once {
    companion object {
        val bar: String

        init {
            bar = "OK"
        }
    }
}

// An extension of the program's comes before the standard library's of
// the same name, which kc calls as a static method of Character: as
// ktor's Char.isLowerCase, for which digits are lower case.
fun Char.isLowerCase(): Boolean = lowercaseChar() == this

fun String.uppercase(): String = "UP"

fun main() {
    println("a1.+-".all { it.isLowerCase() })
    println("x".uppercase())
    val b = DateBuilder()
    b.year = 80
    normalize(b)
    println(b.year)
    b.year = 5
    normalize(b)
    println(b.year)
    b.year = null
    normalize(b)
    println(b.year)
    println(b.year in 1..3000)
    println(letter('q'))
    println(letter(null))

    println(quality("0.5"))
    println(quality("2"))
    val n: Long = 9
    val i = 3
    val s: Short = 2
    println(i..n)
    println(n..i)
    println(s..s)
    println(0f..1f)
    println("a".."c")
    println(listOf(Span.Bounded(1, 100), Span.TailFrom(5), Span.Suffix(3)).toLongRanges(10))
    println(single(listOf(Span.Bounded(1, 100), Span.TailFrom(5)), 10))

    println(vary("deflate"))
    println(vary(null))

    println(mapOf("a" to listOf<String>(), "b" to listOf("1", "2")).pairs())
    println(evens(listOf(1, 2, 3, 4)))
    println(mimes())

    println(Status.all.size)
    println(Status.of(404)?.value)
    println(Codes.second)
    println(Codes.fourth)
    println(Once.bar)
}
