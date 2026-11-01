// Errors of callable references whose types kc infers.

class B {
    fun B.ext(): Int = 1

    fun test() {
        val r = B::ext // ERROR a reference to an extension declared in a class is not allowed
    }
}

fun top() {}

val v = ::top == 1 // ERROR operator '==' cannot be applied
