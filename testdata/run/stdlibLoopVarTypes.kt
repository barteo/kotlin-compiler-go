// Loop variables with types, for (i: Int? in r), which may be supertypes
// of the elements', and do-while loops without bodies.

fun nullable(i: Int?) = i

fun main() {
    var s = ""
    for (i: Int? in 1..3) s += nullable(i)
    for (i: Long? in 1 until 3L) s += i
    for (c: Char? in "ab") s += c
    for (x: Any in doubleArrayOf(1.5, 2.5)) s += x
    for (x: Number in listOf(4, 5)) s += x.toInt()
    for (i: Int in 6 downTo 5) s += i
    for (x: Comparable<*> in arrayOf("z")) s += x
    println(s)

    var n = 0
    do while (n++ < 5);
    do while (false)
    println(n)
}
