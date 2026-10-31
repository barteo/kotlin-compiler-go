// Classes of the program named as built-in types hide them where they are
// in scope, and the qualified names (kotlin.String) still mean the
// built-in types.
sealed class TextValue {
    data class String(val value: kotlin.String) : TextValue()
    data class Resource(val id: kotlin.Int) : TextValue()

    fun show(): kotlin.String = when (this) {
        is String -> "text " + value
        is Resource -> "resource " + id
    }
}

class Int(val n: kotlin.Int) {
    override fun toString() = "Int($n)"
}

fun wrap(n: kotlin.Int): Int = Int(n)

fun lengths(xs: kotlin.collections.List<kotlin.String>): kotlin.collections.List<kotlin.Int> = xs.map { it.length }

fun outside(s: String): kotlin.Int = s.length // String is the built-in here

fun main() {
    val values: List<TextValue> = listOf(TextValue.String("hi"), TextValue.Resource(7))
    for (v in values) {
        println(v.show())
    }
    println(TextValue.String("x"))
    val i: Int = wrap(3)
    println(i)
    println(i.n + 1)
    println(lengths(listOf("a", "bcd")))
    println(outside("four"))
    val s: kotlin.String = "plain"
    println(s.uppercase())
}
