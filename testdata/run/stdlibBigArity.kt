// Function types with more than 22 parameters: kotlin.jvm.functions.FunctionN.

class A(val v: String)

fun join(
    p00: A, p01: A, p02: A, p03: A, p04: A, p05: A, p06: A, p07: A, p08: A, p09: A,
    p10: A, p11: A, p12: A, p13: A, p14: A, p15: A, p16: A, p17: A, p18: A, p19: A,
    p20: A, p21: A, p22: Int,
): String = p00.v + p21.v + p22

typealias F<T> = (A, A, A, A, A, A, A, A, A, A, A, A, A, A, A, A, A, A, A, A, A, A, T) -> String

typealias G = A.(A, A, A, A, A, A, A, A, A, A, A, A, A, A, A, A, A, A, A, A, A, A, A, A) -> Unit

fun <T> call(f: F<T>, a: A, t: T): String = f(a, a, a, a, a, a, a, a, a, a, a, a, a, a, a, a, a, a, a, a, a, a, t)

class Impl(val tag: String) : F<Int> {
    override fun invoke(
        p00: A, p01: A, p02: A, p03: A, p04: A, p05: A, p06: A, p07: A, p08: A, p09: A,
        p10: A, p11: A, p12: A, p13: A, p14: A, p15: A, p16: A, p17: A, p18: A, p19: A,
        p20: A, p21: A, p22: Int,
    ): String = tag + p00.v + p22
}

fun main() {
    val a = A("a")
    val b = A("b")
    val ref = ::join
    println(ref(a, a, a, a, a, a, a, a, a, a, a, a, a, a, a, a, a, a, a, a, a, b, 1))
    println(call(ref, b, 2))

    var count = 0
    val lambda: F<Char> = { p00, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, c ->
        count++
        p00.v + c
    }
    println(call(lambda, a, 'x') + call(lambda, b, 'y') + count)
    println(lambda.invoke(b, a, a, a, a, a, a, a, a, a, a, a, a, a, a, a, a, a, a, a, a, a, 'z'))

    val impl = Impl("i")
    println(impl(a, a, a, a, a, a, a, a, a, a, a, a, a, a, a, a, a, a, a, a, a, a, 3) + call(impl, b, 4))

    val log = StringBuilder()
    val g: G = { x, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, y -> log.append(v + x.v + y.v) }
    a.g(b, a, a, a, a, a, a, a, a, a, a, a, a, a, a, a, a, a, a, a, a, a, a, b)
    val h: G? = if (count > 0) g else null
    h?.invoke(b, a, a, a, a, a, a, a, a, a, a, a, a, a, a, a, a, a, a, a, a, a, a, a, a)
    println(log)

    val any: Any = ref
    println("${any is Function23<*, *, *, *, *, *, *, *, *, *, *, *, *, *, *, *, *, *, *, *, *, *, *, *>} ${any is Function22<*, *, *, *, *, *, *, *, *, *, *, *, *, *, *, *, *, *, *, *, *, *, *>}")
}
