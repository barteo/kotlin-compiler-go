// Joins of branches whose values or locals differ, which need StackMapTable
// frames with the right types.

import java.math.BigInteger
import java.util.AbstractList
import java.util.ArrayList
import java.util.LinkedList

class Box(val label: String)

class Sign(n: Int) {
    val shape: Any? = if (n < 0) StringBuilder("negative") else BigInteger.valueOf(n.toLong())
}

// The branches join to Comparable<*>, an interface.
fun pick(c: Boolean) = if (c) StringBuilder("text") else BigInteger.valueOf(42)

// The branches join to AbstractList<String>, a class.
fun list(c: Boolean): AbstractList<String> = if (c) ArrayList<String>() else LinkedList<String>()

fun mixed(n: Int): Any? = when (n) {
    0 -> StringBuilder("zero")
    1 -> BigInteger.ONE
    2 -> list(true)
    3 -> null
    else -> Box("box").label
}

fun fallback(s: StringBuilder?): Any? = s ?: BigInteger.valueOf(7)

fun lengthOrNull(s: String?): Int? = s?.length

fun builder(c: Boolean): String = StringBuilder(if (c) "yes" else "no").append("!").toString()

fun pair(a: Any?, b: Any?): String = "$a/$b"

fun nested(n: Int): Int {
    var total = 0
    for (i in 0 until n) {
        if (i == 3) continue
        var j = 0
        while (true) {
            val square = i * j
            if (j > i) break
            total += square
            j++
        }
        if (total > 100) break
    }
    return total
}

fun wide(flag: Boolean): Double {
    var big = 1L
    var d = 0.5
    if (flag) {
        big = big * 1000000000000L
        d = d * 3
    } else {
        val extra = 2L
        big = big + extra
    }
    return big.toDouble() + d
}

fun firstPositive(a: Int?, b: Int?): Int {
    val x = a ?: return -1
    if (x > 0) return x
    val y = b ?: return 0
    return y
}

fun counter(n: Int): Int {
    var count = 0
    val add: (Int) -> Unit = { count += it }
    var i = 0
    while (i < n) {
        add(i)
        if (count > 5) {
            count = count * 2
        }
        i++
    }
    return count
}

fun main() {
    println(pick(true))
    println(pick(false))
    for (n in 0..4) {
        println(mixed(n))
    }
    println(fallback(null))
    println(fallback(StringBuilder("s")))
    val l = list(false)
    l.add("x")
    println(l)
    println(lengthOrNull("four"))
    println(lengthOrNull(null))
    println(builder(true))
    println(builder(false))
    println(pair(if (lengthOrNull("ab") == 2) StringBuilder("1") else BigInteger.TEN, if (lengthOrNull(null) == null) list(true) else Box("b")))
    println(nested(10))
    println(wide(true))
    println(wide(false))
    println(firstPositive(5, 1))
    println(firstPositive(null, 1))
    println(firstPositive(-2, null))
    println(firstPositive(-2, 9))
    println(counter(5))
    println(Sign(-1).shape)
    println(Sign(3).shape)
    val box: Box? = if (counter(1) == 0) Box("maybe") else null
    println(box?.label ?: "none")
}
