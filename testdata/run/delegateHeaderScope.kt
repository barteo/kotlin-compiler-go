// A delegation's expression, like the superclass constructor's arguments,
// sees the class's nested classes and companion.
interface I { fun v(): Int }
class S(val n: Int) : I { override fun v() = n }

class A : I by S(Nested().x + Companion.CONST + CONST + Nested.K + Iface.Q + make()) {
    class Nested {
        val x = 1
        companion object { const val K = 10 }
    }
    interface Iface { companion object { const val Q = 100 } }
    companion object {
        const val CONST = 1000
        fun make() = 10000
    }
}

class P(p: Int) : I by S(p + Nested().x) {
    class Nested { val x = 5 }
}

fun main() {
    println(A().v())
    println(P(1).v())
}
