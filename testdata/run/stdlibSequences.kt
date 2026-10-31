fun fibonacci(): Sequence<Long> = sequence {
    var a = 0L
    var b = 1L
    while (true) {
        yield(a)
        val t = a + b
        a = b
        b = t
    }
}

fun nested(n: Int): Sequence<Int> = sequence<Int> {
    yield(-1)
    yieldAll(1..n)
    yieldAll(listOf(10, 20))
    yieldAll(sequenceOf(100, 200))
    yield(-2)
}

class Tree(val value: Int, val left: Tree? = null, val right: Tree? = null) {
    fun walk(): Sequence<Int> = sequence<Int> {
        left?.let { yieldAll(it.walk()) }
        yield(value)
        right?.let { yieldAll(it.walk()) }
    }
}

fun words(text: String, minLength: Int): Iterator<String> = iterator<String> {
    var count = 0
    for (w in text.split(" ")) {
        if (w.length >= minLength) {
            count++
            yield("$count:$w")
        }
    }
    yield("total $count")
}

fun primes(): Sequence<Int> = sequence<Int> {
    val found = ArrayList<Int>()
    var n = 2
    while (true) {
        if (found.none { n % it == 0 }) {
            found.add(n)
            yield(n)
        }
        n++
    }
}

fun main() {
    println(fibonacci().take(15).toList())
    println(nested(3).toList())
    val t = Tree(4, Tree(2, Tree(1), Tree(3)), Tree(6, Tree(5)))
    println(t.walk().toList())
    val it = words("the quick brown fox jumps", 4)
    while (it.hasNext()) {
        println(it.next())
    }
    println(primes().take(10).joinToString())
    var evaluated = 0
    val lazy = sequence<Int> {
        for (i in 1..100) {
            evaluated++
            yield(i * i)
        }
    }
    println(lazy.first { it > 50 })
    println(evaluated)
    val doubling = sequence {
        var x = 1
        repeat(4) {
            yield(x)
            x *= 2
        }
    }
    println(doubling.toList())
}
