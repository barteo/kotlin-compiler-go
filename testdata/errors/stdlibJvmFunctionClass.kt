// A type written as kotlin.jvm.functions.FunctionN is the class, which
// a lambda, an anonymous function or a reference isn't, as in K2. Kotlin's
// own FunctionN, imported by default, is the function type.
import kotlin.jvm.functions.Function0
import kotlin.jvm.functions.Function1 as F1

fun one(): Int = 1

val a: Function0<Int> = { 42 } // ERROR a function value isn't a Function0
val b: kotlin.jvm.functions.Function0<Int> = { 42 } // ERROR a function value isn't a kotlin.jvm.functions.Function0
val c: F1<Int, Int> = { x -> x } // ERROR a function value isn't a F1
val d: Function0<Int> = ::one // ERROR a function value isn't a Function0
val e: Function0<Int> = fun(): Int = 1 // ERROR a function value isn't a Function0
val f: Function1<String, String> = { it }
val g: () -> Int = { 42 }

fun locals() {
    val h: Function0<Int> = { 2 } // ERROR a function value isn't a Function0
    val i: kotlin.Function1<Int, Int> = { it }
    println(f("x") + g() + i(1))
}
