// The warnings kotlinc 2.4.21 reports of the extensions below, as
// TestWarningsShadow reads them: each line ends with a comment naming the
// warnings it reports, if any.
interface T1 { fun test1() {} }
fun T1.test1() {} // EXTENSION_SHADOWED_BY_MEMBER
interface T2 { fun test2(x: String) {} }
fun T2.test2(x: Any) {}
interface T3 { fun test3(x: Any) {} }
fun T3.test3(x: String) {} // EXTENSION_SHADOWED_BY_MEMBER
fun <T : Any?> T3.test3(x: T) {}
fun T3.test3(y: String, z: Int = 1) {}
interface T4 { fun <T> test4(x: T) {} }
fun T4.test4(x: String) {}
interface T5 { fun <T> test5(x: T) {} }
fun <T : Number> T5.test5(x: T) {} // EXTENSION_SHADOWED_BY_MEMBER
interface T6 { fun <T : List<Any>> test6(x: T) {} }
fun <T : Set<Any>> T6.test6(x: T) {}
interface T7 { fun test7(); fun test7b() }
fun T7?.test7() {}
typealias NT7 = T7?
fun NT7.test7b() {}
interface T9 { fun t9() }
typealias AT9 = T9
fun AT9.t9() {} // EXTENSION_SHADOWED_BY_MEMBER
interface T8 { fun f(x: Int); fun g(x: Int = 1); fun h(vararg x: Int); fun i(x: Int) }
fun T8.f(y: Int) {}
fun T8.g(x: Int) {} // EXTENSION_SHADOWED_BY_MEMBER
fun T8.h(x: Int) {}
fun T8.i(x: Int = 2) {}
fun T8.f(x: Int?) {}
fun T8.f(x: Nothing) {} // EXTENSION_SHADOWED_BY_MEMBER
open class Box<T> { fun put(x: T) {}; val v: T? = null; fun get(): T? = null }
fun Box<String>.put(x: String) {}
fun <T> Box<T>.put(x: T) {}
fun Box<String>.get() = "" // EXTENSION_SHADOWED_BY_MEMBER
val Box<Int>.v: Int get() = 1 // EXTENSION_SHADOWED_BY_MEMBER
class SBox : Box<String>()
fun SBox.put(x: String) {} // EXTENSION_SHADOWED_BY_MEMBER
fun SBox.put(x: CharSequence) {}
fun SBox.get(): String? = null // EXTENSION_SHADOWED_BY_MEMBER
data class D(val x: Int, private val y: Int) { internal fun ii() {}; protected fun pp() {}; private fun qq() {} }
fun D.component1() = 42 // EXTENSION_SHADOWED_BY_MEMBER
fun D.component2() = 42
fun D.copy(x: Int) = this
fun D.copy(x: Int, y: Int) = this // EXTENSION_SHADOWED_BY_MEMBER
fun D.ii() {} // EXTENSION_SHADOWED_BY_MEMBER
fun D.pp() {}
fun D.qq() {}
val D.x: Int get() = 1 // EXTENSION_SHADOWED_BY_MEMBER
val D.y: Int get() = 1
fun Any.toString() = "" // EXTENSION_SHADOWED_BY_MEMBER
fun String.hashCode() = 1 // EXTENSION_SHADOWED_BY_MEMBER
fun Int.equals(other: Any?) = true // EXTENSION_SHADOWED_BY_MEMBER
fun Short.equals(x: Any?) = true
infix fun Long.equals(other: Any?) = true
fun T1.toString() = "" // EXTENSION_SHADOWED_BY_MEMBER
fun T1.hashCode(x: Int) = 1
interface IBase { fun foo(); val bar: Int }
object Impl : IBase { override fun foo() {}; override val bar: Int get() = 42 }
object Del : IBase by Impl
fun Del.foo() {} // EXTENSION_SHADOWED_BY_MEMBER
val Del.bar: Int get() = 42 // EXTENSION_SHADOWED_BY_MEMBER
interface Inv { operator fun invoke(); operator fun invoke(s: String); fun invoke(i: Int) }
class HasInv(val w: Inv) { val n: Int = 0 }
fun HasInv.w() {} // EXTENSION_FUNCTION_SHADOWED_BY_MEMBER_PROPERTY_WITH_INVOKE
fun HasInv.w(s: String) {} // EXTENSION_FUNCTION_SHADOWED_BY_MEMBER_PROPERTY_WITH_INVOKE
fun HasInv.w(i: Int) {}
fun HasInv.n() {}
interface Op { fun invoke(); operator fun invoke(i: Int): Int; infix fun bar(i: Int): Int; fun baz(i: Int): Int }
operator fun Op.invoke() {}
operator fun Op.invoke(i: Int) = i // EXTENSION_SHADOWED_BY_MEMBER
infix fun Op.bar(i: Int) = i // EXTENSION_SHADOWED_BY_MEMBER
infix fun Op.baz(i: Int) = i
class Host {
    fun T1.test1() {} // EXTENSION_SHADOWED_BY_MEMBER
    val Box<Int>.v: Int get() = 1 // EXTENSION_SHADOWED_BY_MEMBER
    fun Int.foo() {}
}
fun Host.foo() {}
fun outer() {
    fun T1.test1() {} // EXTENSION_SHADOWED_BY_MEMBER
}
class Hid { @Deprecated("", level = DeprecationLevel.HIDDEN) fun h() {}; @Deprecated("") fun d() {} }
fun Hid.h() {}
fun Hid.d() {} // EXTENSION_SHADOWED_BY_MEMBER
enum class E { A }
val E.name: String get() = "" // EXTENSION_SHADOWED_BY_MEMBER
val Pair<Int, Int>.first: Int get() = 1 // EXTENSION_SHADOWED_BY_MEMBER
fun Pair<Int, Int>.component1() = 1 // EXTENSION_SHADOWED_BY_MEMBER
fun List<Int>.size() = 1
fun StringBuilder.length() = 1
fun <T> Box<T>.get(): T? = null
abstract class Abs<A> { abstract fun m(a: A) }
class Conc : Abs<Int>() { override fun m(a: Int) {} }
fun Conc.m(a: Int) {} // EXTENSION_SHADOWED_BY_MEMBER
fun <A> Abs<A>.m(a: A) {}
open class G2<X> { fun <Y : X> gm(y: Y) {} }
fun <Y : Number> G2<Number>.gm(y: Y) {}
fun <Y : String> G2<Number>.gm(y: Y) {} // FINAL_UPPER_BOUND
