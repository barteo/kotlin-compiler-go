// A reference to a function with a vararg, adapted to an expected function
// type whose parameter types are type variables, gives them the vararg's
// element type, or its array's for a parameter in the vararg's place.
fun <T> foo(b: (Any, T) -> String): String = b("O", "K" as T)
fun <T> bar(b: (Any, T) -> String): String = b("O", arrayOf<Any>("K", "!") as T)

fun of(vararg args: Any): String = args[0].toString() + args[1].toString()
fun of2(a: Any, vararg args: Any): String = a.toString() + args.joinToString("")

fun <T> three(b: (String, T, T) -> String): String = b("a", "b" as T, "c" as T)
fun join(vararg xs: Any?): String = xs.joinToString("-")

fun main() {
    println(foo(::of))
    println(bar(::of2))
    println(three(::join))
}
