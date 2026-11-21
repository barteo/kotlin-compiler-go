// A property with context parameters has no backing field.
class A

context(c: A)
var prop: String
    get() = ""
    set(value) {
        field = value // ERROR unresolved reference: field
    }

fun main() {}
