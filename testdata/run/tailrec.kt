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

// A call followed by a return without a value is in tail position.
var steps = 0

tailrec fun unitSteps(x: Int) {
    steps++
    if (x > 600000) {
        unitSteps(x - 1)
        return
    } else if (x > 300000) {
        notTail(1)
        if (x > 1) {
            unitSteps(x - 1)
        }
        return
    } else if (x > 0) {
        unitSteps(x - 1)
    }
}

// The right sides of ?:, && and || are in tail position.
fun maybe(x: Int): Int? = if (x == 1) x else null

tailrec fun elvis(x: Int): Int = maybe(x) ?: elvis(x - 1)

tailrec fun elvisChain(n: Int): Int? {
    if (n < 0) return null
    if (n == 0) return 777
    return elvisChain(-1) ?: elvisChain(-2) ?: elvisChain(n - 1)
}

tailrec fun anyZero(x: Int): Boolean = x == 0 || anyZero(x - 1)

tailrec fun allPositive(x: Int): Boolean = x > 0 && (x == 1 || allPositive(x - 1))

inline fun <T> nothing(f: () -> T?): T? = f()

tailrec fun whenElvis(n: Int): Int = when (n) {
    0 -> whenElvis(1)
    1 -> n
    else -> nothing<Int> { null } ?: whenElvis(n - 1)
}

// Parentheses don't change tail position.
tailrec fun parens(x: Int): Int {
    if (x == 0) return 42
    return (parens(x - 1))
}

tailrec fun returnInParens(x: Int) {
    if (x == 0) return
    (return returnInParens(x - 1))
}

// A call of a member of an object on the object, named, is on this.
object Counter {
    var calls = 0

    tailrec fun count(i: Int): Int {
        calls++
        if (i <= 0) return calls
        return Counter.count(i - 1)
    }
}

class Outer {
    companion object {
        tailrec fun down(i: Int): Int {
            if (i <= 0) return i
            return Outer.down(i - 1)
        }

        tailrec fun downNamed(i: Int): Int {
            if (i <= 0) return i
            return Outer.Companion.downNamed(i - 1)
        }
    }
}

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
    unitSteps(1000000)
    println(steps)
    println(elvis(1000000))
    println(elvisChain(1000000))
    println(anyZero(1000000))
    println(allPositive(1000000))
    println(whenElvis(1000000))
    println(parens(1000000))
    returnInParens(1000000)
    println(Counter.count(1000000))
    println(Outer.down(1000000))
    println(Outer.downNamed(1000000))
}
