// Errors kotlinc reports past a first error in a declaration.

open class A(x: Int) {
    constructor(z: String) : this(10)
}

// kotlinc's class still has a primary constructor, which this() can't
// choose from constructor().
class C : A(20) { // ERROR supertype initialization is impossible without a primary constructor
    constructor() // ERROR conflicting overloads
    constructor(z: String) : this() // ERROR ambiguous call
}

@Target(AnnotationTarget.TYPEALIAS)
annotation class Anno(val value: Int)

const val CONST = 1

// An inner typealias sees the class's instance members, an outer one
// doesn't.
class TopLevelClass {
    val CONST = "str"

    @Anno(CONST)
    typealias A = String

    @Anno(CONST) // ERROR type mismatch: inferred type is String but Int was expected
    inner typealias B = String // ERROR modifier 'inner' is not applicable to 'typealias'
}

class `_`<`__`>

val testConstructor = _<Any>() // ERROR can be used only in back-ticks
val testConstructor2 = `_`<Any>()

// A property its inference left an error for a cycle is reported where
// another property's inference reads it.
class Test {
    private val y = object {
        val a = y // ERROR type checking has run into a recursive problem
    }

    val z = y.a // ERROR type checking has run into a recursive problem
}
