import java.util.Arrays

enum class Color { RED, GREEN, BLUE }

class Grid(val w: Int, val h: Int) {
    val cells = Array(h) { IntArray(w) }

    fun set(x: Int, y: Int, v: Int) {
        cells[y][x] = v
    }

    fun sum(): Int {
        var s = 0
        for (row in cells) {
            for (c in row) {
                s += c
            }
        }
        return s
    }
}

fun total(xs: IntArray): Int {
    var s = 0
    for (x in xs) s += x
    return s
}

fun describe(xs: Array<out Any>): String {
    var s = ""
    for (x in xs) s += "$x;"
    return s
}

fun <T> firstOf(xs: Array<T>): T = xs[0]

fun <T> swap(xs: Array<T>, i: Int, j: Int) {
    val t = xs[i]
    xs[i] = xs[j]
    xs[j] = t
}

fun kind(x: Any): String = when (x) {
    is IntArray -> "IntArray of ${x.size}"
    is Array<*> -> "Array of ${x.size}"
    else -> "other"
}

fun main(args: Array<String>) {
    println(args.size)

    // Constructors and factories.
    val zeros = IntArray(3)
    println(Arrays.toString(zeros))
    val squares = IntArray(5) { it * it }
    println(Arrays.toString(squares))
    val names = Array(3) { "s$it" }
    println(Arrays.toString(names))
    val longs = longArrayOf(1L, 2L, 3000000000L)
    println(longs[2] + longs.size)
    val ds = doubleArrayOf(0.5, 1.5)
    println(ds[0] + ds[1])
    val flags = booleanArrayOf(true, false)
    println(flags[1] || flags[0])
    val boxed = arrayOf(1, 2, 3)
    println(boxed[0] + boxed[2])
    val nulls = arrayOfNulls<String>(2)
    println(nulls[0] == null)
    nulls[1] = "x"
    println(nulls[1])
    val empty = emptyArray<String>()
    println(empty.size)
    val strs: Array<String> = arrayOf("b", "a", "c")
    println(strs.size)

    // Elements and compound assignment.
    val counts = IntArray(4)
    counts[1] += 2
    counts[1] *= 5
    counts[2]++
    counts[3]--
    println(Arrays.toString(counts))
    boxed[1] += 40
    println(boxed[1])
    strs[0] += "!"
    println(strs[0])
    println(total(squares))

    // Loops and nested arrays.
    for (n in names) print(n + " ")
    println()
    for (i in 0 until squares.size) {
        if (squares[i] == 4) continue
        if (squares[i] > 9) break
        print("${squares[i]} ")
    }
    println()
    val g = Grid(3, 2)
    g.set(0, 0, 5)
    g.set(2, 1, 7)
    println(g.sum())
    println(g.cells[1][2])

    // Variance, generics and type checks.
    println(describe(strs))
    println(describe(boxed))
    println(firstOf(strs))
    swap(strs, 0, 2)
    println(strs[0] + strs[2])
    val copy = squares.clone()
    copy[0] = 100
    println("" + squares[0] + " " + copy[0])
    println(kind(squares))
    println(kind(names))
    println(kind("x"))
    val any: Any = boxed
    if (any is Array<*>) println(any.size)

    // Java arrays and enum values().
    val sorted = arrayOf(3, 1, 2)
    Arrays.sort(sorted)
    println(Arrays.toString(sorted))
    val colors = Color.values()
    println(colors.size)
    println(colors[2])
    for (c in Color.values()) print(c.name + " ")
    println()
    var captured = IntArray(2)
    val fill = { captured[0] = 9 }
    fill()
    println(captured[0])
    captured = IntArray(1)
    println(captured.size)
}
