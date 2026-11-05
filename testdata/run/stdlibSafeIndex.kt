// The index of a safe call is of its chain: a?.b[i] is a?.b?.get(i), and
// a?.b[i] = v, a?.b[i] op= v and a?.b[i]++ change the element only if a
// isn't null. The indices and the value are evaluated only then too.
class Grid {
    val cells = IntArray(3)
    val rows = arrayOf(IntArray(2), IntArray(2))
    val names = mutableListOf("a", "b")
    val counts = mutableMapOf("x" to 1)

    fun row(i: Int) = rows[i]
}

class Holder(var grid: Grid?)

var evals = 0

fun at(i: Int): Int {
    evals++
    return i
}

fun show(g: Grid?): String =
    "${g?.cells[0]} ${g?.rows[1][0]} ${g?.names[1]} ${g?.counts["x"]} ${g?.row(0)[1]}"

fun main() {
    var g: Grid? = null
    g?.cells[at(0)] = at(5)
    g?.cells[at(0)] += at(1)
    g?.cells[at(0)]++
    g?.rows[at(1)][at(0)] = 3
    g?.names[at(0)] = "z"
    g?.counts["x"] = at(2)
    val none: Int? = g?.cells[at(2)]++
    println("${show(g)} $none $evals")

    g = Grid()
    g?.cells[at(0)] = at(5)
    g?.cells[at(0)] += at(1)
    g?.cells[at(0)]++
    ++g?.cells[at(1)]
    g?.rows[at(1)][at(0)] = 3
    g?.rows[1][0] *= 2
    g?.row(0)[1] = 4
    g?.names[at(0)] = "z"
    g?.names[1] += "!"
    g?.counts["x"] = at(2)
    val old: Int? = g?.cells[at(2)]++
    val new = --g?.cells[2]
    println("${show(g)} ${g.cells[1]} $old $new $evals")

    // The receiver is evaluated once.
    val h = Holder(g)
    var reads = 0
    fun holder(): Holder {
        reads++
        return h
    }
    holder().grid?.cells[0]++
    holder().grid?.counts["x"] = holder().grid?.counts["x"]!! + 1
    println("${g.cells[0]} ${g.counts["x"]} $reads")
}
