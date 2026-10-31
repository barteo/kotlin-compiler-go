import kotlin.coroutines.Continuation
import kotlin.coroutines.EmptyCoroutineContext
import kotlin.coroutines.startCoroutine
import kotlin.reflect.KFunction

// SAM constructors given function values rather than lambdas, KRunnable(f),
// which wrap them as a function value passed for the interface is, and
// references to a fun interface's constructor, ::KRunnable, whose objects
// are equal for the same interface.

fun interface KRunnable {
    fun run()
}

fun interface KSupplier<T> {
    fun get(): T
}

fun interface Transform : (String) -> String

fun interface Greeter {
    fun greet(name: String): String

    fun twice(name: String) = greet(greet(name))
}

fun interface Loader {
    suspend fun load(): String
}

fun shout(s: String) = s.uppercase()

fun foo() = "foo"

fun test(a: Any?) {
    a as () -> Unit
    KRunnable(a).run()
}

// An interface's companion invoke, as for a class without a constructor
// that takes the arguments, and an overload whose type variable a lambda
// can't be: C is a String.
class TypeToken<T>

interface Context<C : Any> {
    val value: C

    companion object {
        operator fun <C : Any> invoke(type: TypeToken<C>, value: C): Context<C> =
            object : Context<C> {
                override val value = value
            }

        operator fun <C : Any> invoke(type: TypeToken<C>, getValue: () -> C): Context<C> =
            object : Context<C> {
                override val value get() = getValue()
            }
    }
}

fun <C : Any> context(getValue: () -> C): Context<C> = Context<C>(TypeToken()) { getValue() }

val ks1: (() -> String) -> KSupplier<String> = ::KSupplier
val ks2: (() -> String) -> KSupplier<String> = ::KSupplier
val kn: (() -> Number) -> KSupplier<Number> = ::KSupplier
val kr = ::KRunnable

fun main() {
    val f: () -> String = { "supplied" }
    println(KSupplier(f).get())
    val g: (String) -> String = ::shout
    println(Greeter(g).twice("hi"))
    println(Transform(g)("transform"))
    test { println("from Any?") }

    val r = Runnable { println("java lambda") }
    r.run()
    val h: () -> Unit = { println("java value") }
    Runnable(h).run()

    val plain: () -> String = { "loaded" }
    var loaded = ""
    suspend { loaded = Loader(plain).load() }.startCoroutine(Continuation(EmptyCoroutineContext) { })
    println(loaded)

    println(ks1 { "ref" }.get())
    println(ks1 == ks2)
    println(ks1 == kn)
    println(ks1.hashCode() == kn.hashCode())
    println(ks1 == (::KRunnable as Any))
    println(kr is KFunction<*>)
    kr { println("kr") }.run()

    println(ks1(::foo) == ks2(::foo))
    println(KSupplier(::foo) == ks1(::foo))

    println(Context(TypeToken(), "value").value)
    println(context { "lazy" }.value)
}
