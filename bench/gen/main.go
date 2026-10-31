// Command gen writes a synthetic Kotlin project that uses only the subset
// kc supports, so kc and kotlinc can compile the same input.
//
//	go run ./bench/gen -n 1000 -o /tmp/src
//
// Each file has a class and several functions that call into other files,
// so name resolution crosses file boundaries, a function that calls
// JDK classes, so both compilers read the class path, a function that
// passes lambdas to generic and JDK functions, and a function that uses
// the Kotlin standard library: collections, extension functions with
// lambdas, varargs and default arguments. It also uses the class model:
// an interface with an abstract base class and two subclasses, a sealed
// hierarchy of data classes and an object evaluated by a when with is
// checks, an enum, a companion object with a default argument, copy with
// a named argument, destructuring, and try/catch/finally with a custom
// exception. And it uses arrays, a vararg function called with and
// without the spread operator, a loop over a string's chars with a Char
// range, and Float, Byte and Short arithmetic. And it uses inline
// functions with lambdas, which both compilers inline: let, apply, also,
// takeIf, repeat, with, buildString, and a non-local return from forEach.
// And it uses extensions and operator conventions: extension functions,
// one imported from another file, with a default argument and infix, an
// extension property, a data class with arithmetic and comparison
// operators, a class with plusAssign, get, contains and iterator, and
// compound assignment and in on a list. And it uses coroutines: suspend
// extensions of SequenceScope, with a loop and a when, which a
// sequence { } builder runs.
// Files are in 10 packages, and import from each other. Main.kt
// prints a checksum, which lets the benchmark confirm both compilers'
// output behaves the same.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const fileTemplate = `package p{p}

import java.math.BigInteger
import p{pj}.applyTwice{j}
import p{pj}.classify{j}
import p{pj}.compute{j}
import p{pj}.helper{j}
import p{pj}.shout{j}

class Item{i}(val id: Int, var score: Double, val label: String?) {
    var visits = 0
    val tag: String = "item-" + id

    init {
        if (score < 0.0) {
            this.score = 0.0
        }
    }

    fun bump(by: Int): Int {
        visits += by
        return visits
    }

    fun weight(): Double = score * (visits + 1)

    fun describe(): String {
        val shown = label ?: "none"
        return "Item{i}(id=$id, score=$score, label=$shown, visits=$visits)"
    }
}

val base{i}: Int = {i} % 17 + 3
var calls{i}: Long = 0

fun compute{i}(n: Int): Long {
    var total = 0L
    for (k in 0 until n) {
        total += when {
            k % 3 == 0 -> k * 2L
            k % 5 == 0 -> k.toLong()
            else -> 1L
        }
    }
    calls{i}++
    return total
}

fun classify{i}(x: Int): String = when (x) {
    0 -> "zero"
    in 1..9 -> "small"
    in 10 until 100 -> "medium"
    !in 0..1000 -> "out of range"
    else -> "large"
}

fun helper{i}(s: String?): Int = s?.length ?: 0

fun fib{i}(n: Int): Long {
    if (n < 2) return n.toLong()
    var a = 0L
    var b = 1L
    for (k in 2..n) {
        val next = a + b
        a = b
        b = next
    }
    return b
}

fun safeDivide{i}(a: Int, b: Int): Int? = if (b == 0) null else a / b

fun format{i}(item: Item{i}): String {
    val parts = if (item.visits > 2) "busy" else "quiet"
    val label = item.label
    if (label != null && label.length > 3) {
        return "${item.tag}:$parts:${label.uppercase()}"
    }
    return item.tag + ":" + parts
}

fun jdk{i}(n: Int): Long {
    val sb = StringBuilder()
    sb.append("f{i}-").append(n).append(true)
    val r = java.util.Random({i}L)
    var acc = r.nextInt(100).toLong()
    acc += Math.max(n, {i} % 13) + Math.abs(n - 50)
    acc += Math.sqrt((n * n).toDouble()).toLong()
    acc += if (Integer.MAX_VALUE > n) 1 else 0
    acc += BigInteger.valueOf(n.toLong()).pow(3).bitLength()
    acc += sb.length
    return acc
}

