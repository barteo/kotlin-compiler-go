// A lambda that isn't analyzed against a parameter's function type may
// declare the expected receiver, and context parameters, as its first
// parameters: its receiver is then a plain parameter.

class Sample(val name: String)

fun <K> id(x: K): K = x

val upper: String.() -> String = { s -> s.uppercase() }
val joined: String.(Int) -> String = { s: String, n -> s + n }
val withContext: context(String) (Int) -> String = { c: String, n -> c + "#" + n }

fun pick(b: Boolean): Sample.(String) -> String =
    if (b) { s, x -> s.name + x } else { s, x -> x + s.name }

fun main() {
    println("abc".upper())
    println("x".joined(4))
    println(with("ctx") { withContext(7) })
    val greet: Sample.() -> String = id { s: Sample -> "hi " + s.name }
    println(Sample("ann").greet())
    val explicit = id<Sample.() -> String> { s -> "bye " + s.name }
    println(Sample("bob").explicit())
    println(Sample("cy").(pick(true))("!"))
    println(Sample("cy").(pick(false))("?"))
    fun local() {
        val f: Int.(Int) -> Int = { a, b -> a * 10 + b }
        println(3.f(4))
    }
    local()
}
