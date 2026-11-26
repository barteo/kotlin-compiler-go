// A constant of an enum's companion object, though inlined, is read from
// the companion as kotlinc sees it: uninitialized in the enum's
// constructor code.

enum class P(val m: Double) {
    A(G), // ERROR companion object of enum class 'P' is uninitialized here
    B(Companion.G * 2); // ERROR companion object of enum class 'P' is uninitialized here
    val g: Double = G * m // ERROR companion object of enum class 'P' is uninitialized here
    val h = m * 2
    fun f() = G * m
    companion object {
        const val G = 6.67e-11
    }
}
