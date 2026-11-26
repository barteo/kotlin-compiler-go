// Callable references that take what they name from their context, as in
// K2: a generic extension's type arguments from the implicit receiver the
// reference is bound to, an overload from the function type that calls
// around the reference's call expect, and an overload from a KFunction
// type that explicit type arguments give.
import kotlin.reflect.KFunction0
import kotlin.reflect.KFunction2

class Box<T>(val v: T) {
    fun viaClass(): KFunction0<T> = ::unbox
}

fun <K> Box<K>.unbox(): K = v
fun <K> K.self(): K = this
fun <K> List<K>.second(): K = this[1]
fun <R> select(vararg x: R) = x[0]

fun Box<String>.viaExtension(): String {
    val f = ::unbox
    return f()
}

fun Box<String>.viaSelect(flag: Boolean): String {
    val f = select(::unbox, ::unbox)
    val g = if (flag) ::self else ::self
    return f() + g().v
}

fun foo(i: Int) = "foo(Int) $i"
fun foo(s: String) = "foo(String) $s"
fun <T> id(x: T): T = x
fun <T> both(x: T, y: T): T = x

fun resolve(a: Number, b: Int) = "resolve(Number, Int) $a $b"
fun resolve(a: Number, b: String) = "resolve(Number, String) $a $b"
fun <K> kfun(f: KFunction2<K, String, String>) = f

fun main() {
    println(Box("class").viaClass()())
    println(Box("ext").viaExtension())
    println(Box("sel").viaSelect(true))
    println(with(Box(41)) { ::unbox }() + 1)
    println(with(listOf("a", "bc")) { ::second }().length)
    println(buildList { add(1); add(2); val f = ::second; add(f()) })

    val f1: (Int) -> String = id(id(::foo))
    println(f1(1))
    val f2: (Int) -> String = both(id(::foo), ::foo)
    println(f2(2))
    val f3: (String) -> String = both(id(::foo), id(id(::foo)))
    println(f3("3"))
    println(id<(Int) -> String>(id(id(::foo)))(4))
    println(both(id(::foo), id(id<(String) -> String>(::foo)))("5"))

    val k = kfun<Number>(::resolve)
    println(k.name + ": " + k(6, "x"))
}
