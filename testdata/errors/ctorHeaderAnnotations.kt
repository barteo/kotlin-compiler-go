// The arguments of the annotations of a constructor and its parameters
// don't see the class's instance members, as kotlinc's: CONST is the
// top-level constant there, and the member in the class's members. Those
// of its parameters see its parameters, wherever they are, which the
// constructor's own don't.
@Retention(AnnotationRetention.SOURCE)
@Target(AnnotationTarget.CONSTRUCTOR, AnnotationTarget.VALUE_PARAMETER, AnnotationTarget.PROPERTY, AnnotationTarget.FUNCTION)
annotation class Anno(val value: Int)

const val CONST = 1

class C @Anno(CONST) constructor(@Anno(CONST) x: String, @Anno(CONST) val y: String) {
    @Anno(CONST) constructor(@Anno(CONST) i: Int) : this("", "")

    @Anno(CONST) fun f() {} // ERROR type mismatch
    fun g(@Anno(CONST) p: Int) {} // ERROR type mismatch
    @Anno(CONST) val q = 1 // ERROR type mismatch

    val CONST = "str"
}

class Outer {
    val CONST = "str"

    // An inner class's constructors see the outer class's.
    inner class Inner @Anno(CONST) constructor() // ERROR type mismatch
    class Nested @Anno(CONST) constructor(@Anno(CONST) x: Int)
}

class P1(@Anno(OTHER) x: Int, OTHER: String) // ERROR type mismatch
class P2(OTHER: String, @Anno(OTHER) x: Int) // ERROR type mismatch
class P3(@Anno(OTHER) val OTHER: String) // ERROR type mismatch
class P4 @Anno(OTHER) constructor(OTHER: String)
class P5 {
    constructor(OTHER: String, @Anno(OTHER) x: Int) // ERROR type mismatch
    @Anno(OTHER) constructor(OTHER: Int, y: Int)
}

const val OTHER = 1
