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

    val s: String // ERROR must be a subtype of the property's
        field: Any = 1

    val e // ERROR the type of a property with an explicit backing field must be written
        field = 1

    private val p: Any // ERROR must be less visible
        field: Int = 1

    val m: Any
        private field: Int = 1 // ERROR modifier 'private' is not applicable

    val f: Any // ERROR must be a subtype of the property's type
        field // ERROR must have a type or an initializer

    fun inc(): Int = n + 1

    public inline fun pub(): Int = n + 1 // ERROR unresolved reference
}

interface I {
    val y: Any
        field: Int = 1 // ERROR not allowed in an interface
}

val top: Any
    field: Int // ERROR must be initialized

fun outside(a: A): Int = a.n + 1 // ERROR unresolved reference

open class Holder(val s: Any)

val early: Any
    field = object : Holder(late) {} // ERROR variable 'late' must be initialized

val late: Any
    field = object : Holder(early) {
        fun lazily(): Any = late
    }

// As kotlinc 2.4: a field of a type the property's isn't a supertype of,
// in a property reported for its accessors too, and a field in a class
// that isn't initialized.
class Reported {
    val withGetter: Int // ERROR must be a subtype of the property's
        field: Number = 42
        get() = 30 // ERROR cannot have accessors

    val inferredWithGetter: Int // ERROR must be a subtype of the property's
        field = 3.14
        get() = 10 // ERROR cannot have accessors

    val uninitialized: Number
        field: Int // ERROR must be initialized

    val initializedProperty = listOf(1, 2) // ERROR cannot have an initializer
        field: MutableList<Int>
}
