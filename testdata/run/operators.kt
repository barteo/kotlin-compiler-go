// Operator conventions on the program's types: arithmetic and unary
// operators, comparisons, in, invoke, compound assignments, ++ and --,
// indexed compound assignment, and iteration through an extension.

data class Vec(val x: Int, val y: Int) {
    operator fun plus(o: Vec) = Vec(x + o.x, y + o.y)
    operator fun minus(o: Vec) = Vec(x - o.x, y - o.y)
    operator fun times(k: Int) = Vec(x * k, y * k)
    operator fun unaryMinus() = Vec(-x, -y)
    operator fun unaryPlus() = this
    operator fun inc() = Vec(x + 1, y + 1)
    operator fun dec() = Vec(x - 1, y - 1)
    operator fun get(i: Int) = if (i == 0) x else y
}

operator fun Vec.div(k: Int) = Vec(x / k, y / k)
operator fun Vec.rem(k: Int) = Vec(x % k, y % k)
operator fun Int.times(v: Vec) = v * this

class Money(val cents: Long) : Comparable<Money> {
    override fun compareTo(other: Money): Int = if (cents < other.cents) -1 else if (cents > other.cents) 1 else 0
    override fun toString() = "$" + (cents / 100) + "." + (cents % 100)
}

class Version(val major: Int, val minor: Int) {
    operator fun compareTo(o: Version): Int = if (major != o.major) major - o.major else minor - o.minor
}

class Flag(val on: Boolean) {
    operator fun not() = Flag(!on)
}

class Interval(val lo: Int, val hi: Int) {
    operator fun contains(x: Int) = x >= lo && x <= hi
}

class Greeter(val greeting: String) {
    operator fun invoke(name: String) = "$greeting, $name!"
    operator fun invoke() = "$greeting!"
}

class Counter(var n: Int) {
    operator fun plusAssign(k: Int) {
        n += k
    }
    operator fun minusAssign(k: Int) {
        n -= k
    }
}

class Bag {
    val items = java.util.ArrayList<String>()
    operator fun plusAssign(s: String) {
        items.add(s)
    }
    operator fun contains(s: String) = items.contains(s)
}

class Grid(val w: Int) {
    val cells = java.util.ArrayList<Int>()
    init {
        for (i in 0 until w * w) cells.add(0)
    }
    operator fun get(x: Int, y: Int): Int = cells.get(y * w + x)
    operator fun set(x: Int, y: Int, v: Int) {
        cells.set(y * w + x, v)
    }
}

class Cells {
    val vs = arrayOf(Vec(0, 0), Vec(1, 1))
    operator fun get(i: Int) = vs[i]
    operator fun set(i: Int, v: Vec) {
        vs[i] = v
    }
}

class Holder(var v: Vec)

class Digits(val n: Int)
class DigitIterator(var rest: Int) {
    operator fun hasNext() = rest > 0
    operator fun next(): Int {
        val d = rest % 10
        rest /= 10
        return d
    }
}
operator fun Digits.iterator() = DigitIterator(n)

// Ranges of the program's types: rangeTo and rangeUntil.
class Day(val n: Int) {
    operator fun rangeTo(o: Day) = DayRange(this, o, true)
    operator fun rangeUntil(o: Day) = DayRange(this, o, false)
    operator fun compareTo(o: Day) = n - o.n
}

class DayRange(val from: Day, val to: Day, val closed: Boolean) {
    operator fun contains(d: Day) = d >= from && (if (closed) d <= to else d < to)
    operator fun iterator() = DayIterator(from.n, if (closed) to.n else to.n - 1)
}

class DayIterator(var next: Int, val last: Int) {
    operator fun hasNext() = next <= last
    operator fun next(): Day {
        next += 1
        return Day(next - 1)
    }
}

var calls = 0
fun counted(v: Vec): Vec {
    calls++
    return v
}

fun main() {
    val a = Vec(1, 2)
    val b = Vec(3, 4)
    println(a + b)
    println(b - a)
    println(a * 3)
    println(3 * a)
    println(b / 2)
    println(b % 3)
    println(-a)
    println(+a)
    println(a + b * 2 - a)
    println(a[0] + a[1])

    println(Money(150) < Money(200))
    println(Money(150) >= Money(200))
    println(Version(1, 2) < Version(1, 10))
    println(Version(2, 0) > Version(1, 10))
    println(if (Version(1, 0) <= Version(1, 0)) "le" else "gt")

    println((!Flag(true)).on)
    println(5 in Interval(1, 10))
    println(15 in Interval(1, 10))
    println(15 !in Interval(1, 10))
    val iv = Interval(0, 3)
    for (x in 0..5) {
        when (x) {
            in iv -> print("in ")
            !in Interval(5, 5) -> print("out ")
            else -> print("five ")
        }
    }
    println()

    val g = Greeter("Hello")
    println(g("Ada"))
    println(g())
    println(Greeter("Hi")("Bob"))

    var v = Vec(0, 0)
    v += Vec(1, 1)
    v -= Vec(0, 1)
    v *= 5
    println(v)
    v++
    println(v)
    v--
    v--
    println(v)

    val c = Counter(1)
    c += 10
    c -= 3
    println(c.n)
    val bag = Bag()
    bag += "apple"
    bag += "pear"
    println("pear" in bag)
    println("plum" in bag)
    println(bag.items)

    val h = Holder(Vec(1, 1))
    h.v += Vec(1, 0)
    h.v++
    println(h.v)

    val grid = Grid(3)
    grid[1, 2] = 5
    grid[1, 2] += 10
    grid[1, 2]++
    grid[0, 0]--
    println(grid[1, 2])
    println(grid[0, 0])

    val cells = Cells()
    cells[1] += Vec(10, 10)
    cells[0]++
    println(cells[0])
    println(cells[1])

    val list = java.util.ArrayList<Int>()
    list.add(1)
    list.add(2)
    list[0] += 40
    list[1]++
    println(list)
    val map = java.util.HashMap<String, Int>()
    map.put("a", 1)

    val holders = arrayOf(Holder(Vec(0, 0)))
    holders[0].v += Vec(2, 3)
    println(holders[0].v)

    var w = counted(Vec(1, 1))
    w += counted(Vec(1, 1))
    println("$w $calls")

    var sum = 0
    for (d in Digits(4096)) sum += d
    println(sum)

    for (d in Day(1)..Day(3)) print("${d.n} ")
    for (d in Day(1)..<Day(3)) print("${d.n} ")
    println()
    val week = Day(1)..Day(7)
    println(Day(3) in week)
    println(Day(9) in Day(1)..Day(7))
    println(Day(7) !in Day(1)..<Day(7))
    when (Day(5)) {
        in Day(1)..Day(4) -> println("early")
        in Day(5)..Day(7) -> println("late")
        else -> println("never")
    }
}
