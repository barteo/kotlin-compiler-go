// A lambda returned with return@label takes its parameter types from
// the result type expected, as one the lambda ends with.
fun <A, B> compose(f: (A) -> (B) -> String): (A, B) -> String = { a, b -> f(a)(b) }

fun <T> twice(f: (T) -> (T) -> T, x: T): T = f(x)(x)

fun main() {
    val join = compose<Int, String> { a ->
        if (a < 0) return@compose { b -> "negative $b" }
        return@compose { b -> b + a + b }
    }
    println(join(2, "ab"))
    println(join(-1, "ab"))
    println(twice({ a -> return@twice { b -> a + b * 2 } }, "xyz".length))
    println(twice<String>({ a -> return@twice { b -> a + b + a.length } }, "k"))
}
