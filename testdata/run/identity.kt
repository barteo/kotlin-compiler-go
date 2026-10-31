data class Point(val x: Int, val y: Int)

class Node(val name: String, val next: Node?)

fun same(a: Any?, b: Any?): Boolean = a === b

fun <T> sameT(a: T, b: T): Boolean = a === b

fun lastOf(n: Node): Node {
    var cur = n
    while (cur.next !== null) {
        cur = cur.next!!
    }
    return cur
}

fun describe(n: Node?): String {
    if (n === null) return "none"
    return n.name
}

fun main() {
    val p = Point(1, 2)
    val q = p.copy()
    val r = p
    println(p == q)
    println(p === q)
    println(p !== q)
    println(p === r)
    println(same(p, r))
    println(same(p, q))
    println(same(null, null))
    println(sameT("a", "a"))

    val a = Node("a", Node("b", Node("c", null)))
    println(lastOf(a).name)
    println(describe(null))
    println(describe(a.next))

    val s1 = StringBuilder("x").toString()
    val s2 = StringBuilder("x").toString()
    println(s1 == s2)
    println(s1 === s2)
    val any: Any = s1
    println(any === s1)
    println(s2 !== any)

    val maybe: Node? = if (s1.length > 5) a else null
    val flag = maybe === null
    println(flag)
    if (maybe !== null) println(maybe.name) else println("null")
    val items = arrayOf(p, q)
    var count = 0
    for (it in items) {
        if (it === p) count++
    }
    println(count)
    println(if (p === r && q !== r) "yes" else "no")
}
