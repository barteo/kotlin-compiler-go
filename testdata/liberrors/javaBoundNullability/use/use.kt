// A Java type parameter whose only bound is annotated not-null, read from
// a class file's type annotations.

import lib.G

fun classes(g: G<String>, bad: G<String?>, s: String?) { // ERROR type argument is not within its bounds
    g.foo(null) // ERROR type mismatch
    g.f = null // ERROR type mismatch
    val f: String = g.f
    G.cmp(s) // ERROR not within its bounds
    G.two(null)
}
