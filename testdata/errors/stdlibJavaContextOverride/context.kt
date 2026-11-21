// A Java override of a Kotlin member with context parameters, whose JVM
// method takes them first, is seen as the Kotlin member: called with its
// context arguments, not with them as its first parameters.
interface KotlinContextInterface {
    context(a: String)
    fun foo(b: Int): String

    context(a: String)
    val bar: Double
}

fun usage(a: JavaWithOverride) {
    a.foo(1) // ERROR no context argument
    a.foo("", 1) // ERROR type mismatch
    a.getBar("") // ERROR unresolved reference
    with("OK") {
        a.foo(1)
    }
}
