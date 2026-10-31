// Locals and properties declared without a value, which code assigns
// before reading them.

class Point(x: Int, y: Int, scale: Boolean) {
    val x: Int
    val y: Int
    var label: String
    val norm: Int

    init {
        if (scale) {
            this.x = x * 10
            this.y = y * 10
        } else {
            this.x = x
            this.y = y
        }
        label = "(" + this.x + ", " + this.y + ")"
    }

    init {
        norm = Math.abs(this.x) + Math.abs(y)
    }
}

class Named {
    val name: String
    val size: Int

    constructor(name: String) {
        this.name = name
        size = name.length
    }

    constructor(n: Int) {
        name = "#" + n
        size = 0
    }
}

open class Shape(sides: Int) {
    val kind: String

    init {
        kind = when (sides) {
            3 -> "triangle"
            4 -> "square"
            else -> "polygon"
        }
    }
}

fun classify(n: Int): String {
    val s: String
    if (n < 0) {
        s = "negative"
    } else if (n == 0) {
        s = "zero"
    } else {
        s = "positive"
    }
    return s
}

fun parse(text: String): Int {
    val value: Int
    try {
        value = Integer.parseInt(text)
    } catch (e: NumberFormatException) {
        return -1
    }
    return value
}

fun firstEven(xs: IntArray): Int {
    var found: Int
    var i = 0
    while (true) {
        if (xs[i] % 2 == 0) {
            found = xs[i]
            break
        }
        i++
    }
    return found
}

fun kind(n: Int): String {
    val k: String
    when {
        n % 2 == 0 -> k = "even"
        else -> k = "odd"
    }
    val f = { k + "!" }
    return f()
}

fun main() {
    val p = Point(1, -2, true)
    println(p.x)
    println(p.label)
    println(p.norm)
    println(Point(3, 4, false).label)
    println(Named("abc").size)
    println(Named(7).name)
    println(Shape(4).kind)
    println(classify(-5) + " " + classify(0) + " " + classify(3))
    println(parse("12") + parse("x"))
    println(firstEven(intArrayOf(1, 3, 8, 5)))
    println(kind(3))
    var total: Long
    total = 0L
    for (i in 1..4) {
        total += i
    }
    println(total)
}
