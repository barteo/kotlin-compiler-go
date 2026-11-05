// A const val's initializer must be a constant expression, as K2 rules.
val notConst = 1
fun f() = 2
object O { const val K = 3 }
enum class E { A }

const val ok1 = 1 + 2 * 3
const val ok2 = "a${ok1}b${'c'}${true}${null}"
const val ok3 = O.K + Int.MAX_VALUE
const val ok4 = "x" + 1u
const val ok5 = 1.0 / 3 > 0.1 && !false
const val ok6 = 'a' + 1

const val bad1 = notConst + 1 // ERROR only 'const val' can be used in constant expressions
const val bad2 = f() // ERROR const 'val' initializer should be a constant value
const val bad3 = if (ok1 > 0) 1 else 2 // ERROR const 'val' initializer should be a constant value
const val bad4 = "1" + null // ERROR const 'val' initializer should be a constant value
const val bad5 = 1u + 2u // ERROR const 'val' initializer should be a constant value
const val bad6 = ok1 is Int // ERROR const 'val' initializer should be a constant value
const val bad7 = E.A.name // ERROR const 'val' initializer should be a constant value
const val cyc: Int = cyc + 1 // ERROR const 'val' initializer should be a constant value

fun main() {}
