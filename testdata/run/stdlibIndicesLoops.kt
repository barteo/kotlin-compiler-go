// Loops over the standard library's indices, which kotlinc compiles as
// counted loops, with no IntRange or iterator: of each array type, of a
// CharSequence and of a Collection, and their reversed() and step.
// TestIndicesLoops checks that the functions named counted* call none.

@file:OptIn(ExperimentalUnsignedTypes::class)

fun countedInts(a: IntArray): Long { var s = 0L; for (i in a.indices) s += a[i] * (i + 1); return s }
fun countedLongs(a: LongArray): Long { var s = 0L; for (i in a.indices) s += a[i] * (i + 1); return s }
fun countedChars(a: CharArray): String { val b = StringBuilder(); for (i in a.indices) b.append(i).append(a[i]); return b.toString() }
fun countedFloats(a: FloatArray): Float { var s = 0f; for (i in a.indices) s += a[i] * i; return s }
fun countedDoubles(a: DoubleArray): Double { var s = 0.0; for (i in a.indices) s += a[i] * i; return s }
fun countedBytes(a: ByteArray): Int { var s = 0; for (i in a.indices) s += a[i] * i; return s }
fun countedShorts(a: ShortArray): Int { var s = 0; for (i in a.indices) s += a[i] * i; return s }
fun countedBooleans(a: BooleanArray): Int { var s = 0; for (i in a.indices) if (a[i]) s += i; return s }
fun countedObjects(a: Array<String>): Int { var s = 0; for (i in a.indices) s += a[i].length * i; return s }
fun <T> countedGeneric(a: Array<out T>): List<String> { val r = mutableListOf<String>(); for (i in a.indices) r += "$i=${a[i]}"; return r }
fun countedUInts(a: UIntArray): ULong { var s = 0uL; for (i in a.indices) s += a[i] * (i + 1).toUInt(); return s }
fun countedULongs(a: ULongArray): ULong { var s = 0uL; for (i in a.indices) s += a[i] * (i + 1).toULong(); return s }
fun countedUShorts(a: UShortArray): UInt { var s = 0u; for (i in a.indices) s += a[i] * (i + 1).toUInt(); return s }
fun countedUBytes(a: UByteArray): UInt { var s = 0u; for (i in a.indices) s += a[i] * (i + 1).toUInt(); return s }
fun countedString(t: String): Int { var s = 0; for (i in t.indices) s += t[i].code * (i + 1); return s }
fun countedCharSequence(t: CharSequence): String { val b = StringBuilder(); for (i in t.indices) b.append(t[t.length - 1 - i]); return b.toString() }
fun countedList(l: List<Int>): Int { var s = 0; for (i in l.indices) s += l[i] * (i + 1); return s }
fun countedCollection(c: Collection<String>): Int { var s = 0; for (i in c.indices) s += i; return s }
fun countedNullable(a: IntArray?): Int { var s = 0; if (a != null) for (i in a.indices) s += a[i]; return s }
fun countedPlatform(n: Int): Int { var s = 0; val t = String.format("%d", n); for (i in t.indices) s += t[i] - '0'; return s }

fun countedReversed(a: IntArray): List<Int> { val r = mutableListOf<Int>(); for (i in a.indices.reversed()) r += a[i]; return r }
fun countedReversedString(t: String): String { val b = StringBuilder(); for (i in t.indices.reversed()) b.append(t[i]); return b.toString() }
fun countedStep(a: IntArray, n: Int): List<Int> { val r = mutableListOf<Int>(); for (i in a.indices step n) r += i; return r }
fun countedStepList(l: List<String>): String { val b = StringBuilder(); for (i in l.indices step 2) b.append(l[i]); return b.toString() }
fun countedRangeReversed(a: Int, b: Int): List<Int> { val r = mutableListOf<Int>(); for (i in (a..b).reversed()) r += i; return r }
fun countedDownToReversed(a: Int, b: Int): List<Int> { val r = mutableListOf<Int>(); for (i in (a downTo b).reversed()) r += i; return r }

fun countedBreakContinue(a: IntArray): List<Int> {
    val r = mutableListOf<Int>()
    for (i in a.indices) {
        if (a[i] % 2 == 0) continue
        if (a[i] > 7) break
        r += i
    }
    return r
}

fun countedLabeled(a: Array<IntArray>): List<String> {
    val r = mutableListOf<String>()
    outer@ for (i in a.indices) {
        for (j in a[i].indices) {
            if (a[i][j] < 0) continue@outer
            if (a[i][j] == 0) break@outer
            r += "$i,$j"
        }
    }
    return r
}

fun countedCaptured(a: IntArray): List<() -> Int> {
    val fs = mutableListOf<() -> Int>()
    for (i in a.indices) fs += { i * 10 + a[i] }
    return fs
}

var evaluated = 0
fun array(n: Int): IntArray { evaluated++; return IntArray(n) { it } }
fun countedOnce(): Int { var s = 0; for (i in array(5).indices) s += i; return s }