fun <T, R> mapAll{i}(xs: java.util.ArrayList<T>, f: (T) -> R): java.util.ArrayList<R> {
    val out = java.util.ArrayList<R>()
    for (k in 0 until xs.size) out.add(f(xs.get(k)))
    return out
}

fun applyTwice{i}(f: (Int) -> Int, x: Int): Int = f(f(x))

fun lambdas{i}(n: Int): Long {
    val offset = n % 7 + {i}
    var hits = 0
    val xs = java.util.ArrayList<Int>()
    for (k in 0 until n) xs.add((k * 31 + {i}) % 97)
    java.util.Collections.sort(xs) { a, b -> b - a }
    xs.removeIf { it % 5 == 0 }
    val labels = mapAll{i}(xs) { x ->
        if (x > 50) hits++
        "v" + (x + offset)
    }
    var acc = applyTwice{i}({ it * 2 + offset }, n).toLong()
    acc += labels.size + hits
    xs.forEach { acc += it }
    acc += applyTwice{j}({ x -> x + 1 }, {i} % 10)
    return acc
}

fun stdlib{i}(n: Int): Long {
    val xs = listOf(n, {i} % 11, n * 3, 7)
    val evens = xs.filter { it % 2 == 0 }.map { it * {j} % 50 }
    var acc = evens.size.toLong()
    for (x in xs) acc += x
    val counts = mutableMapOf<String, Int>()
    counts["f{i}"] = n
    counts["g{i}"] = xs.size
    acc += counts.getOrDefault("f{i}", 0) + (counts["g{i}"] ?: 0)
    val p = Pair(xs.first(), xs.last())
    acc += p.first + p.second + xs.joinToString(",").length
    return acc
}

interface Shape{i} {
    val name: String
    fun area(): Double
    fun describe(): String = name + ":" + area().toInt()
}

abstract class Base{i}(override val name: String) : Shape{i} {
    var scaled = 0

    open fun scale(k: Int): Int {
        scaled += k
        return scaled
    }
}

class Rect{i}(val w: Int, val h: Int) : Base{i}("rect{i}") {
    override fun area(): Double = (w * h).toDouble()
}

class Circle{i}(val r: Int) : Base{i}("circle{i}") {
    override fun area(): Double = 3.0 * r * r
    override fun scale(k: Int): Int = super.scale(k * 2)
}

sealed interface Expr{i}
data class Num{i}(val value: Int) : Expr{i}
data class Sum{i}(val left: Expr{i}, val right: Expr{i}) : Expr{i}
data class Prod{i}(val left: Expr{i}, val right: Expr{i}) : Expr{i}
data object Zero{i} : Expr{i}

fun eval{i}(e: Expr{i}): Int = when (e) {
    is Num{i} -> e.value
    is Sum{i} -> eval{i}(e.left) + eval{i}(e.right)
    is Prod{i} -> eval{i}(e.left) * eval{i}(e.right)
    Zero{i} -> 0
}

enum class Level{i}(val weight: Int) {
    LOW(1), MID(5), HIGH(10);

    fun next(): Level{i} = when (this) {
        LOW -> MID
        MID -> HIGH
        HIGH -> LOW
    }
}

class Counter{i} private constructor(val start: Int) {
    var count = start

    companion object {
        var created = 0

        fun create(start: Int = {i} % 5): Counter{i} {
            created++
            return Counter{i}(start)
        }
    }
}

class Failure{i}(val code: Int) : Exception("failure " + code)

fun risky{i}(n: Int): Int {
    require(n >= 0) { "negative: " + n }
    if (n % 3 == 0) throw Failure{i}(n)
    return n * 2
}

