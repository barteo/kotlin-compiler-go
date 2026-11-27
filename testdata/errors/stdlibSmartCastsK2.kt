// Smart casts K2 makes that kc used to miss, each a false error before:
// a cast of a local smart casts the values it holds, a lambda called at
// most once keeps the smart casts of the vars it assigns, do-while(true)
// ends at its breaks, and a receiver smart cast to an intersection has
// the members of each of its types.
import kotlin.contracts.*

interface A { fun foo() }
interface B { fun bar() }
class D(val any: Any?)

fun aliasCast(d: D) {
    val a = d.any
    a as A
    a.foo()
    d.any.foo()
}

fun aliasCastOfParam(x: Any?) {
    val a = x
    a as A
    x.foo()
}

@OptIn(ExperimentalContracts::class)
inline fun atMostOnce(block: () -> Unit) {
    contract { callsInPlace(block, InvocationKind.AT_MOST_ONCE) }
    block()
}

fun capturedAtMostOnce() {
    var s: String? = null
    s = ""
    atMostOnce {
        s.length
        s = null
    }
}

fun endlessDoWhile(x: Any, b: Boolean) {
    do {
        if (b) {
            x as A
            break
        }
    } while (true)
    x.foo()
}

fun endlessDoWhileNoBreak(x: Any) {
    do {
        x as A
    } while (true)
    x.foo()
}

fun Any?.intersectionReceiver() {
    if (this is A && this is B) {
        foo()
        bar()
    }
}

fun intersectionLambdaReceiver(x: Any) {
    with(x) {
        if (this is A && this is B) {
            foo()
            bar()
        }
    }
}

fun failedCast(x: Any) {
    do {
        if (x is A) break
    } while (true)
    x.bar() // ERROR unresolved reference: bar
}
