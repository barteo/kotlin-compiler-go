// Labeled loops, break@ and continue@, labeled lambdas, and do-while.

inline fun twice(block: (Int) -> Unit) {
    for (i in 0 until 2) {
        block(i)
    }
}

fun search(grid: Array<IntArray>, target: Int): String {
    var found = "none"
    outer@ for (r in grid.indices) {
        for (c in grid[r].indices) {
            if (grid[r][c] == target) {
                found = "($r, $c)"
                break@outer
            }
        }
    }
    return found
}

fun skipRows(grid: Array<IntArray>): Int {
    var sum = 0
    rows@ for (row in grid) {
        for (x in row) {
            if (x < 0) continue@rows
        }
        sum += row.sum()
    }
    return sum
}

fun whileLabels(): String {
    val sb = StringBuilder()
    var i = 0
    outer@ while (i < 5) {
        i++
        var j = 0
        inner@ while (true) {
            j++
            if (j > i) break@inner
            if (i == 2) continue@outer
            if (i == 4) break@outer
            sb.append(i).append(j).append(' ')
        }
        sb.append("| ")
    }
    return sb.toString().trim()
}

fun digits(n: Int): Int {
    var count = 0
    var m = n
    do {
        count++
        m /= 10
    } while (m != 0)
    return count
}

fun doScope(): String {
    val parts = mutableListOf<String>()
    var i = 0
    do {
        val next = i * 3
        parts.add(next.toString())
        i++
    } while (next < 10)
    return parts.joinToString(",")
}

fun doLabels(): String {
    val sb = StringBuilder()
    var i = 0
    outer@ do {
        i++
        var j = 0
        do {
            j++
            if (j == 2) continue
            if (i == 3) continue@outer
            if (j * i > 6) break@outer
            sb.append(i * 10 + j).append(' ')
        } while (j < 3)
    } while (i < 5)
    return sb.toString().trim() + " i=$i"
}

fun doAssign(flag: Boolean): Int {
    val x: Int
    do {
        x = if (flag) 1 else 2
    } while (false)
    return x
}

fun tryInLoop(): String {
    val sb = StringBuilder()
    outer@ for (i in 1..3) {
        for (j in 1..3) {
            try {
                if (j == 2) continue@outer
                sb.append("$i$j ")
            } finally {
                sb.append("f ")
            }
        }
    }
    return sb.toString().trim()
}

fun inlineLoops(): String {
    val sb = StringBuilder()
    loop@ for (i in 0..3) {
        twice {
            if (i == 1) continue@loop
            if (i == 3) break@loop
            sb.append("$i.$it ")
        }
    }
    return sb.toString().trim()
}

// The break leaves the caller's loop, not the one in twice's body.
fun inlineBreak(): Int {
    var n = 0
    for (i in 0..5) {
        twice {
            if (i == 2) break
            n++
        }
    }
    return n
}

fun lambdaLabels(): String {
    val sb = StringBuilder()
    listOf(1, 2, 3, 4).forEach lit@{
        if (it % 2 == 0) return@lit
        sb.append(it)
    }
    listOf(5, 6).forEach each@{ x ->
        run inner@{
            if (x == 5) return@inner
            sb.append(x)
        }
        sb.append('.')
    }
    val f = fn@{ x: Int ->
        if (x > 2) return@fn "big"
        "small"
    }
    sb.append(' ').append(f(1)).append(' ').append(f(3))
    return sb.toString()
}

fun main() {
    val grid = arrayOf(intArrayOf(1, 2, 3), intArrayOf(4, -5, 6), intArrayOf(7, 8, 9))
    println(search(grid, 6))
    println(search(grid, 10))
    println(skipRows(grid))
    println(whileLabels())
    println(digits(0))
    println(digits(12345))
    println(doScope())
    println(doLabels())
    println(doAssign(true))
    println(doAssign(false))
    println(tryInLoop())
    println(inlineLoops())
    println(inlineBreak())
    println(lambdaLabels())
}
