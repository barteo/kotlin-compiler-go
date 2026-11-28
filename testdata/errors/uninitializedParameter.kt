// A default value can't use its own parameter or one after it.

fun f(a: Int = b, b: Int = 1): Int = a + b // ERROR parameter 'b' is uninitialized here

fun g(a: Int = a): Int = a // ERROR parameter 'a' is uninitialized here

fun h(a: Int = 1, b: Int = a): Int = a + b

fun outer(): Int {
    fun local(x: Int = y, y: Int = 2): Int = x + y // ERROR parameter 'y' is uninitialized here
    return local()
}
