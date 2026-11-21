import p.*

fun box(b: Box<Int>): Int = b.self().v

fun <N : Node<N>> last(n: N): N = n.next()?.let { last(it) } ?: n

fun test() {
    box(Box(1))
    last(Leaf())
    // lib2's Box is a type of its own, as in K2.
    plain(Box(1).self()) // ERROR type mismatch: inferred type is Box<Int> but Box was expected
}
