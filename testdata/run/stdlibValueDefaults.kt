// Default arguments of value classes' functions, which kotlinc compiles
// to static name-impl$default methods that take the value first, and
// the members and operators of type parameters bounded by built-in types.

@JvmInline
value class Z(val x: Int) {
    fun test(y: Int = 42) = x + y
    fun range(from: Int = 0, to: Int = this.x) = "$from..$to"
    fun String.tagged(sep: String = ":") = this + sep + x
    fun tag() = "a".tagged() + " " + "b".tagged("=")
    inline fun <R> withDefault(fn: (Z) -> R, z: Z = Z(x + 1)) = fn(z)
    fun next() = withDefault({ it.x * 10 })
}

@JvmInline
value class Name(val s: String) {
    fun greet(greeting: String = "Hello", punct: Char = '!') = "$greeting, $s$punct"
}

@JvmInline
value class G<T : Int>(val v: T) {
    fun plus(d: Int = 1) = v + d
    fun show(prefix: String = "#") = prefix + v.toString() + "/" + v.toLong()
}

fun <T : Int> twice(x: T) = x * 2
fun <T : Int> neg(x: T) = -x
fun <T : Long> big(x: T) = 1L + x
fun <T : String> len(x: T) = x.length
fun <T : Int?> orZero(x: T) = x ?: 0
fun <T : Int> same(x: T): T = x.also { println("also $it") }

fun main() {
    val z = Z(800)
    println("" + z.test() + " " + z.test(1) + " " + z.range() + " " + z.range(to = 3) + " " + z.tag() + " " + z.next())
    println(Name("Ann").greet() + " " + Name("Bo").greet(punct = '?') + " " + Name("Cy").greet("Hi"))
    val g = G(7)
    println("" + g.plus() + " " + g.plus(3) + " " + g.show() + " " + g.show("@"))
    println("" + twice(21) + " " + neg(5) + " " + big(2L) + " " + len("abc") + " " + orZero(null) + " " + orZero(4) + " " + same(9))
}
