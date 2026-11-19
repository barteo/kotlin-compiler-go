// The warnings kotlinc 2.4.21 reports of the divisions below, as
// TestWarningsDivision reads them: each line ends with a comment naming
// the warnings it reports, if any.
import kotlin.time.Duration
class Foo { operator fun div(x: Int) = this }
operator fun String.div(x: Int) = this
fun f(x: Int, d: Double, u: UInt, l: Long, dur: Duration, foo: Foo, b: Byte, s: Short, c: Char) {
    println(x / -0) // DIVISION_BY_ZERO
    println(x / (0)) // DIVISION_BY_ZERO
    println(x / 0L) // DIVISION_BY_ZERO
    println(x / 0x0) // DIVISION_BY_ZERO
    println(x / 0b0) // DIVISION_BY_ZERO
    println(u / 0u) // DIVISION_BY_ZERO
    println(d / -0.0)
    println(d / 0.0e0) // DIVISION_BY_ZERO
    println(d / 0.00) // DIVISION_BY_ZERO
    println(d / 0e5) // DIVISION_BY_ZERO
    println(d / 0.0F) // DIVISION_BY_ZERO
    println(l / 0) // DIVISION_BY_ZERO
    println(dur / 0)
    println(foo / 0)
    println("a" / 0)
    println(b / 0) // DIVISION_BY_ZERO
    println(s / 0) // DIVISION_BY_ZERO
    println(x.div(0)) // DIVISION_BY_ZERO
    println(x?.div(0)) // DIVISION_BY_ZERO UNNECESSARY_SAFE_CALL
    println(x.div(other = 0)) // DIVISION_BY_ZERO
    println(x % 0)
    println(x / (-0)) // DIVISION_BY_ZERO
    println(x / +0) // DIVISION_BY_ZERO
    println(d / -0) // DIVISION_BY_ZERO
    var y = 1
    y /= 0 // DIVISION_BY_ZERO
    y = y / 0 // DIVISION_BY_ZERO
    var z = 1.0
    z /= 0.0 // DIVISION_BY_ZERO
    println(x / 0 / 0) // DIVISION_BY_ZERO
    println(x / 0.toInt())
    println(x / @Suppress("X") 0) // DIVISION_BY_ZERO
    println(x / '\u0000'.code)
    println(y + z)
}
val L = 0.div(0) // DIVISION_BY_ZERO
fun g(x: Int, d: Double) {
    println(x / -(0)) // DIVISION_BY_ZERO
    println(d / +0.0)
    println(d / (0.0)) // DIVISION_BY_ZERO
    println(x / -0L)
    println(x / 0.0f) // DIVISION_BY_ZERO
    println(x.div(-0)) // DIVISION_BY_ZERO
    println(x.rem(0))
    println(d % 0.0)
}
fun h(l: Long, d: Double, f: Float) {
    println(l / -0) // DIVISION_BY_ZERO
    println(l / -0L)
    println(d / -0.0F)
    println(f / -0.0)
    println(d / 0.0 / 1) // DIVISION_BY_ZERO
    println(l.div(0L)) // DIVISION_BY_ZERO
    println(d.div(0.0)) // DIVISION_BY_ZERO
    println(l / +0L)
}
