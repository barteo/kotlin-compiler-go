// As in K2, C::name also finds the functions of C's companion object,
// bound to it: a call of overloads that each take one of them is
// ambiguous, one that only one takes isn't.
open class Base

class KotlinClass : Base() {
    fun baz(x: CharSequence): Int = 1
    companion object {
        fun baz(x: String): Int = 1
    }
}

class KotlinClass2 : Base() {
    fun bar(x: CharSequence): Int = 1
    companion object {
        fun bar(x: String): Int = 1
    }
}

fun foo1(x: (String) -> Int) {}

fun foo3(x: (KotlinClass, CharSequence) -> Int) {}
fun foo3(x: (String) -> Int) {}

fun main() {
    foo1(KotlinClass::baz)
    foo3(KotlinClass::baz) // ERROR ambiguous call
    foo3(KotlinClass2::bar)
}
