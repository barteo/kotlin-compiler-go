// Operator extensions that kotlinc calls instead of a built-in operator:
// an infix extension of a number named as its non-infix operator member,
// and a String?.plus extension, which wins over String?.plus(Any?).

infix fun Int.rem(other: Int): Int = 100 + this % other
infix fun Int.plus(other: Int): String = "ext $this $other"

operator fun String?.plus(p: String): String = "custom(${this ?: "nil"}, $p)"

fun main() {
    println(7 rem 3)
    println(7 % 3)
    println(1 plus 2)
    println(1 + 2)
    val a: String? = null
    val b: String? = "x"
    println(a + "y")
    println(b + "y")
    println("s" + "y")
    println(a + 1)
}