var bounds = ""
fun bound(name: String, v: Int): Int { bounds += name; return v }
fun countedOrder(): List<Int> { val r = mutableListOf<Int>(); for (i in (bound("a", 1)..bound("b", 4)).reversed()) r += i; return r }
fun countedDownToOrder(): List<Int> { val r = mutableListOf<Int>(); for (i in (bound("c", 4) downTo bound("d", 1)).reversed()) r += i; return r }

// The size is read once: elements added in the loop aren't visited.
fun countedGrowing(l: MutableList<Int>): List<Int> {
    val seen = mutableListOf<Int>()
    for (i in l.indices) {
        seen += l[i]
        l += l[i] + 100
    }
    return seen
}

// Not the standard library's indices: these are called as they are.
class Grid(val n: Int) {
    val indices: IntRange get() { println("Grid.indices"); return 0 until n }
}

object Shadow {
    val IntArray.indices: IntRange get() { println("Shadow.indices"); return 0..1 }
    fun sum(a: IntArray): Int { var s = 0; for (i in a.indices) s += a[i]; return s }
}

class Tree(val size: Int)
val Tree.indices: IntProgression get() { println("Tree.indices"); return size - 1 downTo 0 step 2 }

fun main() {
    println(countedInts(intArrayOf(3, 1, 4, 1, 5)))
    println(countedLongs(longArrayOf(10L, 20L, 30L)))
    println(countedChars(charArrayOf('x', 'y', 'z')))
    println(countedFloats(floatArrayOf(1.5f, 2.5f, 3.5f)))
    println(countedDoubles(doubleArrayOf(0.25, 0.5, 0.75)))
    println(countedBytes(byteArrayOf(1, 2, 3, 4)))
    println(countedShorts(shortArrayOf(100, 200, 300)))
    println(countedBooleans(booleanArrayOf(true, false, true, true)))
    println(countedObjects(arrayOf("a", "bb", "ccc")))
    println(countedGeneric(arrayOf<Any?>(1, null, "x")))
    println(countedUInts(uintArrayOf(4000000000u, 1u, 2u)))
    println(countedULongs(ulongArrayOf(18000000000000000000uL, 1uL)))
    println(countedUShorts(ushortArrayOf(65535u, 2u)))
    println(countedUBytes(ubyteArrayOf(255u, 1u, 2u)))
    println(countedString("hello"))
    println(countedCharSequence(StringBuilder("stressed")))
    println(countedList(listOf(5, 6, 7)))
    println(countedCollection(linkedSetOf("p", "q", "r", "s")))
    println(countedNullable(intArrayOf(7, 8)))
    println(countedNullable(null))
    println(countedPlatform(98765))

    // Empty receivers.
    println(listOf(countedInts(IntArray(0)), countedLongs(LongArray(0))))
    println(countedChars(CharArray(0)) + "|" + countedString("") + "|" + countedCharSequence(StringBuilder()))
    println(listOf(countedObjects(arrayOf()), countedList(emptyList()), countedCollection(emptySet())))
    println(listOf(countedUInts(UIntArray(0)), countedUBytes(UByteArray(0))))
    println(countedReversed(IntArray(0)) + countedStep(IntArray(0), 3))

    println(countedReversed(intArrayOf(1, 2, 3, 4)))
    println(countedReversedString("drawer"))
    println(countedStep(IntArray(10), 3))
    println(countedStep(IntArray(9), 3))
    println(countedStep(IntArray(1), 5))
    try {
        countedStep(IntArray(3), 0)
    } catch (e: IllegalArgumentException) {
        println(e.message)
    }
    println(countedStepList(listOf("a", "b", "c", "d", "e")))
    println(countedRangeReversed(2, 6))
    println(countedRangeReversed(6, 2))
    println(countedRangeReversed(Int.MAX_VALUE - 2, Int.MAX_VALUE))
    println(countedRangeReversed(Int.MIN_VALUE, Int.MIN_VALUE + 1))
    println(countedDownToReversed(6, 2))
    println(countedDownToReversed(2, 6))
    println(countedDownToReversed(Int.MAX_VALUE, Int.MAX_VALUE - 1))

    println(countedBreakContinue(intArrayOf(2, 3, 4, 5, 6, 7, 9, 11)))
    println(countedLabeled(arrayOf(intArrayOf(1, 2), intArrayOf(-1, 5), intArrayOf(3, 0, 4), intArrayOf(8))))
    println(countedCaptured(intArrayOf(1, 2, 3)).map { it() })

    println(countedOnce())
    println(evaluated)
    println(countedOrder())
    println(countedDownToOrder())
    println(bounds)
    println(countedGrowing(mutableListOf(1, 2, 3)))

    val g = Grid(3)
    for (i in g.indices) print("$i ")
    println()
    println(Shadow.sum(intArrayOf(10, 20, 30)))
    for (i in Tree(5).indices) print("$i ")
    println()

    // withIndex and destructuring over the indices.
    for ((n, i) in intArrayOf(7, 8, 9).indices.withIndex()) print("$n:$i ")
    println()
    for ((i, v) in listOf("u", "v").withIndex()) print("$i$v ")
    println()
}
