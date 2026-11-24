// A when branch whose conditions are all values that aren't null smart
// casts a nullable subject, a subject variable too.

fun length(s: String) = s.length

fun pick(a: String?, n: Int?): Int = when (a) {
    "b", "cc" -> length(a)
    "d", null -> -1
    else -> when (val m = n) {
        1, 2 -> m + 10
        else -> 0
    }
}

fun main() {
    println(pick("b", 1))
    println(pick("cc", 1))
    println(pick(null, 1))
    println(pick("x", 2))
    println(pick("x", null))
}
