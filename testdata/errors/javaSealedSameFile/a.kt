// A Java sealed class or interface without a permits clause permits the
// subclasses in its file, as javac infers them, so a when over them is
// exhaustive.
fun area(s: Shape): Int = when (s) {
    is Shape.Circle -> 1
    is Shape.Poly -> 2
}

fun sides(s: Shape): Int = when (s) {
    is Shape.Circle -> 0
    is Shape.Poly.Tri -> 3
    is Shape.Poly.Quad -> 4
}

fun sum(n: Node): Int = when (n) {
    is Node.Leaf -> n.value()
    is Node.Pair -> sum(n.left()) + sum(n.right())
}

fun partial(s: Shape): Int = when (s) { // ERROR must be exhaustive
    is Shape.Circle -> 1
    is Shape.Poly.Tri -> 3
}
