fun f(`_`: Int) = _ + 1 // ERROR can be used only in back-ticks
fun g(_: Int) = 1 // ERROR names _, __, ___, ... are reserved
annotation class A
fun <T> w() where @A T : Any = Unit // ERROR not allowed inside where clauses
infix fun Int.suspend(c: () -> Unit) {}
fun s() { 1 suspend fun() {} } // ERROR modifier form for a non-built-in 'suspend' function
fun ok(`_`: Int) = `_` + 1
data class P(val a: Int, val b: Int)
fun lambda(f: (P) -> Int) = f(P(1, 2))
val l = lambda { (_, b) -> b }
