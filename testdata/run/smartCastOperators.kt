// Operators on operands that a smart cast narrows, and on nullable ones.

class Box {
    var s: String? = null

    fun add(t: String) {
        if (s == null) {
            s = t
        } else {
            s += " $t" // String?.plus
        }
    }
}

fun incrementSmartCast() {
    var i: Int?
    i = 10
    val a: Int = i++ // the old value, an Int
    val b: Int = ++i
    i += 5
    i -= 1
    i *= 2
    println("$a $b $i")

    var s: Short? = 1
    if (s != null) s++
    var y: Byte? = 0
    if (y != null) y--
    var l: Long? = 7L
    if (l != null) {
        l += 3
        l--
    }
    var d: Double? = 0.5
    if (d != null) d++
    var c: Char? = 'a'
    if (c != null) c++
    println("$s $y $l $d $c")
}

fun compoundOnAny(x: Any?): Any? {
    var p: Any? = x
    if (p is String) {
        p += "K" // p = p + "K"
    }
    if (p is Int) {
        p += 1
    }
    return p
}

fun nullableString(n: Int): String? {
    var s: String? = null
    for (i in 0 until n) s += "ab"
    return s
}

operator fun Int?.inc(): Int? = if (this == null) 0 else this + 1

private operator fun Long?.compareTo(other: Long?): Int = (this ?: 0L).compareTo(other ?: 0L)

operator fun Int.compareTo(c: Char): Int = compareTo(c.code)

class Counter {
    operator fun Int?.plus(n: Int): Int = (this ?: 100) + n

    fun add(x: Int?) = x + 1
}

fun nullableOperands() {
    var i: Int? = null
    i++ // Int?.inc()
    val j = i++
    println("$i $j")
    val a: Long? = null
    val b: Long? = 3L
    println("${a < b} ${a >= b}")
    println("${66 < 'A'} ${64 < 'A'}") // Int.compareTo(Char)
    println("${Counter().add(null)} ${Counter().add(1)}")
    println(null + "!")
    val n: String? = null
    println(n + null)
}

annotation class Tag1
annotation class Tag2

fun eqDI(x: Any?, y: Any?) = x is Double? && y is Int? && x == y
fun eqLF(x: Any?, y: Any?) = x is Long? && y is Float? && x == y
fun neDF(x: Any, y: Any) = x is Double && y is Float && x != y

val minusZero: Any = -0.0

fun equality() {
    println("${eqDI(null, null)} ${eqDI(null, 0)} ${eqDI(0.0, null)} ${eqDI(-0.0, 0)} ${eqDI(0.5, 0)}")
    println("${eqLF(3L, 3.0F)} ${eqLF(0L, -0.0F)} ${eqLF(1L, null)}")
    println("${neDF(0.0, -0.0F)} ${neDF(1.0, 2.0F)}")
    val x: Any = 1
    val y: Any = 1L
    if (x is Int && y is Long) println("${x == y} ${x != y}")
    if (x is Int) println("${x == 1L} ${x == 2.0}")
    val s: Any = "a"
    if (s is String && x is Int) println(s == x)
    println(Tag1() == Tag2())
    if (minusZero is Double) {
        println("${minusZero == 0.0F} ${minusZero == (0.0F as Comparable<Float>)}")
    }
    val d = 1.5
    println(d == (1.5 as Comparable<Double>))
}

// A loop that only increments, decrements or assigns a smart-cast var with
// op= keeps its smart cast.
fun loops() {
    var i: Int? = 1
    if (i != null) {
        while (i != 10) {
            i++
        }
        val n: Int = i
        print("$n ")
    }
    var a: Any = 'a'
    if (a is Char) {
        do {
            ++a
        } while (a < 'e')
        val c: Char = a
        print("$c ")
    }
    var b: Byte? = 3
    var steps = 0
    if (b != null) {
        for (k in 1..3) {
            b--
            steps += b
        }
        val r: Byte = b
        print("$r $steps ")
    }
    var l: Long? = 1L
    if (l != null) {
        while (l < 100) {
            l *= 3
            l -= 1
        }
        val m: Long = l
        println(m)
    }
}

fun main() {
    incrementSmartCast()
    loops()
    nullableOperands()
    equality()
    println(compoundOnAny("O"))
    println(compoundOnAny(41))
    println(compoundOnAny(null))
    println(nullableString(0))
    println(nullableString(2))
    val box = Box()
    box.add("a")
    box.add("b")
    println(box.s)
}
