// Overloads that differ in a parameter of number type, none a subtype of
// another, take null: the more specific number type wins.
class Pick {
    operator fun invoke(a: Long?, b: Long?) = "long $a $b"
    operator fun invoke(a: Int?, b: Long?) = "int $a $b"
    fun ib(x: Int?) = "Int"
    fun ib(x: Byte?) = "Byte"
    fun is_(x: Int?) = "Int"
    fun is_(x: Short?) = "Short"
    fun sb(x: Short?) = "Short"
    fun sb(x: Byte?) = "Byte"
    fun df(x: Double?) = "Double"
    fun df(x: Float?) = "Float"
}

fun main() {
    val p = Pick()
    println(p(a = null, b = null))
    println(p(1, 2L))
    println(p(3L, null))
    println(listOf(p.ib(null), p.is_(null), p.sb(null), p.df(null)))
}
