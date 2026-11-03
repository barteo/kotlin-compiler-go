// Explicit backing fields Kotlin forbids, and reads of the property that
// don't see its field's type.

class A {
    val n: Any
        field: Int = 1

    var v: Any // ERROR a 'var' property cannot have an explicit backing field
        field: Int = 1

    open val o: Any
        field: Int = 1 // ERROR must be final

    val g: Any
        field: Int = 1
        get() = 2 // ERROR cannot have accessors

    val i: Any = 1 // ERROR cannot have an initializer
        field: Int = 1

    val w: Any
        field: String = 1 // ERROR type mismatch

    val s: String
        field: Any = 1 // ERROR must be a subtype of the property's

    val e // ERROR the type of a property with an explicit backing field must be written
        field = 1

    private val p: Any // ERROR must be less visible
        field: Int = 1

    val m: Any
        private field: Int = 1 // ERROR modifier 'private' is not applicable

    val f: Any // ERROR must be initialized
        field // ERROR must have a type or an initializer

    fun inc(): Int = n + 1

    public inline fun pub(): Int = n + 1 // ERROR unresolved reference
}

interface I {
    val y: Any
        field: Int = 1 // ERROR not allowed in an interface
}

val top: Any // ERROR must be initialized
    field: Int // ERROR must be initialized

fun outside(a: A): Int = a.n + 1 // ERROR unresolved reference
