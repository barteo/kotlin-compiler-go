// The warnings kotlinc 2.4.21 reports of the contracts' callsInPlace
// effects below, as TestWarningsInPlace reads them: each line ends with a
// comment naming the warnings it reports, if any.
@file:OptIn(ExperimentalContracts::class)
import kotlin.contracts.*

fun bar(x: () -> Unit) {}
inline fun inl(x: () -> Unit) { x() }
inline fun cross(crossinline x: () -> Unit) { x() }
fun cond(): Boolean = true

fun once(f: () -> Unit) {
    contract { callsInPlace(f, InvocationKind.EXACTLY_ONCE) }
    f()
}
fun twice(f: () -> Unit) {
    contract { callsInPlace(f, InvocationKind.EXACTLY_ONCE) } // WRONG_INVOCATION_KIND
    f()
    f()
}
fun zero(f: () -> Unit) {
    contract { callsInPlace(f, InvocationKind.EXACTLY_ONCE) } // WRONG_INVOCATION_KIND
}
fun maybe(f: () -> Unit) {
    contract { callsInPlace(f, InvocationKind.EXACTLY_ONCE) } // WRONG_INVOCATION_KIND
    if (cond()) f()
}
fun branches(f: () -> Unit) {
    contract { callsInPlace(f, InvocationKind.EXACTLY_ONCE) }
    if (cond()) f() else f()
}
fun earlyReturn(f: () -> Unit) {
    contract { callsInPlace(f, InvocationKind.EXACTLY_ONCE) }
    if (cond()) {
        f()
        return
    }
    f()
}
fun earlyReturnMissing(f: () -> Unit) {
    contract { callsInPlace(f, InvocationKind.EXACTLY_ONCE) } // WRONG_INVOCATION_KIND
    if (cond()) return
    f()
}
fun throwing(f: () -> Unit) {
    contract { callsInPlace(f, InvocationKind.EXACTLY_ONCE) }
    if (cond()) throw Exception()
    f()
}
fun erroring(f: () -> Unit) {
    contract { callsInPlace(f, InvocationKind.EXACTLY_ONCE) }
    if (cond()) error("x") else f()
}
fun atMost(f: () -> Unit) {
    contract { callsInPlace(f, InvocationKind.AT_MOST_ONCE) }
    if (cond()) f()
}
fun atMostTwice(f: () -> Unit) {
    contract { callsInPlace(f, InvocationKind.AT_MOST_ONCE) } // WRONG_INVOCATION_KIND
    f()
    if (cond()) f()
}
fun atLeast(f: () -> Unit) {
    contract { callsInPlace(f, InvocationKind.AT_LEAST_ONCE) }
    f()
    while (cond()) f()
}
fun atLeastLoop(f: () -> Unit) {
    contract { callsInPlace(f, InvocationKind.AT_LEAST_ONCE) } // WRONG_INVOCATION_KIND
    while (cond()) f()
}
fun atLeastDo(f: () -> Unit) {
    contract { callsInPlace(f, InvocationKind.AT_LEAST_ONCE) }
    do { f() } while (cond())
}
fun forLoop(f: () -> Unit) {
    contract { callsInPlace(f, InvocationKind.AT_MOST_ONCE) } // WRONG_INVOCATION_KIND
    for (i in 1..3) f()
}
fun unknown(f: () -> Unit) {
    contract { callsInPlace(f) }
    for (i in 1..3) f()
}
fun viaRun(f: () -> Unit) {
    contract { callsInPlace(f, InvocationKind.EXACTLY_ONCE) }
    run { f() }
}
fun viaInl(f: () -> Unit) {
    contract { callsInPlace(f, InvocationKind.EXACTLY_ONCE) } // WRONG_INVOCATION_KIND
    inl { f() }
}
fun viaInlArg(f: () -> Unit) {
    contract { callsInPlace(f, InvocationKind.EXACTLY_ONCE) } // WRONG_INVOCATION_KIND
    inl(f)
}
fun viaOnce(f: () -> Unit) {
    contract { callsInPlace(f, InvocationKind.EXACTLY_ONCE) }
    once(f)
}
fun viaInvoke(f: () -> Unit) {
    contract { callsInPlace(f, InvocationKind.EXACTLY_ONCE) }
    f.invoke()
}
fun viaWhen(f: () -> Unit, x: Int) {
    contract { callsInPlace(f, InvocationKind.EXACTLY_ONCE) }
    when (x) {
        1 -> f()
        else -> f()
    }
}
fun tryCatch(f: () -> Unit) {
    contract { callsInPlace(f, InvocationKind.EXACTLY_ONCE) } // WRONG_INVOCATION_KIND
    try { f() } catch (e: Exception) { }
}
fun tryFinally(f: () -> Unit) {
    contract { callsInPlace(f, InvocationKind.EXACTLY_ONCE) }
    try { f() } finally { }
}
fun tryRethrow(f: () -> Unit) {
    contract { callsInPlace(f, InvocationKind.EXACTLY_ONCE) }
    try { f() } catch (e: Exception) { throw e }
}
fun finallyCalls(f: () -> Unit) {
    contract { callsInPlace(f, InvocationKind.EXACTLY_ONCE) }
    try { cond() } finally { f() }
}
fun returnInTry(f: () -> Int): Int {
    contract { callsInPlace(f, InvocationKind.EXACTLY_ONCE) } // WRONG_INVOCATION_KIND
    try { return f() } catch (e: Exception) { return 1 }
}
fun andAnd(f: () -> Boolean): Boolean {
    contract { callsInPlace(f, InvocationKind.EXACTLY_ONCE) } // WRONG_INVOCATION_KIND
    return cond() && f()
}
fun elvis(f: () -> String?): String {
    contract { callsInPlace(f, InvocationKind.EXACTLY_ONCE) }
    return f() ?: "x"
}
fun leakArg(f: () -> Unit) {
    contract { callsInPlace(f, InvocationKind.AT_MOST_ONCE) } // LEAKED_IN_PLACE_LAMBDA
    bar(f) // LEAKED_IN_PLACE_LAMBDA
}
fun leakVal(f: () -> Unit) {
    contract { callsInPlace(f, InvocationKind.AT_MOST_ONCE) } // LEAKED_IN_PLACE_LAMBDA
    val g = f // LEAKED_IN_PLACE_LAMBDA
    g()
}
fun leakAssign(f: () -> Unit) {
    contract { callsInPlace(f, InvocationKind.AT_MOST_ONCE) } // LEAKED_IN_PLACE_LAMBDA
    val g: () -> Unit
    g = f // LEAKED_IN_PLACE_LAMBDA
    g()
}
fun leakReturn(f: () -> Unit): () -> Unit {
    contract { callsInPlace(f, InvocationKind.AT_MOST_ONCE) } // LEAKED_IN_PLACE_LAMBDA
    return f // LEAKED_IN_PLACE_LAMBDA
}
fun leakLocalFun(f: () -> Unit) {
    contract { callsInPlace(f, InvocationKind.AT_MOST_ONCE) } // LEAKED_IN_PLACE_LAMBDA
    fun g() { f() } // LEAKED_IN_PLACE_LAMBDA
    g()
}
fun leakObject(f: () -> Unit) {
    contract { callsInPlace(f, InvocationKind.AT_MOST_ONCE) } // LEAKED_IN_PLACE_LAMBDA
    object : Runnable { override fun run() { f() } }.run() // LEAKED_IN_PLACE_LAMBDA
}
fun leakLambda(f: () -> Unit) {
    contract { callsInPlace(f, InvocationKind.AT_MOST_ONCE) } // LEAKED_IN_PLACE_LAMBDA
    bar { f() } // LEAKED_IN_PLACE_LAMBDA
}
fun leakLet(f: () -> Unit) {
    contract { callsInPlace(f, InvocationKind.AT_MOST_ONCE) } // LEAKED_IN_PLACE_LAMBDA
    f.let { it() } // LEAKED_IN_PLACE_LAMBDA
}
fun leakToString(f: () -> Unit): String {
    contract { callsInPlace(f, InvocationKind.AT_MOST_ONCE) } // LEAKED_IN_PLACE_LAMBDA
    return f.toString() // LEAKED_IN_PLACE_LAMBDA
}
fun crossLambda(f: () -> Unit) {
    contract { callsInPlace(f, InvocationKind.AT_MOST_ONCE) } // LEAKED_IN_PLACE_LAMBDA
    cross { f() } // LEAKED_IN_PLACE_LAMBDA
}
fun two(f: () -> Unit, g: () -> Unit) {
    contract {
        callsInPlace(f, InvocationKind.EXACTLY_ONCE)
        callsInPlace(g, InvocationKind.AT_MOST_ONCE) // WRONG_INVOCATION_KIND
    }
    f()
    g()
    g()
}
fun inlineLambdaInLoop(f: () -> Unit) {
    contract { callsInPlace(f, InvocationKind.AT_MOST_ONCE) } // WRONG_INVOCATION_KIND
    listOf(1).forEach { f() }
}
fun compareIt(f: () -> Unit, g: () -> Unit): Boolean {
    contract { callsInPlace(f, InvocationKind.AT_MOST_ONCE) }
    return f == g
}
