// Classes, objects and interfaces whose supertypes are function types:
// they implement kotlin.jvm.functions.FunctionN, with a bridge from the
// erased invoke, and suspend ones SuspendFunction too.

import kotlin.coroutines.*

class Greeter(val greeting: String) : (String) -> String {
    override fun invoke(name: String) = "$greeting, $name"
}

class Counter : () -> Int {
    var n = 0
    override fun invoke() = ++n
}

interface Parser : (String) -> Int

fun interface Op : (Int, Int) -> Int

abstract class Twice : (Long) -> Long {
    override fun invoke(x: Long) = step(step(x))
    abstract fun step(x: Long): Long
}

class Length : suspend (String) -> Int {
    override suspend fun invoke(s: String) = s.length
}

fun apply(f: (String) -> String, s: String) = f(s)

fun <T> call(f: () -> T) = f()

suspend fun length(f: suspend (String) -> Int) = f("four")

fun main() {
    println(Greeter("Hi")("Ann"))
    println(apply(Greeter("Hello"), "Bob"))
    val c = Counter()
    c()
    println(call(c))
    val p = object : Parser {
        override fun invoke(s: String) = s.toInt() * 2
    }
    println(p("21"))
    println(listOf("1", "2").map(p))
    val plus = Op { a, b -> a + b }
    println(plus(2, 3))
    println(object : Twice() { override fun step(x: Long) = x * 3 }(2L))
    val f: Any = Greeter("Yo")
    println((f as (String) -> String)("Cy"))
    suspend {
        println(Length()("abc"))
        println(length(Length()))
        println(Length().javaClass.interfaces.map { it.simpleName })
    }.startCoroutine(Continuation(EmptyCoroutineContext) { it.getOrThrow() })
}
