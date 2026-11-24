// CONST_VAL_WITH_NON_CONST_INITIALIZER: a Java field whose initializer
// isn't constant isn't one to Kotlin either.

const val K = 1
const val a = J.SUM + 1
const val b = J.NAME
const val c = J.FROM_KOTLIN // kotlinc evaluates the Kotlin const val
const val d = J.TIME // ERROR const 'val' initializer

// A Java field that reads a const val back is a cycle, as kotlinc finds.
class CycleFoo {
    companion object {
        const val FOO = CycleBaz.BAZ + 1 // ERROR const 'val' initializer should be a constant value
    }
}
class CycleBaz {
    companion object {
        const val BAZ = J.BACK + 1 // ERROR const 'val' initializer should be a constant value
    }
}
