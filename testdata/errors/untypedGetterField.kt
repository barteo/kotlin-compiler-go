// A getter that gives its property its type has no field, whatever its body.
val blockBody // ERROR must have a type
    get() {
        return field // ERROR unresolved reference: field
    }

val exprBody
    get() = field // ERROR unresolved reference: field

val typed: Int = 1
    get() {
        return field
    }

fun main() {}
