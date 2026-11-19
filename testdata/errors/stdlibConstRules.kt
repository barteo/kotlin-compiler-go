enum class E { A }

const val ok1 = 1
const val ok2: Long = 1
const val ok3 = "s" + 1
const val ok4 = 1.0 / 0
const val ok5 = 1.0 % 0
const val ok6 = 1.0.div(0)
const val ok7 = ok1 + 1
const val ok8 = 10 / 2
const val ok9 = 1u
typealias IntAlias = Int
const val ok10: IntAlias = 1

const val e: E = E.A // ERROR const 'val' has type 'E'
const val arr: IntArray = intArrayOf() // ERROR const 'val' has type
const val range = 1.rangeUntil(2) // ERROR const 'val' has type
const val n: String? = null // ERROR const 'val' has type

object O {
    const val noInit: Int // ERROR const 'val' should have an initializer
    init {
        noInit = 1
    }
}

const val div0 = 1 / 0 // ERROR const 'val' initializer should be a constant value
const val rem0 = 1 % (1 - 1) // ERROR const 'val' initializer should be a constant value
const val div0Call = 0.div(0) // ERROR const 'val' initializer should be a constant value
const val incCall = ok1.inc() // ERROR const 'val' initializer should be a constant value

const val bad = { -10 }() // ERROR const 'val' initializer should be a constant value
const val copy = bad // ERROR const 'val' initializer should be a constant value

object A {
    const val r1: Int = 1 + B.r2 // ERROR const 'val' initializer should be a constant value
}

class B {
    companion object {
        const val r2: Int = A.r1 + 2 // ERROR const 'val' initializer should be a constant value
    }
}
