// Loops over reversed() of stepped progressions and over reversed()
// progressions with a step, which kotlinc compiles as counted loops with
// the bounds computed in place. TestReversedLoops checks that the
// functions named counted* call no reversed, step or iterator.

fun countedSteppedReversed(a: Int, b: Int, n: Int): List<Int> { val l = mutableListOf<Int>(); for (i in (a..b step n).reversed()) l += i; return l }
fun countedDownToSteppedReversed(a: Int, b: Int, n: Int): List<Int> { val l = mutableListOf<Int>(); for (i in (a downTo b step n).reversed()) l += i; return l }
fun countedConstSteppedReversed(a: Int): List<Int> { val l = mutableListOf<Int>(); for (i in (a..20 step 3).reversed()) l += i; return l }
fun countedLongSteppedReversed(a: Long, b: Long, n: Long): List<Long> { val l = mutableListOf<Long>(); for (i in (a..b step n).reversed()) l += i; return l }
fun countedLongDownToSteppedReversed(a: Long, b: Long): List<Long> { val l = mutableListOf<Long>(); for (i in (a downTo b step 2L).reversed()) l += i; return l }
fun countedCharSteppedReversed(a: Char, b: Char): String { val s = StringBuilder(); for (c in (a..b step 2).reversed()) s.append(c); return s.toString() }
fun countedCharDownToSteppedReversed(a: Char, b: Char, n: Int): String { val s = StringBuilder(); for (c in (a downTo b step n).reversed()) s.append(c); return s.toString() }

fun countedIndicesReversedStep(a: IntArray, n: Int): List<Int> { val l = mutableListOf<Int>(); for (i in a.indices.reversed() step n) l += i; return l }
fun countedListIndicesReversedStep(a: List<String>): List<String> { val l = mutableListOf<String>(); for (i in a.indices.reversed() step 2) l += a[i]; return l }
fun countedReversedStep(a: Int, b: Int, n: Int): List<Int> { val l = mutableListOf<Int>(); for (i in (a..b).reversed() step n) l += i; return l }
fun countedDownToReversedStep(a: Int, b: Int, n: Int): List<Int> { val l = mutableListOf<Int>(); for (i in (a downTo b).reversed() step n) l += i; return l }
fun countedLongReversedStep(a: Long, b: Long): List<Long> { val l = mutableListOf<Long>(); for (i in (a..b).reversed() step 3L) l += i; return l }
fun countedCharReversedStep(a: Char, b: Char): String { val s = StringBuilder(); for (c in (a..b).reversed() step 2) s.append(c); return s.toString() }

fun countedLongReversed(a: Long, b: Long): List<Long> { val l = mutableListOf<Long>(); for (i in (a..b).reversed()) l += i; return l }
fun countedLongDownToReversed(a: Long, b: Long): List<Long> { val l = mutableListOf<Long>(); for (i in (a downTo b).reversed()) l += i; return l }
fun countedCharReversed(a: Char, b: Char): String { val s = StringBuilder(); for (c in (a..b).reversed()) s.append(c); return s.toString() }
fun countedCharDownToReversed(a: Char, b: Char): String { val s = StringBuilder(); for (c in (a downTo b).reversed()) s.append(c); return s.toString() }
fun countedCharConstReversed(): String { val s = StringBuilder(); for (c in ('a'..'e').reversed()) s.append(c); return s.toString() }

fun countedBreakContinue(a: Int, b: Int): List<Int> {
    val l = mutableListOf<Int>()
    for (i in (a..b step 2).reversed()) {
        if (i % 3 == 0) continue
        if (i < 4) break
        l += i
    }
    return l
}

fun countedCaptured(a: Long, b: Long): List<() -> Long> {
    val fs = mutableListOf<() -> Long>()
    for (i in (a..b step 2L).reversed()) fs += { i }
    return fs
}

