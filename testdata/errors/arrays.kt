fun two(vararg a: Int, vararg b: Int) {} // ERROR multiple vararg-parameters are prohibited

fun one(x: Int) = x

fun ints(vararg xs: Int) = xs.size

fun <T> make(): Array<T> = arrayOf<T>() // ERROR cannot use 'T' as reified type parameter

fun <T> sized(n: Int): Array<T?> = arrayOfNulls<T>(n) // ERROR cannot use 'T' as reified type parameter

fun main() {
    val a = intArrayOf(1)
    one(*a) // ERROR the spread operator (*foo) can only be applied in a vararg position
    val boxed = arrayOf(1, 2)
    ints(*boxed) // ERROR type mismatch
    ints(*1) // ERROR the spread operator can only be applied to arrays
    val n: IntArray? = null
    ints(*n) // ERROR the spread operator cannot be applied to a nullable array
    val x: Any = a
    println(x is Array<String>) // ERROR cannot check for instance of erased type
    a.size = 3 // ERROR val cannot be reassigned
    val s: Array<Any> = arrayOf("x")
    val t: Array<String> = arrayOf("y")
    val u: Array<Any> = t // ERROR type mismatch
    println(s.size + u.size)
}

fun withDefault(vararg xs: Int = longArrayOf()) {} // ERROR type mismatch
