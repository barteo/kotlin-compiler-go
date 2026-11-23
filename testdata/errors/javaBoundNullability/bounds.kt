// A Java type parameter whose only bound is annotated not-null is
// T : Any, and its uses in its class's or method's members aren't platform
// types, as in K2.

fun classes(g: G<String>, bad: G<String?>, s: String?) { // ERROR type argument is not within its bounds
    g.foo(null) // ERROR type mismatch
    g.f = null // ERROR type mismatch
    val f: String = g.f
    val b: JavaBox<String?> = g.box(JavaBox("")) // ERROR type mismatch
    val c: JavaBox<String> = g.box(JavaBox(""))
    g.maybe(null)
    g.maybe(s)
}

fun methods(s: String?) {
    M.id(null) // ERROR not within its bounds
    M.id(s) // ERROR not within its bounds
    M.id<String?>("") // ERROR not within its bounds
    val a: String = M.id("")
    M.cmp(s) // ERROR not within its bounds
    M.num(null)
    M.num<Int?>(null)
    M.two(null)
    M.plain(null)
    M.plain(s)
}

class K1 : G<String>() {
    override fun foo(a: String?): String? = a // ERROR overrides nothing
}

class K2 : G<String>() {
    override fun foo(a: String): String = a
}

class K3 : I<String> { // ERROR is not abstract and does not implement abstract member
    override fun apply(a: String?): String? = a // ERROR overrides nothing
}

fun <V> unbounded(g: G<V>) {} // ERROR type argument is not within its bounds
fun <V : Any> bounded(g: G<V>) {}
