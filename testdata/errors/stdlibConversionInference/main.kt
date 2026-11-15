// Conversions during inference: a function value passed for a suspend
// function type whose variables it gives, a lambda of an if passed for
// a variable whose bound is a suspend function type, a lambda passed for
// a variable a Java interface bounds, and a SAM constructor's lambda
// whose declared parameter doesn't fit the expected type.

fun <T> foo1(f: suspend (T) -> String): T = TODO()
fun <T, K> foo2(f: suspend (T) -> K): Map<T, K> = TODO()
fun <T> foo3(x: T, f: suspend (T) -> String) {}
fun <I> id(e: I): I = e

fun suspendGenerics(f: (Int) -> String) {
    val a: Int = foo1(f)
    val b: Map<Int, String> = foo2(f)
    val c: Int = foo1(id(f))
    foo3("a", f) // ERROR type mismatch
}

class Bar(val foo: Foo<suspend () -> Unit>)
class Foo<out T : Any>(val state: T?)

fun suspendBranches(b: Boolean) {
    Bar(Foo({ -> }))
    Bar(Foo(if (b) { -> } else null))
    Bar(Foo(if (b) { -> } else { -> }))
}

fun javaSam() {
    val t: Ticker = Selectors.select({ s -> s + s }, null)
    val l: Collection<Int> = Selectors.compute(Computable { listOf(1) })
}

fun takeFunction(f: MyFunction<in Number, out CharSequence>) {}

fun samConstructor() {
    takeFunction(MyFunction { x -> x.toInt().toString() })
    takeFunction(MyFunction { x: Int -> x.toString() }) // ERROR type mismatch: inferred type is Int but Number was expected
}
