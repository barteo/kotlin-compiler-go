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

fun main() {
    incrementSmartCast()
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
