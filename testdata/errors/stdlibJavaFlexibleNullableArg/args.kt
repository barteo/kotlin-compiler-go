// A nullable argument passed for a Java parameter of a type variable's
// flexible type, String? <: T!, is K2's String? <: T?: an equality
// constraint T = String still holds, so JavaBox(s) is a JavaBox<String>
// where one is expected, but a T with no such constraint is a String?.

fun take(b: JavaBox<String>) {}
fun takeP(p: P<String>) {}
fun ret(s: String?): JavaBox<String> = JavaBox(s)
class KB<T>(t: T)
fun takeKB(b: KB<String>) {}

fun accepted(s: String?, n: String, c: Boolean) {
    take(JavaBox(s))
    val b: JavaBox<String> = JavaBox(s)
    take(J.wrap(s))
    take(JavaBox(null))
    takeP(P(s, n))
    takeP(P(n, s))
    takeP(P.of(s, s))
    val e: JavaBox<String> = JavaBox(if (c) s else null)
    val r: JavaBox<in String> = JavaBox(s)
    val l: List<JavaBox<String>> = listOf(JavaBox(s))
    val m: MutableList<String> = J.listOf(s)
}

fun rejected(s: String?) {
    val y: String = J.transform(s) // ERROR type mismatch
    val z = JavaBox(s)
    take(z) // ERROR type mismatch
    val l: List<String> = J.listOf(s) // ERROR type mismatch
    take(JavaBox<String?>(s)) // ERROR type mismatch
    takeKB(KB(s)) // ERROR type mismatch
    val q: JavaBox<out String> = JavaBox(s) // ERROR type mismatch
}
