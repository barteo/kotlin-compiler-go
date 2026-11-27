// An explicit backing field is still the field where it is reported for
// a modifier, and visible in an inline property's private setter.
class A {
    val a: Number
        private field = 1 // ERROR modifier 'private' is not applicable to 'backing field'

    fun rest() = A().a + 10

    val s: Any
        field: String = ""

    inline var p: Any
        get() = ""
        private set(value) {
            s.length
        }
}

const val c: Any // ERROR a const property cannot have an explicit backing field
    field = 42

val d by lazy { 2 as Number }
    field = 10 // ERROR a delegated property cannot have a backing field
