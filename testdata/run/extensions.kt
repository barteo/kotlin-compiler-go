// Extension functions: on Kotlin's types, the program's classes and Java
// classes; nullable and generic receivers; default and named arguments;
// infix extensions; implicit receivers in their bodies; and how calls
// choose between members and extensions.

open class Shape(val name: String) {
    fun area() = 0
}

class Square(val side: Int) : Shape("square")

interface Named {
    val label: String
}

class Dog(override val label: String) : Named

class Vec(val x: Int, val y: Int)

fun String.shout(): String = this + "!"

fun String.exclaim(n: Int = 2): String {
    var s = this
    for (i in 0 until n) {
        s = s.shout() // an extension calling another one
    }
    return s
}

fun Int.twice() = this * 2
fun Int.isEven(): Boolean = this % 2 == 0
fun Long.half(): Long = this / 2L
fun Double.squared() = this * this
fun Char.repeat3(): String = "$this$this$this"
fun Boolean.toYesNo() = if (this) "yes" else "no"

// Members and extensions of the receiver are visible unqualified.
fun Vec.len2(): Int = x * x + y * y
fun Vec.plusX(d: Int) = Vec(x + d, y)
fun Vec.describe() = "(${this@describe.x}, $y) len2=${len2()}"
fun Vec.scaledBy(k: Int): Vec {
    val g = { v: Int -> v * k } // a lambda capturing the receiver's values
    return Vec(g(x), g(this.y))
}
fun Vec.lazyLen(): () -> Int = { len2() } // a lambda capturing the receiver

// A member wins over an extension with the same signature.
fun Shape.area() = -1
fun Shape.kind() = "shape " + name
fun Square.kind() = "square of side $side" // more specific receiver
fun Any.kind() = "something"

// Receivers of interface types, Any, and Java classes.
fun Named.greet() = "hello, $label"
fun Any?.describeNull(): String = if (this == null) "null!" else "value " + toString()
fun StringBuilder.line(s: String): StringBuilder = append(s).append('\n')

// Nullable receivers: called without ?., this may be null.
fun String?.orDash(): String = this ?: "-"
fun String?.lengthOrZero(): Int {
    if (this == null) return 0
    return length // smart cast of this
}

// Generic receivers.
fun <T> Array<T>.second(): T = this[1]
fun <T : Comparable<T>> T.clamp(lo: T, hi: T): T = if (this < lo) lo else if (this > hi) hi else this
fun <T> T.alsoPrint(f: (T) -> String): T {
    println(f(this))
    return this
}

// Overloads by receiver type.
fun Int.show() = "Int $this"
fun String.show() = "String $this"

// Function types as receivers.
fun (() -> Unit).twice() {
    this()
    this()
}
fun ((Int) -> Int).then(g: (Int) -> Int): (Int) -> Int = { x -> g(this(x)) }

infix fun Int.pow(e: Int): Int {
    var r = 1
    for (i in 0 until e) r *= this
    return r
}

fun Int.applyTwice(f: (Int) -> Int): Int = f(f(this))

fun Int.factorial(): Long = if (this <= 1) 1L else this * (this - 1).factorial()

// An extension with the name of the standard library's: the program's
// own declarations come first.
fun String.reversed(): String = "mine"

// Operator extensions: indexing and destructuring.
class Pair2(val a: Int, val b: Int)
operator fun Pair2.get(i: Int) = if (i == 0) a else b
operator fun Pair2.component1() = a
operator fun Pair2.component2() = b

// Extensions called on implicit receivers: a class's this, and the receiver
// of a lambda with a receiver.
class Counter(var n: Int) {
    fun bumpTwice(): Int {
        bump()
        bump()
        return n
    }
}
fun Counter.bump() {
    n += 1
}
fun build(f: StringBuilder.() -> Unit): String {
    val sb = StringBuilder()
    sb.f()
    return sb.toString()
}

fun main() {
    println("hi".shout())
    println("hi".exclaim())
    println("hi".exclaim(n = 3))
    println(21.twice())
    println(4.isEven())
    println(10L.half())
    println(1.5.squared())
    println('z'.repeat3())
    println(true.toYesNo())

    val v = Vec(3, 4)
    println(v.len2())
    println(Vec(1, 2).plusX(5).x)
    println(Vec(1, 2).describe())
    println(Vec(1, 2).scaledBy(3).describe())
    println(v.lazyLen()())

    println(Shape("s").area())
    println(Shape("circle").kind())
    println(Square(2).kind())
    val s: Shape = Square(3)
    println(s.kind()) // chosen by the static type
    println(5.kind())
    println(Dog("rex").greet())
    println(null.describeNull())
    println(42.describeNull())
    val sb = StringBuilder()
    sb.line("one").line("two")
    print(sb)

    val n: String? = null
    println(n.orDash())
    println("x".orDash())
    println(n.lengthOrZero())
    println("four".lengthOrZero())
    println(n?.shout())
    println("y"?.shout())

    println(arrayOf("a", "b", "c").second())
    println(15.clamp(0, 10))
    println("m".clamp("a", "k"))
    val w = Vec(7, 8).alsoPrint { "vec " + it.x }
    println(w.y)

    println(7.show())
    println("seven".show())

    var count = 0
    val inc = { count += 1 }
    inc.twice()
    println(count)
    val addThenDouble = { x: Int -> x + 1 }.then { it * 2 }
    println(addThenDouble(4))

    println(2 pow 10)
    println(2.pow(3))
    println(3.applyTwice { it * 10 })
    println(20.factorial())
    println("abc".reversed())

    val pr = Pair2(3, 4)
    println(pr[0] + pr[1])
    val (pa, pb) = pr
    println(pa * pb)
    println(Counter(5).bumpTwice())
    print(build { line("built") })
}
