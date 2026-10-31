val answer = 42
var counter: Long = 0

fun square(x: Int) = x * x
fun average(a: Double, b: Double): Double = (a + b) / 2
fun mixed(i: Int, l: Long, d: Double) = i + l + d

fun main() {
    println(square(7))
    println(7 / 2)
    println(-7 % 3)
    println(7.0 / 2)
    println(average(1.5, 2.0))
    println(mixed(1, 2L, 0.5))
    println(10000000000)
    println(2147483647 + 1)
    println(0x7F + 0b101)
    println(1_000_000 * 3L)
    val big: Long = 5
    println(big * 1000000000)
    counter += 3
    counter *= answer.toLong()
    println(counter)
    var x = 10
    x -= 3
    x *= 2
    x /= 3
    x %= 3
    x++
    println(x)
    var d = 1.0
    d += 0.25
    d--
    println(d)
    println(1.0 / 0.0)
    println(-(3.5))
    println(answer.toDouble() / 4)
    println((7.9).toInt())
    println(123456789012L.toInt())
}
