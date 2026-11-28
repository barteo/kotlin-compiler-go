// Errors kotlinc reports about supertypes and declarations, where kc used
// to report more after them.

class Unresolved : Missing() { // ERROR Missing
    override fun foo() {}
    override val bar: Int get() = 1
}

interface BadArity<T> {
    fun foo()
}

class Arity : BadArity { // ERROR one type argument expected
    override fun foo() {}
}

open class Base {
    open fun foo() {}
    fun fixed() {}
}

interface WithClass : Base { // ERROR an interface cannot inherit from a class
    override fun foo() {}
    override fun fixed() {} // ERROR final and cannot be overridden
    override fun nothing() {} // ERROR overrides nothing
}

interface Root {
    fun root() {}
}

interface Cyc1 : Root, Cyc3 {} // ERROR cycle
interface Cyc2 : Cyc1 {} // ERROR cycle
interface Cyc3 : Cyc2 {} // ERROR cycle

fun cycle(a: Cyc1, b: Cyc2) {
    a.root()
    b.root() // ERROR unresolved reference: root
}

class Props {
    val Any.x1: String // ERROR extension property must have accessors
    var Any.x2: String // ERROR extension property must have accessors
        get() = ""

    constructor(s: String) {
        x1 = s
        x2 = s
        x1.length
    }
}

val Any.top: Int // ERROR extension property must have accessors

fun useTop() = 1.top + 1
