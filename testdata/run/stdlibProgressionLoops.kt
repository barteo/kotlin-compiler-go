// Loops over values of the standard library's progression classes, which
// kotlinc compiles as counted loops over their first, last and step, with
// no iterator: IntRange, LongRange and CharRange and their progressions,
// those that reversed() and step return among them. TestIndicesLoops
// checks that the functions named looped* call no iterator.

fun loopedIntRange(r: IntRange): List<Int> { val l = mutableListOf<Int>(); for (i in r) l += i; return l }
fun loopedIntProgression(p: IntProgression): List<Int> { val l = mutableListOf<Int>(); for (i in p) l += i; return l }
fun loopedLongRange(r: LongRange): List<Long> { val l = mutableListOf<Long>(); for (i in r) l += i; return l }
fun loopedLongProgression(p: LongProgression): List<Long> { val l = mutableListOf<Long>(); for (i in p) l += i; return l }
fun loopedCharRange(r: CharRange): String { val b = StringBuilder(); for (c in r) b.append(c); return b.toString() }
fun loopedCharProgression(p: CharProgression): String { val b = StringBuilder(); for (c in p) b.append(c); return b.toString() }
fun loopedNullable(r: IntRange?): Int { var s = 0; if (r != null) for (i in r) s += i; return s }

fun loopedUntilReversed(a: Int, b: Int): List<Int> { val l = mutableListOf<Int>(); for (i in (a until b).reversed()) l += i; return l }
fun loopedSteppedReversed(a: Int, b: Int, n: Int): List<Int> { val l = mutableListOf<Int>(); for (i in (a..b step n).reversed()) l += i; return l }
fun loopedUntilSteppedReversed(a: Int, b: Int): List<Int> { val l = mutableListOf<Int>(); for (i in (a until b step 3).reversed()) l += i; return l }
fun loopedDownToSteppedReversed(a: Int, b: Int): List<Int> { val l = mutableListOf<Int>(); for (i in (a downTo b step 3).reversed()) l += i; return l }
fun loopedIndicesReversedStep(a: IntArray): List<Int> { val l = mutableListOf<Int>(); for (i in a.indices.reversed() step 2) l += i; return l }
fun loopedLongReversed(a: Long, b: Long): List<Long> { val l = mutableListOf<Long>(); for (i in (a..b).reversed()) l += i; return l }
fun loopedCharReversed(a: Char, b: Char): String { val s = StringBuilder(); for (c in (a..b).reversed()) s.append(c); return s.toString() }

fun loopedBreakContinue(r: IntRange): List<Int> {
    val l = mutableListOf<Int>()
    for (i in r) {
        if (i % 3 == 0) continue
        if (i > 10) break
        l += i
    }
    return l
}

fun loopedLabeled(p: IntProgression, q: LongProgression): List<String> {
    val l = mutableListOf<String>()
    outer@ for (i in p) {
        for (j in q) {
            if (j == 2L) continue@outer
            if (i == 7) break@outer
            l += "$i,$j"
        }
    }
    return l
}

fun loopedCaptured(r: CharRange): List<() -> Char> {
    val fs = mutableListOf<() -> Char>()
    for (c in r) fs += { c }
    return fs
}

var made = 0
val counted: IntRange get() { made++; return 1..4 }
fun loopedOnce(): Int { var s = 0; for (i in counted) s += i; return s }

class Grid(val n: Int) {
    val indices: IntRange get() { println("Grid.indices"); return 0 until n }
}
fun loopedMember(g: Grid): List<Int> { val l = mutableListOf<Int>(); for (i in g.indices) l += i; return l }

// Not counted: a variable of another type than the elements.
fun anyVar(r: IntRange): String { val b = StringBuilder(); for (x: Any in r) b.append(x); return b.toString() }

fun main() {
    println(loopedIntRange(1..5))
    println(loopedIntRange(5..1))
    println(loopedIntRange(Int.MAX_VALUE - 2..Int.MAX_VALUE))
    println(loopedIntRange(IntRange.EMPTY))
    println(loopedIntProgression(10 downTo 1 step 3))
    println(loopedIntProgression(1..10 step 4))
    println(loopedIntProgression(Int.MIN_VALUE + 4 downTo Int.MIN_VALUE step 2))
    println(loopedIntProgression(Int.MAX_VALUE - 5..Int.MAX_VALUE step 2))
    println(loopedIntProgression(1 downTo 5))
    println(loopedIntProgression(IntProgression.fromClosedRange(0, 10, -1)))
    println(loopedLongRange(Long.MAX_VALUE - 1..Long.MAX_VALUE))
    println(loopedLongRange(3L..1L))
    println(loopedLongProgression(Long.MIN_VALUE + 2 downTo Long.MIN_VALUE))
    println(loopedLongProgression(0L..20L step 7L))
    println(loopedCharRange('a'..'f'))
    println(loopedCharRange('�'..'￿').map { it.code })
    println(loopedCharProgression('z' downTo 'a' step 5))
    println(loopedCharProgression('a'..'k' step 3))
    println(loopedNullable(1..3))
    println(loopedNullable(null))

    println(loopedUntilReversed(1, 5))
    println(loopedUntilReversed(5, 1))
    println(loopedUntilReversed(0, Int.MIN_VALUE))
    println(loopedUntilReversed(Int.MIN_VALUE, Int.MIN_VALUE + 2))
    println(loopedSteppedReversed(1, 10, 3))
    println(loopedSteppedReversed(1, 1, 3))
    try {
        loopedSteppedReversed(1, 10, -1)
    } catch (e: IllegalArgumentException) {
        println(e.message)
    }
    println(loopedUntilSteppedReversed(0, 10))
    println(loopedDownToSteppedReversed(10, 0))
    println(loopedIndicesReversedStep(IntArray(6)))
    println(loopedIndicesReversedStep(IntArray(0)))
    println(loopedLongReversed(1L, 4L))
    println(loopedCharReversed('p', 't'))

    println(loopedBreakContinue(1..20))
    println(loopedLabeled(5..9, 1L..3L))
    println(loopedCaptured('x'..'z').map { it() })
    println(loopedOnce())
    println(made)
    println(loopedMember(Grid(3)))
    println(anyVar(3..6))
}
