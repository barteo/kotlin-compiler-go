// Smart casts of local vars, which follow assignments, branches and
// loops, of x!!, and of final val properties read through stable values.

class Node(val name: String, val parent: Node?)

class Import(val name: String, val alias: String?)

open class Shape

class Circle(val r: Int) : Shape()

class Square(val side: Int) : Shape()

fun path(n: Node): String {
    val sb = StringBuilder()
    var c: Node? = n
    while (c != null) {
        sb.append(c.name)
        c = c.parent
        if (c != null) sb.append('<')
    }
    return sb.toString()
}

fun cached(map: java.util.HashMap<String, String>, key: String): String {
    var value: String? = map.get(key)
    if (value == null) {
        value = key + "!"
        map.put(key, value)
    }
    return value
}

fun firstOrDefault(xs: Array<String?>): String {
    var found: String? = null
    for (x in xs) {
        if (x != null) {
            found = x
            break
        }
    }
    if (found == null) return "none"
    return found
}

fun area(s: Shape): Int {
    var shape: Shape = s
    if (shape is Circle) {
        return 3 * shape.r * shape.r
    }
    shape = Square(2)
    return shape.side * shape.side
}

fun kind(x: Any): String {
    var v: Any = x
    when (v) {
        is String -> v = v.length
        is Int -> v = v + 1
        else -> return "other"
    }
    return "num " + (v as Int)
}

fun asserted(s: String?): Int {
    var t = s
    t!!.length
    return t.length
}

fun alias(i: Import): String {
    if (i.alias != null) {
        return i.alias.uppercase()
    }
    return i.name
}

fun widened(): Any {
    var a: Any = "text"
    a = 42
    return a + 1
}

fun countdown(n: Int): String {
    var x: Int? = n
    val sb = StringBuilder()
    do {
        sb.append(x).append(' ')
        x = if (x!! > 0) x - 1 else null
    } while (x != null)
    return sb.toString()
}

fun main() {
    println(path(Node("c", Node("b", Node("a", null)))))
    val m = java.util.HashMap<String, String>()
    println(cached(m, "k") + cached(m, "k"))
    println(firstOrDefault(arrayOf(null, "b", "c")) + firstOrDefault(arrayOf(null)))
    println(area(Circle(1)) + area(Square(5)))
    println(kind("abc") + " " + kind(4) + " " + kind(1.5))
    println(asserted("four"))
    println(alias(Import("a.B", "c")) + alias(Import("a.B", null)))
    println(widened())
    println(countdown(3))
}
