// Definitely non-nullable types.

fun <T : Any> nonNullBound(x: T & Any) = x // ERROR intersection types are supported only for definitely non-nullable types: left part must be a type parameter with nullable bounds
fun notParam(x: String & Any) = x // ERROR left part must be a type parameter with nullable bounds
fun <T> notAny(x: T & CharSequence) = x // ERROR right part must be non-nullable 'Any'
fun <T> nullableLeft(x: T? & Any) = x // ERROR left part must be a type parameter with nullable bounds

fun <T> passNull(x: T & Any) = x
fun <T> assign(x: T): T & Any {
    val y: T & Any = x // ERROR type mismatch
    return y
}

fun main() {
    passNull<String?>(null) // ERROR type mismatch: inferred type is Nothing? but String was expected
}
