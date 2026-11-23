// The warnings kotlinc 2.4.21 reports with -Xuse-fir-experimental-checkers
// (not -Wextra) of the empty ranges of
// constants below, as TestWarningsEmptyRange reads them: each line ends
// with a comment naming the warnings it reports, if any. They follow its
// EmptyRangeChecker and the test extraCheckers/emptyRangeChecker of its
// diagnostics tests.
@file:Suppress("UNUSED_VARIABLE")

fun ranges(v: Int, c: Char) {
    for (i in 2..1) {} // EMPTY_RANGE
    for (i in 1..1) {}
    val a = 10..0 // EMPTY_RANGE
    if (v in -5 until -5) {} // EMPTY_RANGE
    if (v in 1 until 2) {}
    if (c in 'b'..'a') {} // EMPTY_RANGE
    if (0.0F in 0.02F..0.01F) {} // EMPTY_RANGE
    if (0.0 in 0.0..<0.0) {} // EMPTY_RANGE
    for (i in 1 downTo 2) {} // EMPTY_RANGE
    for (i in 2 downTo 1) {}
    for (i in 0..<-1) {} // EMPTY_RANGE
    for (i in v..1) {}
    val d = 1L..2L
    val e = 2L..1L // EMPTY_RANGE
}
