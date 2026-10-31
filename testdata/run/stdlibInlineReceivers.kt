// The standard library's inline functions that take lambdas with
// receivers: apply, with, run, buildString and buildList.

class Point(var x: Int, var y: Int) {
    fun describe(): String = "($x, $y)"

    // Extensions called on this unqualified, in a method.
    fun moved(dx: Int): Point = apply { x += dx }
    fun label(): String = run { describe() + "#" + x }
}

class Doc(val title: String) {
    val lines = ArrayList<String>()
    fun add(line: String): Doc = apply { lines.add(line) }
    fun render(): String = buildString {
        append(title)
        for (l in lines) {
            append("|").append(l)
        }
        append(" ").append(this@Doc.title.length)
    }
}

fun main() {
    val sb = StringBuilder().apply {
        append("a")
        append(1)
        this.append('c')
    }
    println(sb)
    println(buildString {
        append("x=")
        append(length)
        repeat(2) { append(it) }
    })

    val p = Point(1, 2).apply {
        x = 10
        y += 5
    }
    println(p.describe() + " " + p.moved(1).label())
    println(with(p) { x * y })
    println(p.run { describe() + "!" })
    println(with("abc") { length + get(0).code })

    // return@apply, and an early return from with.
    val q = Point(0, 0).apply {
        if (x == 0) return@apply
        x = 99
    }
    println(q.describe())
    println(with(q) {
        if (y > 0) return@with "positive"
        "zero"
    })

    val xs: List<Int> = buildList { add(1); add(2) }
    val ys = buildList<String> { add("a"); addAll(listOf("b", "c")) }
    println(xs.toString() + ys)
    println(Doc("t").add("one").add("two").render())

    // Nested receivers: this@apply is the innermost apply's.
    val nested = StringBuilder("o").apply {
        StringBuilder("i").apply {
            append(this@apply.length)
        }.let { append(it) }
    }
    println(nested)

    // A lambda object inside an inlined lambda captures the receiver.
    val getters = StringBuilder("r").run { listOf(1, 2).map { n -> { append(n).toString() } } }
    println(getters[0]() + getters[1]())
}
