// Operators with the standard library: its operator extensions (BigInteger
// arithmetic, contains, plusAssign), in on collections, strings and
// arrays, and compound assignment of list elements.

import java.math.BigInteger
import java.math.BigDecimal

data class Point(val x: Int, val y: Int)

operator fun Point.plus(o: Point) = Point(x + o.x, y + o.y)

fun main() {
    val a = BigInteger.valueOf(2)
    val b = BigInteger("10")
    println(a + b)
    println(b - a)
    println(a * b * b)
    println(b / a)
    println(b % BigInteger.valueOf(3))
    println(-a)
    println(a < b)
    println(a >= b)
    var big = BigInteger.ONE
    for (i in 1..25) big *= BigInteger.valueOf(i.toLong())
    println(big)
    println(BigDecimal("1.5") + BigDecimal("2.25"))

    val xs = listOf(1, 2, 3)
    println(2 in xs)
    println(5 in xs)
    println(5 !in xs)
    println("ell" in "hello")
    println('z' in "hello")
    val arr = arrayOf("a", "b")
    println("b" in arr)
    val m = mapOf("k" to 1)
    println("k" in m)
    when (3) {
        in xs -> println("found")
        else -> println("missing")
    }

    val ys = mutableListOf(10, 20)
    ys += 30
    ys -= 10
    ys[0] += 5
    ys[1]++
    println(ys)
    val names = mutableListOf("x")
    names[0] += "yz"
    println(names)
    val counts = mutableMapOf("a" to 1)
    counts["a"] = counts.getValue("a") + 1
    println(counts)

    var p = Point(1, 1)
    p += Point(2, 3)
    println(p)
    val points = mutableListOf(Point(0, 0))
    points[0] += Point(5, 5)
    println(points)
}
