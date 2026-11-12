class Response(val size: Int?, val headers: Long?)

fun total(r: Response?): Long = (r?.size ?: 0) + (r?.headers ?: 0)

fun orZero(l: Long?): Long = l ?: 0

fun plusOne(l: Long?) = 1 + (l ?: -1)

fun shorts(s: Short?) = (s ?: 2) * 3

fun main() {
    println(total(Response(3, 4)))
    println(total(null))
    println(orZero(null))
    println(plusOne(5))
    println(plusOne(null))
    println(shorts(null))
}
