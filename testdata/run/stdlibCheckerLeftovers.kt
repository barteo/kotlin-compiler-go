// Programs kc used to reject: the last check errors of the box tests.
import kotlin.properties.ReadOnlyProperty
import kotlin.reflect.KProperty

abstract class Your {
    abstract val your: String
    fun foo() = your
}

// field in an object expression of the getter.
val top: String = "O"
    get() = object : Your() {
        override val your = field
    }.foo() + "K"

class Holder {
    val member: String = "O"
        get() = object : Your() {
            override val your = field
        }.foo() + "K"
}

// tailrec on a final class's override, whose call with the inherited
// default isn't a tail call.
open class A {
    open fun foo(s: String = "OK") = s
}

class B : A() {
    override tailrec fun foo(s: String): String = if (s == "OK") s else foo()
}

// by ::f, a function's provideDelegate.
var captured = "fail"

class CaptureContext<T>(val capture: (T) -> Unit) : ReadOnlyProperty<T, () -> Unit> {
    override fun getValue(thisRef: T, property: KProperty<*>) = { -> capture(thisRef) }
}

operator fun <T> ((T) -> Unit).provideDelegate(thisRef: T, property: KProperty<*>) = CaptureContext(this)

fun right(arg: Right) { captured = "OK" }
class Right { val prop: () -> Unit by ::right }

// A provideDelegate whose receiver's bound a String isn't within.
open class Named(val value: String)

operator fun <P : Named> P.provideDelegate(host: Any?, p: Any): P = this
operator fun <V> V.getValue(receiver: Any?, p: Any): V = this

val named by Named("named")
val plain by "plain"

// The types of a lambda's parameters infer the call's type arguments.
fun <T, R> Iterator<T>.foldl(acc: R, f: (e: T, acc: R) -> R): R {
    var r = acc
    while (hasNext()) r = f(next(), r)
    return r
}

// And a variable with a lower bound is fixed before one with an upper.
fun <T, VR : T> pair(x: T, fn: (VR?, T) -> Unit) = fn(null, x)
fun takeInt(x: Int) = println("takeInt $x")

// A local of a member's name doesn't shadow a built-in member.
@JvmInline
value class Z(val s: String)

// @UnsafeVariance in a typealias.
typealias Action<K> = (@UnsafeVariance K) -> Unit
class Tag<out L>(val action: Action<L>)

// Overloads of a local function.
fun locals(): String {
    fun f(x: Int) = "int $x"
    fun f(x: String) = "string $x"
    return f(1) + ", " + f("a")
}

// A return from the code around an array's init lambda.
fun firstNegative(xs: IntArray): Int {
    IntArray(xs.size) { i -> if (xs[i] < 0) return i; xs[i] }
    return -1
}

// super in an object expression's header.
interface Factory { fun name(): String }

open class Base {
    open fun create(): Factory = object : Factory { override fun name() = "base" }
}

class Derived : Base() {
    override fun create(): Factory = object : Factory by super.create() {}
}

fun main() {
    println(top)
    println(Holder().member)
    println(B().foo("x"))
    Right().prop()
    println(captured)
    println(named.value + " " + plain)
    val sum = (1..1000).iterator().foldl(0) { e: Int, acc: Long -> acc + e }
    println(sum)
    val five = 5
    pair(five) { prev: Int?, new -> takeInt(new) }
    val z = Z("z")
    val hashCode = Z::hashCode
    println(hashCode(z) == z.s.hashCode())
    val toString = Z::toString
    println(toString(z))
    var got = ""
    Tag<String> { got = it }.action("tag")
    println(got)
    println(locals())
    println(firstNegative(intArrayOf(1, 2, -3, 4)))
    println(firstNegative(intArrayOf(1)))
    println(Derived().create().name())
    val labelled = listOf(1, 2, 3) zip listOf(4, 5, 6)
    println(labelled.map { (a, b) -> a + b })
}
