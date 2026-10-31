inline fun rec(n: Int): Int = if (n == 0) 0 else rec(n - 1) // ERROR cannot be recursive

inline fun run2(crossinline f: () -> Unit) {
    f()
}

fun g() {
    run2 { return } // ERROR is not allowed here
}

fun h() {
    inline fun local() {} // ERROR local inline functions are not yet supported
}

class Hidden {
    private val secret = 1
    internal fun helper() = 2
    @PublishedApi internal fun published() = 3
    inline fun leaks() = secret // ERROR public-API inline function cannot access non-public-API property 'secret'
    inline fun calls() = helper() // ERROR public-API inline function cannot access non-public-API function 'helper'
    inline fun allowed() = published()
    private inline fun inside() = secret + helper()
    open inline fun <reified T> opened(x: Any) = x is T // ERROR only type parameters of inline functions can be reified
}

inline fun cycleA(x: Int = cycleB()): Int = x // ERROR the call of 'cycleB' is a part of an inline call cycle
inline fun cycleB(x: Int = cycleA()): Int = x // ERROR the call of 'cycleA' is a part of an inline call cycle
inline fun argA(x: Int = argB(42)): Int = x
inline fun argB(x: Int = argA(42)): Int = x
inline fun bodyA(): Int = bodyB() // ERROR the call of 'bodyB' is a part of an inline call cycle
inline fun bodyB(): Int = bodyA() // ERROR the call of 'bodyA' is a part of an inline call cycle

interface WithDefault {
    fun h(x: Int = 3, block: () -> String): Any
}
open class InheritsDefault : WithDefault {
    final override inline fun h(x: Int, block: () -> String) = block() // ERROR functional parameters with inherited default values are not yet supported in inline functions
}
open class InlineH {
    inline fun h(x: Int, block: () -> String) = block()
}
class IntersectsDefault : InlineH(), WithDefault // ERROR functional parameters with inherited default values are not yet supported in inline functions
