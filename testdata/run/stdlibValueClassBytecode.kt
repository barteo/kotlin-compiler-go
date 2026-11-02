// Value classes' bytecode: an interface's default function called on a
// value class through a smart cast of this, whose interface is an
// indirect superinterface; as? to a generic value class; a value class
// override of an interface function whose defaults it inherits; and a
// suspend fun interface returning Result, which a lambda implements.

import kotlin.coroutines.*

interface IBase {
    fun testDefault1() = if (this is B) this.foo() else "fail"
}

interface IFoo : IBase {
    fun foo(): String
    fun testDefault2() = if (this is B) this.foo() else "fail"
}

@JvmInline
value class B(val x: String) : IFoo {
    override fun foo() = x
}

fun IBase.test1() = if (this is IFoo) foo() else "fail"

@JvmInline
value class Wrap<T : Int>(val s: T)

fun <T : Int> casts(a1: Any, a2: Wrap<T>?, a3: Any?, a4: Any?): Int {
    val b1 = a1 as Wrap<T>
    val b2 = a2 as Wrap<T>
    val b3 = (a3 as Wrap<T>?) as Wrap<T>
    val b4 = (a4 as? Wrap<T>) as Wrap<T>
    val b5 = "x" as? Wrap<T>
    return b1.s + b2.s + b3.s + b4.s + (b5?.s ?: 100)
}

interface Sum {
    fun f(a: Long = 1L, b: Long = 2L, c: Long = 3L): Long
    fun g(a: Int = 10): String
}

@JvmInline
value class L(val x: Long) : Sum {
    override fun f(a: Long, b: Long, c: Long) = x + a + b + c
    override fun g(a: Int) = "$x/$a"
}

fun interface ResultProvider {
    suspend fun getResult(): Result<String>
}

var globalResult = "<empty>"

suspend fun handle(p: ResultProvider) {
    globalResult = p.getResult().getOrThrow()
}

suspend fun later(): String = suspendCoroutine { it.resume("later") }

fun builder(c: suspend () -> Unit) {
    c.startCoroutine(Continuation(EmptyCoroutineContext) { it.getOrThrow() })
}

fun main() {
    println(B("OK").test1())
    println(B("OK").testDefault1())
    println(B("OK").testDefault2())
    println(casts(Wrap(1), Wrap(2), Wrap(3), Wrap(4)))
    println(L(1000).f())
    println(L(1000).f(b = 20))
    println(L(7).g())
    val s: Sum = L(5)
    println(s.f(c = 300))
    builder { handle { Result.success("OK") } }
    println(globalResult)
    builder { handle { Result.success(later()) } }
    println(globalResult)
    println(runCatching { builder { handle { Result.failure(IllegalStateException("boom")) } } }.exceptionOrNull()?.message)
}
