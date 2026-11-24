// The UNUSED_ANONYMOUS_PARAMETER warnings kotlinc 2.4.21 reports with
// -Wextra of the lambdas below, as TestWarningsUnusedParam reads them:
// each line ends with a comment naming the warnings it reports, if any.
fun foo(a: (Int) -> Any) {}
fun foo2(a: Int.(Int) -> Any) {}
fun foo3(a: (Int, Int) -> Any) {}
fun s(a: suspend (Int) -> Unit) {}
fun baz() {}

fun bar() {
    foo { x -> x::toString }
    foo { x -> "$x" }
    foo { x -> "${x + 1}" }
    foo { x -> run { x } }
    foo { x -> object { val y = x } }
    foo { x -> class L { val y = x }; L() }
    foo { x ->
        fun loc() = x
        loc()
    }
    foo { x -> when (x) { else -> 1 } }
    foo { x -> x.also { } }
    foo2 { x -> this } // UNUSED_ANONYMOUS_PARAMETER
    s { x -> } // UNUSED_ANONYMOUS_PARAMETER
    @Suppress("UNUSED_ANONYMOUS_PARAMETER")
    foo { x -> 1 }
    foo { `_` -> 1 } // UNUSED_ANONYMOUS_PARAMETER
    foo { _ -> 1 }
    foo { x -> listOf(1).map { it + x } }
    foo { x -> try { x } finally { } }
    foo { x -> return@foo 1 } // UNUSED_ANONYMOUS_PARAMETER
    foo { p1: Int -> baz() } // UNUSED_ANONYMOUS_PARAMETER
    foo3 { p1: Int, p2: Int -> p1 } // UNUSED_ANONYMOUS_PARAMETER
    foo { a -> foo { b -> foo { c -> a + c } } } // UNUSED_ANONYMOUS_PARAMETER
    foo { it -> 1 } // UNUSED_ANONYMOUS_PARAMETER
    foo { 1 }
    foo(fun(x: Int) = 1)
}
val p: (Int, String) -> Unit = { a, b -> println(b) } // UNUSED_ANONYMOUS_PARAMETER
class C(val f: (Int) -> Unit = { u -> }) // UNUSED_ANONYMOUS_PARAMETER
data class P(val a: Int, val b: Int)
fun g(h: (P) -> Any) {}
fun t() { g { (a, b) -> a + b } }
