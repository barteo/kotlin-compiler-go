// JVM signature clashes: of a getter with a function, of members with
// those they inherit but don't override, and with the methods and fields
// that kotlinc adds.

class G<T>

class PropAndFun {
    val x = 1
    fun getX() = 2 // ERROR platform declaration clash
    var y = 1
    fun setY(v: Int) {} // ERROR platform declaration clash
}

val topX = 1
fun getTopX() = 2 // ERROR platform declaration clash

val aa = 1
val Aa = 2 // ERROR platform declaration clash

val <T> G<T>.ext: Int get() = 1
val G<String>.ext: Int get() = 2 // ERROR platform declaration clash

fun local() {
    class L {
        val x = 1
        fun getX() = 2 // ERROR platform declaration clash
    }
}

open class Base {
    fun getZ() = 1
    open fun foo(t: G<String>) {}
}

class FromBase : Base() {
    val z = 2 // ERROR accidental override
    fun foo(t: G<Int>) {} // ERROR accidental override
}

interface Generic<T> {
    fun bar(t: T) {}
}

class Bridged : Generic<String> {
    override fun bar(t: String) {}
    fun bar(o: Any) {} // ERROR accidental override
}

interface WithFun {
    fun getW() = 1
}

interface WithProp {
    val w: Int get() = 2
}

class Both : WithFun, WithProp // ERROR inherited platform declarations clash

interface G1<T> {
    fun g(l: G<T>) {}
}

interface G2<T> {
    fun g(l: G<T>) {}
}

class TwoSubstitutions : G1<String>, G2<Int> // ERROR inherited platform declarations clash

class Defaults {
    fun `d$default`(c: Defaults, x: Int, m: Int, mh: Any) {} // ERROR platform declaration clash
    fun d(x: Int = 1) {} // ERROR platform declaration clash
}

class Overloads {
    @JvmOverloads fun o(s: String = "") {} // ERROR platform declaration clash
    fun o() {} // ERROR platform declaration clash
}

enum class E { // ERROR platform declaration clash
    A;
    fun values(): Array<E> = arrayOf() // ERROR platform declaration clash
}

object O { // ERROR platform declaration clash
    lateinit var INSTANCE: O // ERROR platform declaration clash
}

class Finals {
    fun notify() {} // ERROR accidental override
    fun getClass(): Class<Any>? = null // ERROR accidental override
}

// None of these clash.

class Overrides : Generic<Int> {
    override fun bar(t: Int) {}
}

open class Value<T> {
    open fun get(): T? = null
}

class IntValue : Value<Int>() {
    override fun get(): Int = 1
    val data = 1
}

interface Same1 {
    fun s(x: G<String>) {}
}

interface Same2<T> {
    fun s(x: G<T>) {}
}

class SameMember : Same1, Same2<String> {
    override fun s(x: G<String>) {}
}

interface Top<D> {
    fun getData(): D
}

abstract class DefaultImpl : Top<Int>

class Bottom(val data: Int) : DefaultImpl() {
    override fun getData(): Int = data
}

class PrivateProp(private val p: Int) {
    fun getP() = p
}

object PlainInstance {
    val INSTANCE: PlainInstance? = null
}

fun wait(x: Int) {}

// An unresolved receiver isn't left out of the signature.
fun Nope.unknownRecv(x: Int) {} // ERROR unresolved
fun unknownRecv(x: Int) {}
