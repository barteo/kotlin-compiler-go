// Branches of platform types join to a platform type, which a non-null
// parameter takes, as kotlinc infers it.
sealed class C {
    data object A : C()
    data object B : C()
    data object N : C()
}
class Line(val c: C)
fun f(k: Int, o: Any): Line {
    val c = when (k) {
        1 -> C.A::class.java.cast(o)
        2 -> C.B::class.java.cast(o)
        else -> C.N
    }
    return Line(c)
}
fun g(k: Int, o: Any): Line {
    val c = if (k == 1) C.A::class.java.cast(o) else C.N
    return Line(c)
}
fun h(k: Int, o: Any): Line {
    val c = when (k) {
        1 -> C.A::class.java.cast(o)
        else -> C.B::class.java.cast(o)
    }
    return Line(c)
}
fun main() { println(f(1, C.A).c); println(g(2, C.A).c); println(h(1, C.A).c) }
