package d

annotation class Platform(val name: String = "")

// Expect declarations without actual ones: a processor generates those.
@Platform("clock")
expect class Clock(zone: String) {
    val zone: String
    fun now(): Long
    companion object {
        fun system(): Clock
    }
}

expect interface Feed {
    fun read(): String
}

expect object Registry : Feed {
    override fun read(): String
}

@Platform
expect fun clock(zone: String = "UTC"): Clock

expect val defaultClock: Clock

class Timer(val clock: Clock, val feed: Feed = Registry) {
    fun start(): Long = clock.now()
}

expect class Bare {
    fun f()
}

expect class Made {
    constructor(n: Int)
    var n: Int
}

expect annotation class Tag(val text: String)

// Expect declarations with actual ones in Actuals.kt: both are declarations,
// and a class's name is the first of the two.
@Platform("expect")
expect class Paired(n: Int = 1) {
    val n: Int
    val String.width: Int
    fun f(a: Int = 2): String
    class Nested {
        fun g()
    }
}

expect interface Port {
    fun read(): String
}

expect object Hub : Port {
    override fun read(): String
}

expect class Text

expect annotation class Note(val text: String = "")

expect value class Key(val raw: String)

@Platform("expect")
expect fun paired(a: Int = 1): Paired

// Overloads: three have an actual one.
expect fun over(a: Int): Int
expect fun over(a: String): Int
expect fun String.over(a: Int): Int
expect fun <T> over(a: T, b: List<T>): T
expect fun over(a: Long, b: Int): Int

@Platform("expect")
expect val pairedVal: Paired
expect val String.pairedVal: Int
expect val Int.pairedVal: Int
expect var <T> List<T>.pairedSize: Int
expect val lazily: Int

class Uses(val p: Paired, val n: Paired.Nested, val port: Port = Hub, val text: Text, val key: Key)

@Note
fun noted(text: Text): Text = text

// The actual declaration first, and second.
actual fun early(): Int = 1
expect fun early(): Int

expect class Late
actual class Late

actual class Soon
expect class Soon
