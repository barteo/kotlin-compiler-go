import kotlin.coroutines.*
import kotlin.coroutines.intrinsics.*

suspend fun <T> later(v: T): T = suspendCoroutineUninterceptedOrReturn { c ->
    Thread { c.resume(v) }.start()
    COROUTINE_SUSPENDED
}

open class Base(val name: String) {
    open suspend fun greet(who: String): String {
        val a = later("Hello")
        return "$a, $who, from ${later(name)}"
    }

    open suspend fun twice(x: Int = 21): Int = later(x) * 2

    suspend fun final(): String = later("final " + name)

    private suspend fun secret(): Int {
        val x = later(40)
        return x + later(2)
    }

    suspend fun callSecret(): Int = secret() + 0
}

class Derived : Base("derived") {
    override suspend fun greet(who: String): String {
        val s = super.greet(who)
        return later("[") + s + "]"
    }

    override suspend fun twice(x: Int): Int = super.twice(x) + 1
}

abstract class Shape {
    abstract suspend fun area(): Double

    suspend fun describe(): String = "area " + later(area())
}

class Square(val side: Double) : Shape() {
    override suspend fun area(): Double {
        val s = later(side)
        return s * s
    }
}

interface Repo {
    suspend fun get(id: Int): String

    suspend fun getAll(ids: List<Int>, prefix: String = "#"): String {
        val sb = StringBuilder()
        for (id in ids) {
            sb.append(prefix).append(get(id))
        }
        return later(sb.toString())
    }
}

class MapRepo : Repo {
    override suspend fun get(id: Int): String = later("item$id")
}

class Logged(private val inner: Repo) : Repo by inner {
    override suspend fun get(id: Int): String {
        val v = inner.get(id)
        return later("logged($v)")
    }
}

class Wrapped(inner: Repo) : Repo by inner

interface Source<T> {
    suspend fun next(): T
}

class Counter : Source<Int> {
    var n = 0
    override suspend fun next(): Int {
        n = later(n + 1)
        return n
    }
}

suspend fun <T> take(s: Source<T>, k: Int): List<T> {
    val out = ArrayList<T>()
    repeat(k) { out.add(s.next()) }
    return out
}

object Registry {
    private var count = 0
    suspend fun register(name: String): Int {
        later(name)
        count++
        return later(count)
    }
}

class Factory {
    companion object {
        suspend fun create(n: Int): String = "made " + later(n)

        @JvmStatic
        suspend fun make(n: Int): String {
            val m = later(n * 2)
            return "static $m"
        }
    }
}

@JvmInline
value class Meters(val v: Int) {
    suspend fun plus(o: Meters): Meters {
        val a = later(v)
        return Meters(a + later(o.v))
    }
}

suspend fun grow(m: Meters): Meters {
    val more = later(m.v + 1)
    return Meters(more)
}

suspend fun measure(): Meters = grow(later(Meters(1)))

suspend fun anonymous(): String {
    val r = object : Repo {
        override suspend fun get(id: Int): String = later("anon$id")
    }
    return r.getAll(listOf(1, 2), prefix = "-")
}

interface Sink<T> {
    suspend fun put(x: T): T
}

class IntSink : Sink<Int> {
    var sum = 0
    override suspend fun put(x: Int): Int {
        sum += later(x)
        return sum
    }
}

suspend fun <T> feed(s: Sink<T>, v: T): T = s.put(v)

@JvmOverloads
suspend fun overloaded(a: Int, b: String = "b", c: Long = 3L): String = later("$a$b$c")

suspend fun main() {
    println(Base("base").greet("you"))
    println(Derived().greet("me"))
    val b: Base = Derived()
    println(b.twice())
    println(Base("b").twice())
    println(b.final())
    println(Base("x").callSecret())
    println(Square(3.0).describe())
    println(MapRepo().getAll(listOf(1, 2, 3)))
    println(Logged(MapRepo()).getAll(listOf(4), "*"))
    println(Wrapped(MapRepo()).get(5))
    println(take(Counter(), 3))
    println(Registry.register("a") + Registry.register("b"))
    println(Factory.create(1))
    println(Factory.make(2))
    println(measure().v)
    println(Meters(3).plus(Meters(4)).v)
    println(anonymous())
    val sink = IntSink()
    feed(sink, 2)
    println(feed(sink, 3) + sink.put(4))
    println(overloaded(1) + overloaded(2, "x") + overloaded(3, "y", 9L))
}
