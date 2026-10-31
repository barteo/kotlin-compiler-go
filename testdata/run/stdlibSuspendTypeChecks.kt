// is and as with suspend function types: as kotlinc's, is checks for a
// SuspendFunction whose arity counts the continuation, and as casts to
// the larger FunctionN, so a lambda taking a Continuation casts to a
// suspend function type.
import kotlin.coroutines.*

suspend fun twice(x: Int) = x * 2

fun run(c: suspend () -> String): String {
    var res = "none"
    c.startCoroutine(Continuation(EmptyCoroutineContext) { res = it.getOrThrow() })
    return res
}

@Suppress("UNCHECKED_CAST")
fun check(name: String, f: Any) {
    val zero = f as? suspend () -> Any?
    val one = f as? suspend (Int) -> Int
    println("$name: ${zero != null} ${one != null}")
}

fun main() {
    val s: suspend (Int) -> Int = { it + 1 }
    val r: suspend (Int) -> Int = ::twice
    val p: (Int) -> Int = { it }
    val p2: (Int, Int) -> Int = { a, b -> a + b }
    check("lambda", s)
    check("reference", r)
    check("plain", p)
    check("plain2", p2)
    check("suspend0", suspend { 1 })
    println(s is suspend (Int) -> Int)

    val raw: (Continuation<String>) -> Any? = { "raw" }
    println(run(raw as suspend () -> String))
    val t = ::twice as suspend (Int) -> Int
    println(run { t(4).toString() })
    try {
        p2 as suspend (Int) -> Int
        println("cast")
    } catch (e: ClassCastException) {
        println("ClassCastException")
    }
    val n: Any? = null
    println(n as? suspend () -> Unit)
    try {
        n as suspend (Int) -> Unit
    } catch (e: NullPointerException) {
        println(e.message)
    }
}
