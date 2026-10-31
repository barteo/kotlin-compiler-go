package shapes

// A library whose declarations need Kotlin metadata to be used from
// Kotlin: nullability, default and named arguments, extensions, data,
// enum, sealed and value classes, objects, generics, operators and
// deprecation.

sealed class Shape {
    abstract val area: Double
    data class Circle(val radius: Double) : Shape() {
        override val area: Double get() = 3.0 * radius * radius
    }
    data class Rect(val w: Double, val h: Double = w) : Shape() {
        override val area: Double get() = w * h
    }
    object Empty : Shape() {
        override val area: Double get() = 0.0
    }
}

enum class Color(val hex: String) { RED("#f00"), GREEN("#0f0"), BLUE("#00f") }

@JvmInline
value class Meters(val value: Double) {
    operator fun plus(other: Meters): Meters = Meters(value + other.value)
    fun scaled(by: Double = 2.0, unit: String = "m") = "" + value * by + unit
    inline fun <R> fold(f: (Double) -> R, x: Double = value): R = f(x)
}

class Box<out T>(val item: T) {
    fun <R> map(f: (T) -> R): Box<R> = Box(f(item))
    operator fun component1(): T = item
}

interface Named {
    val name: String
    fun greet(greeting: String = "Hello"): String = greeting + ", " + name
}

class Person(override val name: String, val nick: String? = null) : Named {
    var visits: Int = 0
    operator fun get(i: Int): Char = name[i]
    operator fun invoke(times: Int): String = name.repeat(times)

    @Deprecated("use greet", level = DeprecationLevel.HIDDEN)
    fun hello(): String = "hidden"

    companion object {
        const val SPECIES = "human"
        @JvmStatic fun of(name: String): Person = Person(name)
    }
}

object Registry {
    private val items = mutableListOf<String>()
    fun add(vararg names: String): Int {
        items.addAll(names)
        return items.size
    }
}

fun describe(s: Shape): String = when (s) {
    is Shape.Circle -> "circle " + s.radius
    is Shape.Rect -> "rect " + s.w + "x" + s.h
    Shape.Empty -> "empty"
}

fun format(value: Double, prefix: String = "", suffix: String = ""): String = prefix + value + suffix

fun String.shout(): String = uppercase() + "!"

val String.initial: Char get() = this[0]

var counter: Int = 0

fun lengthOf(s: String?): Int = s?.length ?: -1

internal fun secret(): String = "internal"

// Several bounds: T erases to Number, the class among them.
class Ranked<T>(val value: T) where T : Comparable<T>, T : Number {
    fun higher(other: T): T = if (value >= other) value else other
}

fun <T> largest(xs: List<T>): T? where T : CharSequence, T : Comparable<T> = xs.maxOrNull()

// A fun interface: an app compiled by kotlinc converts lambdas to it
// because the metadata says so.
fun interface Transform {
    fun apply(s: String): String
}

fun transform(s: String, t: Transform): String = t.apply(s)
