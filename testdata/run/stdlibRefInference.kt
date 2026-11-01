// Callable references whose types kc infers from their receivers, their
// overloads and their uses, with the standard library: see refInference.kt.

import kotlin.reflect.KCallable
import kotlin.reflect.KProperty

class B {
    companion object {
        fun foo(): String = "B.Companion.foo"
    }
}

fun B.foo(): String = "B.foo"

fun call(f: Any): String = if (f is Function0<*>) f.invoke() as String else (f as Function1<B, String>).invoke(B())

val <T> List<T>.second: T
    get() = this[1]

fun top(): Int = 1
fun Int.intExt(): Int = this

fun reg(vararg items: KProperty<*>): String = "properties"
fun reg(vararg items: KCallable<*>): String = "callables"
val p = 0

fun main() {
    println(call(B::foo))
    val second = List<String>::second
    println(second(listOf("x", "y")))
    println(::top == Int::intExt)
    println(Int::intExt == Int::intExt)
    println(reg(::p))
    println(reg(::main))
    val k: KCallable<*> = ::top
    println(k.name)
}
