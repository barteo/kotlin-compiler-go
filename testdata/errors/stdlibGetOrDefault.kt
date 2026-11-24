// getOrDefault has Kotlin's signature, getOrDefault(key: K, defaultValue:
// V), on a Kotlin Map and on a Java one.

fun foo(x: Map<Int, String>, b: java.util.HashMap<Int, String>) {
    x.getOrDefault(1, "")
    x.getOrDefault(1, 2) // ERROR type mismatch: inferred type is Int but String was expected
    b.getOrDefault(1, "")
    b.getOrDefault(1, 2) // ERROR type mismatch: inferred type is Int but String was expected
}
