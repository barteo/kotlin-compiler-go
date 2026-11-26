annotation class A1(var x: Int) // ERROR an annotation parameter cannot be 'var'
annotation class A2(x: Int) // ERROR 'val' keyword is missing in annotation parameter
annotation class A3(val x: Int) {
    fun f() = 1 // ERROR members are prohibited in annotation classes
    val y: Int get() = 1 // ERROR members are prohibited in annotation classes
    init { } // ERROR members are prohibited in annotation classes
}
annotation class A4(val l: java.util.ArrayList<Int>) // ERROR invalid type of annotation member
annotation class A5(val s: String?) // ERROR annotation parameters cannot be nullable
annotation class A6(val a: Array<Array<String>>) // ERROR invalid type of annotation member
open class Base
annotation class A7 : Base() // ERROR annotation class cannot have supertypes
open annotation class A8 // ERROR modifier 'open' is not applicable to 'annotation class'
annotation fun f() {} // ERROR modifier 'annotation' is not applicable to 'top level function'
enum annotation class EA { X } // ERROR modifier 'annotation' is not applicable to 'enum class'
annotation class Ok(val x: Int, val e: E, val inner: A8, val xs: IntArray)
enum class E { X }

fun g() = Ok(1, E.X, A8(), intArrayOf(1))

annotation class R
@receiver:R class NotExtension // ERROR this annotation is not applicable to target 'declaration' and use-site target '@receiver'
@receiver:R fun notExtension() {} // ERROR this annotation is not applicable to target 'declaration' and use-site target '@receiver'
fun @receiver:R String.extension() {}
val @receiver:R String.extension: Int get() = 1

// A nested annotation's explicit type arguments must fit the expected
// type, which an annotation call doesn't infer from.
interface TI<T>
class TC<T> : TI<T>
annotation class TA1<T>(val x: Int)
annotation class TA2<T1, T2 : TI<T1>>(val x: TA1<TI<T2>>)
annotation class TA3(val x: TA2<Int, TC<Int>>)

@TA3(TA2<String, TC<String>>(TA1(40))) // ERROR type mismatch
fun wrongTypeArgs() {}

@TA3(TA2<Int, TC<Int>>(TA1(40)))
fun rightTypeArgs() {}
