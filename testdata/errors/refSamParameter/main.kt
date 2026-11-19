// A callable reference's parameters aren't converted to the fun
// interfaces or Java interfaces its function takes: it fits a function
// type with those interfaces, not one with function types in their place.
// A fun interface's constructor converts its parameter.
fun interface KF { fun run() }
fun kf(k: KF) {}
fun foo(x: (() -> Unit) -> Unit) {}
fun fooR(x: (Runnable) -> Unit) {}
fun <T> gen(t: T, x: (() -> Unit) -> Unit) {}
fun <T> any(x: (T) -> Unit) {}

fun test(a: A) {
    foo(A::invokeLater) // ERROR inapplicable candidate
    foo(::kf) // ERROR inapplicable candidate
    foo(a::inst) // ERROR inapplicable candidate
    gen(1, A::invokeLater) // ERROR inapplicable candidate
    val f: (() -> Unit) -> Unit = A::invokeLater // ERROR inapplicable candidate
    val g: (Int, () -> Unit) -> Unit = A::two // ERROR inapplicable candidate

    fooR(A::invokeLater)
    val h: (Runnable) -> Unit = A::invokeLater
    val k: (KF) -> Unit = ::kf
    any(A::invokeLater)
    val r = A::invokeLater
    r(Runnable { })
    A.invokeLater { }
    kf { }
    val c: (() -> Unit) -> KF = ::KF
    foo(::KF)
}
