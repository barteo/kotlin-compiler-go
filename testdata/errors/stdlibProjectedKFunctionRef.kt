import kotlin.reflect.KFunction2

// As K2, a KFunction type whose projected argument holds a type variable
// tells nothing of the parameter it projects: it doesn't choose among a
// reference's overloads.
interface Foo {
    fun resolve(var1: Int): String
    fun resolve(var1: String): String
}

fun <K> bar2(f: KFunction2<out K, String, String>) {} // ERROR projection is conflicting

fun main() {
    bar2(Foo::resolve) // ERROR overload resolution ambiguity
}
