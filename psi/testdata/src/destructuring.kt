// Destructuring forms (Kotlin 2.4 PSI).
data class P(val a: Int, val b: String)

fun forms(p: P, list: List<P>) {
    val (a, b) = p
    val (x: Int, y) = p
    val [first, second] = p
    var [m, n: String] = p
    (val a1, var b1) = p
    (val a2 = a, val b2: String = b) = p
    [val c1, var c2] = p
    (val renamed = b) = p
    val (q = a) = p
    [val e1 = a] = p
    (a3, val b3) = p
    (val a4, b4) = p

    for ((k, v) in list) {}
    for ([k, v] in list) {}
    for ((val k = a, val v = b) in list) {}
    for ([val k, val v] in list) {}

    list.forEach { (a, b) -> }
    list.forEach { [a, b] -> }
    list.forEach { (val a, val b = b) -> }
    list.forEach { [val a, val b]: P -> }
    list.forEach { (var a) -> }

    val () = p
    val [] = p
    val (a5, b5 = p
    val [a6, b6 = p
}

val (topA, topB) = P(1, "")
