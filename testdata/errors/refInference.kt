// Errors of callable references whose types kc infers.

import Obj.len

class B {
    fun B.ext(): Int = 1
    val B.prop: Int get() = 2

    fun test(b: B) {
        val r = B::ext // ERROR a reference to an extension declared in a class is not allowed
        val f: (B) -> Int = B::ext // ERROR a reference to an extension declared in a class is not allowed
        val p = B::prop // ERROR a reference to an extension declared in a class is not allowed
        val q = b::prop // ERROR a reference to an extension declared in a class is not allowed
    }
}

object Obj {
    val String.len: Int get() = length
}

fun obj() = String::len // ERROR a reference to an extension declared in a class is not allowed

fun top() {}

val v = ::top == 1 // ERROR operator '==' cannot be applied
