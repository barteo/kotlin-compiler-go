// tailrec functions: calls of themselves in tail position are jumps, so
// recursion as deep as a loop doesn't overflow the stack.

tailrec fun sum(n: Long, acc: Long = 0): Long = if (n == 0L) acc else sum(n - 1, acc + n)

tailrec fun gcd(a: Int, b: Int): Int {
    if (b == 0) return a
    return gcd(b, a % b)
}

tailrec fun countDown(n: Int, step: Int = 1, seen: Int = n) {
    if (n <= 0) {
        println("done " + seen)
        return
    }
    countDown(n - step)
}

tailrec fun Int.digits(acc: Int = 0): Int = when {
    this < 10 -> acc + 1
    else -> (this / 10).digits(acc + 1)
}

class Node(val value: Int, val next: Node?)

tailrec fun Node.last(): Int {
    val n = next ?: return value
    return n.last()
}

class Walker(val limit: Int) {
    tailrec fun walk(i: Int, acc: Int): Int =
        if (i >= limit) acc
        else walk(i + 1, acc + i % 7)
}

tailrec fun collatz(n: Long, steps: Int = 0): Int = when {
    n == 1L -> steps
    n % 2 == 0L -> collatz(n / 2, steps + 1)
    else -> collatz(3 * n + 1, steps + 1)
}

fun notTail(n: Int): Int = if (n == 0) 0 else 1 + notTail(n - 1)

fun main() {
    println(sum(1000000))
    println(gcd(1071, 462))
    countDown(1000000)
    countDown(10, 3)
    println(123456789.digits())
    var list: Node? = null
    for (i in 1..100000) list = Node(i, list)
    println(list!!.last())
    println(Walker(1000000).walk(0, 0))
    println(collatz(27))
    println(notTail(100))
}
