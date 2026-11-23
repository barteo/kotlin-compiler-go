// A property with context parameters has no backing field.
class A

context(c: A)
var prop: String
    get() = ""
    set(value) {
        field = value // ERROR unresolved reference: field
    }

fun main() {}

// One that would need a field is reported at its context parameters, and
// checked as if it had one, as kotlinc does.
context(c: A) // ERROR property with context parameters cannot be initialized because it has no backing field
val initialized: String = ""

context(c: A) // ERROR property with context parameters cannot be initialized because it has no backing field
val withField: Any
    field = 1

context(c: A)
var withAccessors: String = "" // ERROR initializer is not allowed here because this property has no backing field
    get() = ""
    set(value) {
        field = value // ERROR unresolved reference: field
    }
