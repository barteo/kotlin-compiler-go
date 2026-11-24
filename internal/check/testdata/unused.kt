// The warnings kotlinc 2.4.21 reports of the unused expressions below, as
// TestWarningsUnused reads them: each line ends with a comment naming the
// warnings it reports, if any.
object O
enum class E { A }
class C { companion object; val p = 1; fun m(q: Int) {
    p
    this // UNUSED_EXPRESSION
    q // UNUSED_EXPRESSION
    this.p
} }
const val K = 1
val top = 2
fun f1(x: Int, s: String?, a: Array<Int>) {
    O // UNUSED_EXPRESSION
    E.A
    C // UNUSED_EXPRESSION
    Unit // UNUSED_EXPRESSION
    null // UNUSED_EXPRESSION
    -1 // UNUSED_EXPRESSION
    !true
    'c' // UNUSED_EXPRESSION
    1L // UNUSED_EXPRESSION
    1.0 // UNUSED_EXPRESSION
    true // UNUSED_EXPRESSION
    K
    top
    s!!
    (x) // UNUSED_EXPRESSION
    x as Any
    {} // UNUSED_LAMBDA_EXPRESSION
    { y: Int -> y } // UNUSED_LAMBDA_EXPRESSION
    x.toString()
    x + 1
    x == 1
    a[0]
    "a$x" // UNUSED_EXPRESSION
    "a${x + 1}"
    "a${top}"
    """raw""" // UNUSED_EXPRESSION
    String::class // UNUSED_EXPRESSION
    x::class // UNUSED_EXPRESSION
    ::f1 // UNUSED_EXPRESSION
    if (x > 0) 1 else 2 // UNUSED_EXPRESSION
    when (x) { 1 -> 2; else -> 3 } // UNUSED_EXPRESSION
    0..1
    @Suppress("UNUSED_EXPRESSION") x
    val v = 1
    v // UNUSED_EXPRESSION
    var w = 1
    w // UNUSED_EXPRESSION
    w = 3
    print(w)
}
fun <T> run2(f: () -> T): T = f()
fun f2(x: Int, c: Boolean) {
    if (c) Unit else Unit
    when { else -> Unit }
    if (c) { Unit } // UNUSED_EXPRESSION
    if (c) x // UNUSED_EXPRESSION
    if (c) { x } else { 2 } // UNUSED_EXPRESSION
    for (i in 0..1) i // UNUSED_EXPRESSION
    for (i in 0..1) { i } // UNUSED_EXPRESSION
    while (c) x // UNUSED_EXPRESSION
    do { x } while (c) // UNUSED_EXPRESSION
    try { x } catch (e: Exception) { e } finally { x } // UNUSED_EXPRESSION
    run2 { x }
    run2 { x; 1 } // UNUSED_EXPRESSION
    val v = run2 { if (c) 1 else 2 }
    val w = if (c) { x; 1 } else { 2 } // UNUSED_EXPRESSION
    val u = when { c -> { 1; 2 } else -> 3 } // UNUSED_EXPRESSION
    run2 { if (c) 1 else 2 }
    if (c) x else if (c) 2 else 3 // UNUSED_EXPRESSION
    when (x) { 1 -> x; 2 -> Unit; else -> {} } // UNUSED_EXPRESSION
    print(v + w + u)
}
val p: Int get() { 1; return 2 } // UNUSED_EXPRESSION
fun g(x: Int) = x
fun h(x: Int): Int { x; return x } // UNUSED_EXPRESSION
class A { init { 1 } } // UNUSED_EXPRESSION
fun e(x: Int) { x } // UNUSED_EXPRESSION
fun l() = { x: Int -> x; 1 } // UNUSED_EXPRESSION
fun foo() {}
fun bar(f: () -> Unit) {}
infix fun Int.ck(f: () -> Unit) {}
fun n3() {
    1 ck { }

    { } // UNUSED_LAMBDA_EXPRESSION
}
fun n4(x: Int) {
    x // UNUSED_EXPRESSION
    { } // UNUSED_LAMBDA_EXPRESSION
}
fun n5() {
    val a = 1
    { } // UNUSED_LAMBDA_EXPRESSION
}
fun n6() {
    -1.0
    +1 // UNUSED_EXPRESSION
    -1L
    -(1) // UNUSED_EXPRESSION
}
fun n7() {
    -0 // UNUSED_EXPRESSION
    -0L
    -0.0
    +0 // UNUSED_EXPRESSION
    -(1 - 1)
}
// Lambdas whose body is checked against an expected Unit result, whose
// last statement is unused too, and anonymous functions.
fun uRun(block: () -> Unit) {}
fun <T> uConsume(block: () -> T) {}
fun interface UF { fun f() }
fun uTake(f: UF) {}
fun uDef(f: () -> Unit = { 1; 2 }) {}
class UK(val f: () -> Unit = { 3 }) {
    val p = uRun { "" } // UNUSED_EXPRESSION
    init { uRun { 'c' } } // UNUSED_EXPRESSION
}
fun u1(c: Boolean, sb: StringBuilder, a: String) {
    uRun { 1 } // UNUSED_EXPRESSION
    uRun { val y = 2; y } // UNUSED_EXPRESSION
    uRun { uRun { 'c' } } // UNUSED_EXPRESSION
    uConsume<Unit> { 2.0 } // UNUSED_EXPRESSION
    uConsume { 3 }
    val f: () -> Unit = { "" } // UNUSED_EXPRESSION
    f()
    uRun { if (c) "" else "a" } // UNUSED_EXPRESSION
    uRun { fun() = 1 } // UNUSED_EXPRESSION
    uRun { null } // UNUSED_EXPRESSION
    uRun { "x" + 1 }
    uRun { String::class } // UNUSED_EXPRESSION
    uTake { 2 } // UNUSED_EXPRESSION
    sb.also { it } // UNUSED_EXPRESSION
    sb.let { it }
    uConsume { when (a) { "" -> "" } } // UNUSED_EXPRESSION
    uConsume { when (a) { "" -> ""; else -> "b" } }
    listOf(1).map { it }
}
fun u2(c: Boolean) {
    fun() = 1 // UNUSED_EXPRESSION
    if (c) fun() = 1 else fun() {} // UNUSED_EXPRESSION
    when (1) {
        0 -> fun() = 2 // UNUSED_EXPRESSION
        else -> fun() = 3 // UNUSED_EXPRESSION
    }
    val h = if (c) fun() = 1 else fun() = 2
    h()
    uConsume { fun() = 1 }
    try { fun() = 1 } finally { fun() = 2 } // UNUSED_EXPRESSION
    String::class // UNUSED_EXPRESSION
}
// R? that the expected type makes Unit isn't coerced to Unit, and the
// class literal of a reified type parameter isn't reported.
class UnInv<T>(val x: T?)
fun <R> unFoo(f: () -> R?): UnInv<R> = UnInv(f())
fun unG(): UnInv<Unit> = unFoo { if (false) Unit else null }
inline fun <reified K> unSel() {
    K::class
    String::class // UNUSED_EXPRESSION
}