fun classes{i}(n: Int): Long {
    val shapes = java.util.ArrayList<Shape{i}>()
    shapes.add(Rect{i}(n % 7 + 1, 3))
    shapes.add(Circle{i}(n % 5 + 1))
    var acc = 0L
    for (s in shapes) {
        acc += s.area().toLong() + s.describe().length
        if (s is Base{i}) acc += s.scale(n % 4)
    }
    val e: Expr{i} = Sum{i}(Num{i}(n), Prod{i}(Num{i}({i} % 9), Sum{i}(Num{i}(2), Zero{i})))
    acc += eval{i}(e)
    val p = Num{i}(n)
    val q = p.copy(value = n + 1)
    acc += if (p == q) 1 else q.value
    val (v) = q
    acc += v + q.hashCode() % 100
    val level = Level{i}.entries[n % 3].next()
    acc += level.weight + level.ordinal
    val c = Counter{i}.create()
    c.count += n
    acc += c.count + Counter{i}.created
    try {
        acc += risky{i}(n)
    } catch (f: Failure{i}) {
        acc += f.code
    } finally {
        acc += 1
    }
    return acc
}

fun total{i}(vararg xs: Int): Int {
    var s = 0
    for (x in xs) s += x
    return s
}

fun arrays{i}(n: Int): Long {
    val squares = IntArray(n % 8 + 4) { it * it }
    squares[1] += {i} % 7
    val names = arrayOf("a{i}", "b{j}", "c")
    names[2] += n
    var acc = total{i}() + total{i}(n, 2) + total{i}(*squares) + total{i}(1, *squares, n).toLong()
    for (name in names) acc += name.length
    val word = "kotlin{i}"
    var letters = 0
    for (ch in word) {
        if (ch in 'a'..'z') letters++
    }
    val shifted = 'a' + n % 26
    acc += letters + (shifted - 'a') + word[0].code
    val f = n * 0.5f + {i} % 3
    acc += (f * 4).toInt()
    val b: Byte = (n % 100).toByte()
    val s: Short = ({i} % 1000).toShort()
    acc += b + s
    val bytes = byteArrayOf(1, 2, b)
    for (x in bytes) acc += x
    return acc
}

fun firstOver{i}(xs: List<Int>, limit: Int): Int {
    xs.forEach { if (it > limit) return it }
    return -1
}

fun inline{i}(n: Int): Long {
    val item = Item{i}(n, 2.0, "x{i}").apply {
        bump(2)
        score += visits
    }
    var acc = item.let { it.visits + it.id }.toLong()
    acc += (n % 9).takeIf { it > 3 } ?: 0
    val text = buildString {
        append("in{i}-")
        repeat(n % 4 + 1) { append(it) }
        append(item.tag.length)
    }.also { acc += it.length }
    repeat(3) { k -> acc += k * {i} % 5 }
    acc += firstOver{i}(listOf(n, n * 2, {j}), n + 1)
    acc += with(item) { visits * 2 + text.length }
    return acc
}

data class Money{i}(val cents: Long) : Comparable<Money{i}> {
    operator fun plus(o: Money{i}) = Money{i}(cents + o.cents)
    operator fun times(k: Int) = Money{i}(cents * k)
    operator fun unaryMinus() = Money{i}(-cents)
    override fun compareTo(other: Money{i}): Int = if (cents < other.cents) -1 else if (cents > other.cents) 1 else 0
}

operator fun Money{i}.minus(o: Money{i}) = Money{i}(cents - o.cents)

class Ledger{i} {
    val entries = java.util.ArrayList<Money{i}>()

    operator fun plusAssign(m: Money{i}) {
        entries.add(m)
    }

    operator fun get(k: Int): Money{i} = entries.get(k)
    operator fun contains(m: Money{i}) = entries.contains(m)
    operator fun iterator() = entries.iterator()
}

fun String.shout{i}(times: Int = 1): String {
    var s = this
    for (k in 0 until times) s += "!"
    return s
}

val String.vowels{i}: Int
    get() {
        var c = 0
        for (ch in this) {
            if (ch in "aeiou") c++
        }
        return c
    }

fun Int.clampTo{i}(lo: Int, hi: Int): Int = if (this < lo) lo else if (this > hi) hi else this

