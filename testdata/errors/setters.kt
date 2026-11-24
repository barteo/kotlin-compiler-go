class A {
    var y: Int = 0
        set(v: String) {} // ERROR setter parameter type must be equal to the type of the property, i.e. 'Int'
}

class B {
    val noField: Int = 1 // ERROR initializer is not allowed here because this property has no backing field
        get() = 2
    val needsInit: Int // ERROR property must be initialized or be abstract
        get() = field
}

interface I {
    var v: Int // ERROR property in an interface cannot have a backing field
        get() = 1
}

open class C {
    open var w = 1
        private set // ERROR private setters are not allowed for open properties
}

fun f() {
    val a = A()
    a.y = 1
}

// A setter's parameter and a getter's return type of another type than the
// property's: in the accessor, they are of the type written.
class WrongAccessorTypes {
    var x: Int = 0
        set(value: String) { // ERROR setter parameter type must be equal to the type of the property, i.e. 'Int'
            field = value // ERROR type mismatch: inferred type is String but Int was expected
        }
    val y: Int
        get(): String = "s" // ERROR getter return type must be equal to the type of the property, i.e. 'Int'
    val e = 1
        get(): String { // ERROR getter return type must be equal to the type of the property, i.e. 'Int'
            return field // ERROR type mismatch: inferred type is Int but String was expected
        }
    val same = 1
        get(): Int = field
}
