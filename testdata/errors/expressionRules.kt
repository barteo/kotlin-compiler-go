// Rules about expressions that kotlinc's expression checkers report:
// named arguments of a function type's call, super?., a super call that
// leaves a parameter to its default value, a named function as a body
// without braces, a label two receivers have, and a type parameter as a
// catch parameter's type.
fun named(f: (String) -> Unit, g: (s: String) -> Unit) {
    f(p1 = "") // ERROR named arguments are not allowed for function types
    g(s = "") // ERROR named arguments are not allowed for function types
    f("")
}

interface Defaults { fun foo(a: Int, b: Int = 0): Int }
open class Base : Defaults {
    override fun foo(a: Int, b: Int) = a + b
    open fun own(x: Int = 0) = x
    open fun many(vararg xs: Int) = xs.size
    fun g() {}
}
class Derived : Base() {
    override fun own(x: Int) = x
    fun calls() {
        super.foo(1) // ERROR super calls with default arguments are not allowed
        super.own() // ERROR super calls with default arguments are not allowed
        super.foo(1, 2)
        super.foo(b = 1, a = 2)
        super.many()
        super?.g() // ERROR the safe call operator is not allowed on super
    }
}

fun bodies(c: Boolean) {
    for (i in 0..1) fun named() {} // ERROR anonymous functions with names are prohibited
    if (c) fun other() {} // ERROR anonymous functions with names are prohibited
    for (i in 0..1) { fun inBlock() {} }
    if (c) fun() {}
}

class Dup : Base() {
    fun String.Dup() {
        this@Dup // ERROR ambiguous label: Dup
        super@Dup.g() // ERROR ambiguous label: Dup
    }
    fun lambda() = "x".run2 Dup@{ this@Dup.length }
}
fun <R> String.run2(f: String.() -> R) = f()
fun Int.ext() { "s".run2 { fun String.ext() { this@ext } } } // ERROR ambiguous label: ext

fun <T : Throwable> catchParam() {
    try { } catch (e: T) { } // ERROR non-reified type parameters cannot be used by catch parameters
}
inline fun <reified T : Throwable> catchReified() {
    try { } catch (e: T) { }
}

fun refs(param: String) {
    val a = ::param // ERROR references to variables aren't supported yet
    val local = "local"
    val b = ::local // ERROR references to variables aren't supported yet
    val c = { y: Int -> ::y } // ERROR references to variables aren't supported yet
}
fun shadowed() {}
fun byFunction(shadowed: Int) { val r: () -> Unit = ::shadowed }
class PropParam(val p: Int, q: Int) {
    init {
        ::p
        ::q // ERROR references to variables aren't supported yet
    }
}
val withField: Int = 1
    get() {
        ::field // ERROR references to variables aren't supported yet
        return field
    }

inline fun <reified T : Any> reifiedAny() {}
inline fun <reified T> reifiedAnything() {}
inline fun <reified F> dnn() {
    reifiedAny<F & Any>() // ERROR definitely-non-nullable type as a reified type argument
    reifiedAnything<Inv2<F & Any>>()
}
interface Inv2<T>

val Int.minusAssign: (Int) -> Unit get() = {}
class Ops { operator fun minus(x: Int) = this }
val Ops.plus: (Int) -> Ops get() = { this }
fun operators() {
    1 -= 2 // ERROR property 'minusAssign' cannot be used as operator
    var i = 1
    i -= 2 // ERROR assignment operators ambiguity
    var o = Ops()
    o = o + 1 // ERROR property 'plus' cannot be used as operator
    o -= 1
    o.plus(1)
    2 *= 3 // ERROR variable expected
}
