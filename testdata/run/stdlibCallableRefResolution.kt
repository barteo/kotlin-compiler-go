// Callable references resolved with the calls they are passed to, and
// among overloads: a reference with one candidate of the arity the
// parameter's function type takes gives the call's variables its
// function's types, a reference without an expected type names the
// most specific of its functions, and one passed to an overloaded
// function drops the candidates it doesn't fit.
import kotlin.reflect.KFunction1
import kotlin.reflect.KFunction2
import kotlin.reflect.KProperty0
import kotlin.reflect.KProperty1

interface B<E, SC> { fun show(): String }
class Inv<T>
class Foo<T>(val x: Int) : B<T, Inv<T>> { override fun show() = "Foo($x)" }

fun <T1, T2, S> bar(list: T2, fn: (S) -> B<T1, T2>): String = "bar"

fun sum(vararg ints: Int) = println("sum ${ints.sum()}")
fun <T> myLet(block: (T) -> Unit) = println("myLet")
fun <T> myLet(t: T, block: (T) -> Unit) = block(t)

class Doubler { fun twice(x: String): Int = x.length * 2 }
fun <T, E> apply1(x: (T) -> E, t: T): E = x(t)
fun <T, E> apply2(x: (Doubler, T) -> E, t: T): E = x(Doubler(), t)

open class Base
class Derived : Base()
fun Base.describe() = "base"
fun Derived.describe() = "derived"
fun pick(a: Any) = "any $a"
fun pick(a: Int) = "int $a"

fun <T, R> call1(x: KFunction1<T, R>) = "KFunction1"
fun <T, R> call1(x: KProperty1<T, R>) = "KProperty1"

class Sample {
    fun over(): String = "over()"
    fun over(x: Int): String = "over($x)"
}

object Picker {
    fun <T> take(fn: KFunction1<T, Boolean>): String = "one"
    @JvmName("take2") fun <T, U> take(fn: KFunction2<T, U, Boolean>): String = "two"
}
class Plain
fun Plain.test() = true

fun <T : Any, V> prop(p: KProperty1<T, V>) = "KProperty1 ${p.name}"
fun <V> prop(p: KProperty0<V>) = "KProperty0 ${p.name}"
class Entry(val key: String)

fun interface Action { fun run() }
fun interface Job<V> { fun call(): V }
fun schedule(a: Action) = "Action"
fun <V> schedule(j: Job<V>) = "Job ${j.call()}"

class Logger {
    fun flush(): String = "flushed"
    fun start() = schedule(this::flush)
}

typealias Direct = Pair<String, Int>

object Scope {
    fun foo(): Int = 1

    object Nested {
        @Deprecated("hidden", level = DeprecationLevel.HIDDEN)
        fun foo(): String = "hidden"

        fun <T> take(f: () -> T): T = f()

        fun test() {
            println("take ${take(::foo)}")
            println("call ${foo()}")
        }
    }
}

fun main() {
    println(bar(Inv<String>(), ::Foo))
    myLet(::sum)
    myLet(intArrayOf(1, 2, 3), ::sum)
    println(apply1(Doubler()::twice, "abc"))
    println(apply2(Doubler::twice, "abcd"))

    val d = Derived::describe
    println(d(Derived()))
    val p = ::pick
    println(p(7))

    println(call1(Sample::over))
    println(Picker.take(Plain::test))
    println(prop(Entry::key))
    println(Logger().start())

    val pairs = listOf(Direct("a", 1), Direct("bb", 2))
    println(pairs.map(Direct::first))

    Scope.Nested.test()
}
