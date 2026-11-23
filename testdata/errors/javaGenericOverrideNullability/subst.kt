// A Java override of a generic supertype's method has the nullability of
// its parameters there, through the type arguments: B1's bar(String x) of
// A<String>'s bar(@NotNull T x). B2's bar(Integer x) overrides nothing.
fun test(b1: B1, b2: B2) {
    b1.bar(null) // ERROR type mismatch: inferred type is Nothing? but String was expected
    b2.bar(null as Int?)
}
