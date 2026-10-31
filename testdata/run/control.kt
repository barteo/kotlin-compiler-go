fun sign(n: Int): String {
    if (n < 0) return "negative"
    if (n == 0) {
        return "zero"
    }
    return "positive"
}

fun grade(score: Int): String = when (score) {
    100 -> "perfect"
    in 90..99 -> "A"
    in 80 until 90 -> "B"
    70, 71, 72 -> "C-low"
    !in 0..100 -> "invalid"
    else -> "other"
}

fun collatz(start: Long): Int {
    var n = start
    var steps = 0
    while (n != 1L) {
        n = if (n % 2 == 0L) n / 2 else 3 * n + 1
        steps++
    }
    return steps
}

fun firstMultiple(of: Int, limit: Int): Int {
    var i = 1
    while (true) {
        if (i * of > limit) break
        i++
    }
    return i
}

fun forever(): Int {
    var i = 0
    while (true) {
        i++
        if (i > 3) return i
    }
}

fun main() {
    println(sign(-5))
    println(sign(0))
    println(sign(9))
    for (s in 100 downTo 60 step 10) {
        println("$s: ${grade(s)}")
    }
    println(grade(71))
    println(grade(150))
    println(collatz(27))
    println(firstMultiple(7, 50))
    println(forever())
    var sum = 0
    for (i in 0 until 10) {
        if (i % 2 == 0) continue
        if (i > 7) break
        sum += i
    }
    println(sum)
    for (i in 1..10 step 3) print("$i ")
    println()
    for (i in 0..<3) print(i)
    println()
    for (i in 5..1) println("never")
    val x = 15
    val kind = when {
        x < 10 -> "small"
        x < 100 -> "medium"
        else -> "large"
    }
    println(kind)
    println(x in 10..20 && x !in 12..14)
    println(!(x > 3) || x == 15)
    val big = if (x > 10) {
        val doubled = x * 2
        doubled + 1
    } else {
        0
    }
    println(big)
    for (i in 2147483646..2147483647) println(i)
}
