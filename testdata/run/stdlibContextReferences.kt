// References and values passed for context function types, whose context
// parameters they take as their first parameters: references to functions
// and properties, unbound and bound, with default values and varargs, to
// inline functions; suspend conversions of function values; fun
// interfaces whose method has context parameters; a reference bound to a
// companion that extends its class; and extension properties of function
// type invoked on implicit receivers.

import kotlin.coroutines.Continuation
import kotlin.coroutines.EmptyCoroutineContext
import kotlin.coroutines.startCoroutine

class A(val n: String)

fun withA(f: context(A) (Int) -> String) = f(A("a"), 1)
fun withAExt(f: context(A) Int.() -> String) = f(A("b"), 2)
fun withBoth(f: context(A, Int) () -> String) = f(A("c"), 3)
inline fun inlineWithA(f: context(A) () -> String) = f(A("i"))

fun valueParam(a: A, i: Int) = "value ${a.n} $i"
fun A.extension(i: Int) = "ext $n $i"
inline fun inlineValue(a: A, i: Int) = "inline ${a.n} $i"
fun withDefault(a: A, s: String = "default") = "${a.n} $s"
fun vararged(vararg xs: A) = xs.joinToString { it.n }

val A.prop: String
    get() = "prop $n"

fun interface Action {
    context(a: A)
    fun run(s: String): String
}

fun suspendWithA(f: suspend context(A) () -> String) {
    f.startCoroutine(A("s"), Continuation(EmptyCoroutineContext) { println(it.getOrThrow()) })
}

val plain: (A) -> String = { "plain ${it.n}" }
val receiver: A.() -> String = { "receiver $n" }

open class Outer {
    fun member() = "member of ${this::class.simpleName}"

    companion object : Outer()
}

fun call(f: () -> String) = f()

val String.shout: (Int) -> String
    get() = { n -> uppercase() + "!".repeat(n) }

val <T> T.echo: context(T) (T) -> String
    get() = { x -> "$this ${contextOf<T>()} $x" }

fun main() {
    println(withA(::valueParam))
    println(withA(A::extension))
    println(withA(::inlineValue))
    println(withAExt(::valueParam))
    println(withBoth(A::extension))
    println(inlineWithA(::withDefault))
    println(inlineWithA(A::prop))
    println(inlineWithA(::vararged))
    val ref = Action(::withDefault)
    println(with(A("sam2")) { ref.run("given") })
    suspendWithA(plain)
    suspendWithA(receiver)
    println(call(Outer::member))
    with("s") { println(shout(2)) }
    with(1) { with(2) { println(echo(3)) } }
}
