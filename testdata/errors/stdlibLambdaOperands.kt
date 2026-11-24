// An operator's operand is checked with a hint, not an expected type: the
// operator is a call, as (0..2).contains(n) or " ".plus(d). A lambda's
// result doesn't contradict it.
fun operands(n: Int?): String {
    if ({ n }.let { it() } in 0..2) return "in"
    if (run { n } !in 0..2) return "!in"
    val s: String = run { "" } + run { 1.5 }
    return " " + run { 1.5 }
}

fun expected(n: Int?): Int = run {
    n // ERROR type mismatch
}
