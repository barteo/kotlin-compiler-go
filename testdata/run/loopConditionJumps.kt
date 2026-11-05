// break and continue in loop conditions: in a do-while's condition they
// are the do-while's own; in a while's, the enclosing loop's.
fun doWhileBreak(): Int {
    var i = 0
    do {
        i++
    } while (if (i < 5) true else break)
    return i
}

fun doWhileContinue(): String {
    val sb = StringBuilder()
    var i = 0
    do {
        i++
        sb.append(i)
    } while (if (i++ < 3) continue else false)
    return sb.toString()
}

fun whileInLoop(): String {
    val sb = StringBuilder()
    for (k in 1..3) {
        var j = 0
        while (if (k == 2) break else j < k) {
            sb.append(k)
            j++
        }
        sb.append(';')
    }
    return sb.toString()
}

fun nested(): Int {
    var n = 0
    outer@ for (a in 1..3) {
        do {
            n++
        } while (if (n % 2 == 0) break@outer else false)
    }
    return n
}

fun main() {
    println(doWhileBreak())
    println(doWhileContinue())
    println(whileInLoop())
    println(nested())
}
