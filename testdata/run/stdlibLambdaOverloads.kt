// Overloads that take lambdas and fun interfaces: a lambda's declared
// parameter types and a type parameter's bounds rule out candidates, a
// parameter that an argument converts to a Kotlin fun interface is as
// specific as the interface's function type, a candidate converting
// fewer arguments wins among equally specific ones, and an integer
// literal result fits the type @OverloadResolutionByLambdaReturnType
// expects.
@file:OptIn(kotlin.experimental.ExperimentalTypeInference::class)

fun foo(a: Any, f: (Any) -> Int) = "any ${f(a)}"
fun foo(i: Int, f: (Int) -> Int) = "int ${f(i)}"

abstract class Table
object Invoker {
    operator fun <T : Table> invoke(table: T) = "table"
    operator fun <R> invoke(block: Invoker.() -> R): R = block()
}

fun <T1> baz(x: T1, y: (T1) -> Unit) = "T1"
@JvmName("baz2")
fun <T2> baz(x: T2, y: (String) -> Unit) = "String"

enum class Color { RED }
fun paint(c: Color) = "paint $c"

fun interface KRunnable { fun invoke() }

object Kotlin {
    fun foo(i: Int, r: KRunnable) = "Int, KRunnable"
    fun foo(n: Number, f: () -> Unit) = "Number, fun"
    fun bar() {}
    fun test(f: () -> Unit) {
        println(foo(1, ::bar))
        println(foo(1, f))
        println(foo(1) {})
    }
}

object Java {
    fun foo(i: Int, r: Runnable) = "Int, Runnable"
    fun foo(n: Number, f: () -> Unit) = "Number, fun"
    fun bar() {}
    fun test(f: () -> Unit) {
        println(foo(1, ::bar))
        println(foo(1, f))
        println(foo(1) {})
    }
}

object Same {
    fun run(r: KRunnable) = "KRunnable"
    fun run(f: () -> Unit) = "fun"
    fun test(f: () -> Unit) {
        println(run {})
        println(run(f))
    }
}

class Flow<out T>(val v: T)

@OverloadResolutionByLambdaReturnType
fun <T> Flow<T>.debounce(timeoutMillis: (T) -> Long): String = "Long ${timeoutMillis(v)}"

@JvmName("debounceText")
@OverloadResolutionByLambdaReturnType
fun <T> Flow<T>.debounce(timeout: (T) -> String): String = "String ${timeout(v)}"

fun main() {
    println(foo(1) { x: Int -> x + 1 })
    println(Invoker { "block" })
    println(baz(Color.RED, { x: String -> }))
    fun paint(s: String) = "local $s"
    println(paint(Color.RED))
    println(paint("x"))
    Kotlin.test {}
    Java.test {}
    Same.test {}
    println(Flow(1).debounce { 0 })
    println(Flow(1).debounce { "s" })
}
