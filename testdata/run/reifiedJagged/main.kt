fun <T> show(a: Array<T>): String {
    var s = ""
    for (x in a) s += if (s == "") "$x" else ", $x"
    return "[$s]"
}

fun show(a: IntArray): String {
    var s = ""
    for (x in a) s += if (s == "") "$x" else ", $x"
    return "[$s]"
}

fun main() {
    val s: Array<Array<String>> = jagged { x, y -> "$x-$y" }
    println(show(s[0]) + show(s[1]))
    val a: Array<Array<Array<String>>> = jagged { x, y -> arrayOf("$x$y") }
    println(a[1][0][0])
    val ia: Array<Array<IntArray>> = jagged { x, y -> intArrayOf(x + y + 1) }
    println(ia[1][1][0])
    val d: Array<Array<Array<String>>> = deep { x, y, z -> "$x$y$z" }
    println(show(d[0][0]))
    println(show(squares { it + 1 }))
    println(show(counted { "c$it" }))
}
