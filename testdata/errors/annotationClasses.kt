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
annotation class Ok(val x: Int, val e: E, val inner: A8, val xs: IntArray)
enum class E { X }

fun g() = Ok(1, E.X, A8(), intArrayOf(1))

annotation class R
@receiver:R class NotExtension // ERROR this annotation is not applicable to target 'declaration' and use-site target '@receiver'
@receiver:R fun notExtension() {} // ERROR this annotation is not applicable to target 'declaration' and use-site target '@receiver'
fun @receiver:R String.extension() {}
val @receiver:R String.extension: Int get() = 1
