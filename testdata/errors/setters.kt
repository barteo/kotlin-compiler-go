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