var log = StringBuilder()
fun <T> v(x: T): T { log.append(x).append(' '); return x }
fun countedOrder(): List<Any> {
    val l = mutableListOf<Any>()
    for (i in (v(1)..v(9) step v(4)).reversed()) l += i
    for (i in (v(9L) downTo v(1L) step v(4L)).reversed()) l += i
    for (i in (v(1)..v(9)).reversed() step v(3)) l += i
    for (c in (v('a')..v('d')).reversed()) l += c
    for (i in (v(5L)..v(7L)).reversed()) l += i
    return l
}

// Not inlined, as with kotlinc: until and ..< stepped, then reversed.
fun untilSteppedReversed(a: Int, b: Int): List<Int> { val l = mutableListOf<Int>(); for (i in (a until b step 3).reversed()) l += i; return l }

fun main() {
    println(countedSteppedReversed(1, 10, 3))
    println(countedSteppedReversed(1, 1, 3))
    println(countedSteppedReversed(5, 1, 2))
    println(countedSteppedReversed(Int.MAX_VALUE - 5, Int.MAX_VALUE, 2))
    println(countedSteppedReversed(Int.MIN_VALUE, Int.MIN_VALUE + 5, 2))
    println(countedSteppedReversed(0, Int.MAX_VALUE, Int.MAX_VALUE))
    try {
        countedSteppedReversed(1, 10, 0)
    } catch (e: IllegalArgumentException) {
        println(e.message)
    }
    println(countedDownToSteppedReversed(10, 0, 3))
    println(countedDownToSteppedReversed(0, 10, 3))
    println(countedDownToSteppedReversed(Int.MIN_VALUE + 5, Int.MIN_VALUE, 2))
    println(countedDownToSteppedReversed(Int.MAX_VALUE, Int.MAX_VALUE - 5, 2))
    println(countedConstSteppedReversed(4))
    println(countedLongSteppedReversed(1L, 10L, 4L))
    println(countedLongSteppedReversed(Long.MAX_VALUE - 4, Long.MAX_VALUE, 3L))
    try {
        countedLongSteppedReversed(1L, 10L, -2L)
    } catch (e: IllegalArgumentException) {
        println(e.message)
    }
    println(countedLongDownToSteppedReversed(9L, 0L))
    println(countedLongDownToSteppedReversed(Long.MIN_VALUE + 3, Long.MIN_VALUE))
    println(countedCharSteppedReversed('a', 'h'))
    println(countedCharSteppedReversed('h', 'a'))
    println(countedCharDownToSteppedReversed('z', 'q', 3))
    println(countedCharDownToSteppedReversed('￿', '￺', 2).map { it.code })

    println(countedIndicesReversedStep(IntArray(7), 2))
    println(countedIndicesReversedStep(IntArray(0), 2))
    println(countedIndicesReversedStep(IntArray(4), 10))
    try {
        countedIndicesReversedStep(IntArray(3), -1)
    } catch (e: IllegalArgumentException) {
        println(e.message)
    }
    println(countedListIndicesReversedStep(listOf("a", "b", "c", "d", "e")))
    println(countedReversedStep(1, 10, 4))
    println(countedReversedStep(Int.MIN_VALUE, Int.MIN_VALUE + 4, 3))
    println(countedReversedStep(10, 1, 4))
    println(countedDownToReversedStep(10, 1, 4))
    println(countedDownToReversedStep(Int.MAX_VALUE, Int.MAX_VALUE - 4, 3))
    println(countedLongReversedStep(1L, 10L))
    println(countedCharReversedStep('a', 'f'))

    println(countedLongReversed(1L, 4L))
    println(countedLongReversed(4L, 1L))
    println(countedLongReversed(Long.MIN_VALUE, Long.MIN_VALUE + 2))
    println(countedLongDownToReversed(4L, 1L))
    println(countedLongDownToReversed(Long.MAX_VALUE, Long.MAX_VALUE - 2))
    println(countedCharReversed('p', 't'))
    println(countedCharReversed('t', 'p'))
    println(countedCharReversed('\u0000', '\u0002').map { it.code })
    println(countedCharDownToReversed('t', 'p'))
    println(countedCharConstReversed())

    println(countedBreakContinue(1, 20))
    println(countedCaptured(1L, 7L).map { it() })
    println(countedOrder())
    println(log)
    println(untilSteppedReversed(0, 10))
}