infix fun Int.pow{i}(e: Int): Long {
    var r = 1L
    for (k in 0 until e) r *= this
    return r
}

fun extensions{i}(n: Int): Long {
    var acc = "f{i}".shout{i}(n % 3).length.toLong()
    acc += "kotlin{i}".vowels{i} + n.clampTo{i}(2, 9) + (2 pow{i} n % 5)
    acc += "x".shout{j}().length
    var m = Money{i}(n.toLong()) + Money{i}(5) * 3
    m -= Money{i}(1)
    m += -Money{i}(2)
    val ledger = Ledger{i}()
    ledger += m
    ledger += Money{i}({i}.toLong())
    if (m in ledger) acc += 1
    if (ledger[0] > Money{i}(10)) acc += 2
    for (e in ledger) acc += e.cents
    val ys = mutableListOf(n, 1)
    ys[0] += 3
    ys += 7
    for (y in ys) acc += y
    if (n in ys) acc += 1
    return acc
}

suspend fun SequenceScope<Int>.steps{i}(n: Int) {
    var k = 0
    while (k < n) {
        when (k % 3) {
            0 -> yield(k)
            1 -> yield(k * {j} % 17)
            else -> yieldAll(listOf(k, -k))
        }
        k++
    }
}

suspend fun SequenceScope<Int>.pairs{i}(xs: List<Int>) {
    for (x in xs) {
        val y = x + {i} % 5
        steps{i}(2)
        yield(x * y)
    }
}

fun coroutines{i}(n: Int): Long {
    val seq = sequence<Int> {
        steps{i}(n % 7 + 3)
        pairs{i}(listOf(n, {i} % 9))
        yield(-1)
    }
    var acc = 0L
    for (v in seq) acc += v
    return acc + seq.take(3).count()
}

fun run{i}(): Long {
    val item = Item{i}({i}, 1.5 + {i} % 4, if ({i} % 2 == 0) "even" else null)
    item.bump(base{i})
    item.bump(1)
    var acc = compute{i}(base{i} * 2) + helper{i}(item.label) + fib{i}(base{i})
    acc += (safeDivide{i}(acc.toInt(), {i} % 3) ?: -1).toLong()
    acc += classify{i}(acc.toInt()).length
    acc += format{i}(item).length
    acc += item.weight().toLong()
    acc += helper{j}(classify{j}({i}))
    acc += compute{j}(3)
    acc += jdk{i}(base{i})
    acc += lambdas{i}(base{i})
    acc += stdlib{i}(base{i})
    acc += classes{i}(base{i})
    acc += arrays{i}(base{i})
    acc += inline{i}(base{i})
    acc += extensions{i}(base{i})
    acc += coroutines{i}(base{i})
    return acc
}
`

const mainTemplate = `fun main() {
    var checksum = 0L
{calls}    println("checksum: $checksum")
}
`

// packages is the number of packages the files are in.
const packages = 10

func main() {
	n := flag.Int("n", 100, "number of files")
	out := flag.String("o", "src", "output directory")
	flag.Parse()
	if err := os.MkdirAll(*out, 0o755); err != nil {
		log.Fatal(err)
	}
	for i := range *n {
		j := (i*7 + 1) % *n // another file this one calls into
		src := strings.NewReplacer("{i}", strconv.Itoa(i), "{j}", strconv.Itoa(j),
			"{p}", strconv.Itoa(i%packages), "{pj}", strconv.Itoa(j%packages)).Replace(fileTemplate)
		if err := os.WriteFile(filepath.Join(*out, fmt.Sprintf("File%d.kt", i)), []byte(src), 0o644); err != nil {
			log.Fatal(err)
		}
	}
	var calls strings.Builder
	for i := range *n {
		fmt.Fprintf(&calls, "    checksum += p%d.run%d()\n", i%packages, i)
	}
	src := strings.Replace(mainTemplate, "{calls}", calls.String(), 1)
	if err := os.WriteFile(filepath.Join(*out, "Main.kt"), []byte(src), 0o644); err != nil {
		log.Fatal(err)
	}
}
