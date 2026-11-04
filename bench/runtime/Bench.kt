// Workloads for bench/runtime.sh, which compiles this file with kc and
// with kotlinc and runs both, to compare the speed of the code each
// compiler writes (not the speed of compiling).
//
// Each workload returns a checksum, so the JIT can't drop its work and the
// two builds can be checked to compute the same thing. main runs every
// workload for a few warmup rounds, then times the rest and prints one line
// per workload: name, median milliseconds per round, checksum.

import kotlin.math.sqrt

// Primitive arithmetic in nested loops over ranges.
fun loops(): Long {
    var sum = 0L
    for (i in 0 until 3_000) {
        for (j in i downTo 0 step 3) {
            sum += (i xor j) and 0xff
        }
    }
    return sum
}

// The standard library's inline functions over primitive arrays: the
// lambdas are inlined, and how their Int arguments are passed matters.
fun inlineLambdas(): Long {
    val a = IntArray(200_000) { it * 7 % 1000 }
    var total = 0L
    repeat(5) { r ->
        total += a.fold(0L) { acc, x -> acc + x * r }
        total += a.count { it % 3 == r % 3 }
        a.forEachIndexed { i, x -> if (x > 990) total += i }
    }
    return total
}

// Boxed collections: map, filter, sort, groupBy.
fun collections(): Long {
    val xs = (1..100_000).map { (it * 2_654_435_761L % 100_003).toInt() }
    val evens = xs.filter { it % 2 == 0 }.map { it / 2 }
    val sorted = evens.sortedDescending()
    val groups = xs.groupBy { it % 17 }
    return sorted.take(100).sum().toLong() + groups.values.sumOf { it.size.toLong() * it.first() } + evens.size
}

data class Point(val x: Int, val y: Int)

// Hashing: data class equals/hashCode as map keys, and string keys.
fun hashing(): Long {
    val counts = HashMap<Point, Int>()
    for (i in 0 until 200_000) {
        val p = Point(i % 300, i % 271)
        counts[p] = (counts[p] ?: 0) + 1
    }
    val words = HashMap<String, Int>()
    for (i in 0 until 100_000) {
        val w = "w" + (i % 997)
        words.merge(w, 1, Int::plus)
    }
    return counts.size.toLong() * 31 + counts.getValue(Point(0, 0)) + words.size + words.getValue("w7")
}

// String building and templates.
fun strings(): Long {
    val sb = StringBuilder()
    for (i in 0 until 50_000) {
        sb.append("item ").append(i).append(';')
        if (i % 1000 == 0) sb.append("${i / 1000}:${sb.length}\n")
    }
    val s = sb.toString()
    return s.length.toLong() + s.split(';').count { it.endsWith("7") } + s.hashCode()
}

sealed interface Shape {
    fun area(): Double
}

class Circle(val r: Double) : Shape {
    override fun area() = 3.141592653589793 * r * r
}

class Rect(val w: Double, val h: Double) : Shape {
    override fun area() = w * h
}

object Empty : Shape {
    override fun area() = 0.0
}

// Interface calls and a when over a sealed type.
fun shapes(): Long {
    val shapes = List(100_000) {
        when (it % 3) {
            0 -> Circle(it % 10 + 1.0)
            1 -> Rect(it % 7 + 1.0, it % 5 + 1.0)
            else -> Empty
        }
    }
    var area = 0.0
    var sides = 0L
    repeat(5) {
        for (s in shapes) {
            area += s.area()
            sides += when (s) {
                is Circle -> 0
                is Rect -> 4
                Empty -> 1
            }
        }
    }
    return area.toLong() + sides
}

enum class Op { ADD, SUB, MUL, XOR }

// A when over an enum, which kc compiles to equals calls and kotlinc to
// an ordinal switch.
fun enums(): Long {
    val ops = Op.entries
    var acc = 1L
    for (i in 0 until 1_000_000) {
        acc = when (ops[i and 3]) {
            Op.ADD -> acc + i
            Op.SUB -> acc - (i shr 1)
            Op.MUL -> acc * 3 % 1_000_000_007
            Op.XOR -> acc xor i.toLong()
        }
    }
    return acc
}

// Function values that aren't inlined, with a captured var.
fun closures(): Long {
    var calls = 0
    val fs: List<(Int) -> Int> = listOf({ calls++; it + 1 }, { calls++; it * 2 }, { calls++; it - 3 })
    var x = 0
    for (i in 0 until 1_000_000) {
        x = fs[i % 3](x) and 0xffff
    }
    return x.toLong() + calls
}

// A sequence builder: a suspend lambda's state machine, resumed per element.
fun sequences(): Long {
    val primes = sequence {
        var n = 2
        while (true) {
            var prime = true
            val limit = sqrt(n.toDouble()).toInt()
            var d = 2
            while (d <= limit) {
                if (n % d == 0) {
                    prime = false
                    break
                }
                d++
            }
            if (prime) yield(n)
            n++
        }
    }
    return primes.take(5_000).fold(0L) { acc, p -> acc + p } + generateSequence(1L) { it * 3 % 1_000_003 }.take(200_000).sum()
}

class Workload(val name: String, val run: () -> Long)

fun main(args: Array<String>) {
    val warmup = args.getOrNull(0)?.toInt() ?: 10
    val rounds = args.getOrNull(1)?.toInt() ?: 20
    val workloads = listOf(
        Workload("loops", ::loops),
        Workload("inlineLambdas", ::inlineLambdas),
        Workload("collections", ::collections),
        Workload("hashing", ::hashing),
        Workload("strings", ::strings),
        Workload("shapes", ::shapes),
        Workload("enums", ::enums),
        Workload("closures", ::closures),
        Workload("sequences", ::sequences),
    )
    for (w in workloads) {
        var check = 0L
        repeat(warmup) { check = w.run() }
        val times = LongArray(rounds)
        for (r in 0 until rounds) {
            val start = System.nanoTime()
            val c = w.run()
            times[r] = System.nanoTime() - start
            check(c == check) { "${w.name}: checksum changed from $check to $c" }
        }
        times.sort()
        println("${w.name}\t${"%.3f".format(times[rounds / 2] / 1e6)}\t$check")
    }
}
