// Each read of a variable before it is assigned is reported, and a
// const's by another const's initializer.
val a: Int = b // ERROR variable 'b' must be initialized
val b: Int = a + b // ERROR variable 'b' must be initialized

class C {
    val a: Int = b // ERROR variable 'b' must be initialized
    val b: Int = a + b // ERROR variable 'b' must be initialized

    companion object {
        const val A: String = B // ERROR variable 'B' must be initialized
        const val B: String = "CONSTANT"
        val tConst: String = Z
        init { println(Z) }
        const val Z: String = "CONSTANT"
    }
}

fun foo() {
    val a: Int
    a + 1 // ERROR variable 'a' must be initialized
    a + 1 // ERROR variable 'a' must be initialized
}

val tConst: String = Z
const val Z: String = "CONSTANT"
const val X: String = Y // ERROR variable 'Y' must be initialized
const val Y: String = "CONSTANT"
