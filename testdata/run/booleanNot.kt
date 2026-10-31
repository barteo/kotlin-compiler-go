// Boolean's not() is !x, as a call.
fun flip(b: Boolean) = b.not()

fun main() {
    println(flip(true))
    println(flip(false))
    val ok = 1 > 2
    println(ok.not().and(true))
    println((ok || true).not())
    val maybe: Boolean? = if (flip(true)) true else null
    println(maybe?.not())
}
