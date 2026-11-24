// A Java method whose last parameter is a vararg is not a binary
// operator: x + "" resolves to the extension, x.plus("") to the method.
operator fun Base.plus(s: String): Int = 0

operator fun Base.contains(s: String): Boolean = true

fun ok(x: Base) {
    val y = x + ""
    y.inc()
    val z = x.plus("")
    z.length
    val b: Boolean = "" in x
}

fun bad(x: Base) {
    val y = x + ""
    y.length // ERROR unresolved reference
}
